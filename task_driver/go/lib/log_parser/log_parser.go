package log_parser

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"

	"go.skia.org/infra/go/ring"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/td"
)

const (
	// Maximum number of output lines stored in memory to pass along as
	// error messages for failed tests.
	OUTPUT_LINES = 20

	// logNameStdout is the log name used for the stdout stream of each step.
	logNameStdout = "stdout"
	// logNameStderr is the log name used for the stderr stream of each step.
	logNameStderr = "stderr"
)

// Step represents a step in a tree of steps generated during Run.
type Step struct {
	ctx      context.Context
	stdout   io.Writer
	stderr   io.Writer
	logBuf   *ring.StringRing
	name     string
	parent   *Step
	root     *Step
	children map[string]*Step
}

// newStep returns a Step instance.
func newStep(ctx context.Context, name string, parent, root *Step) *Step {
	logBuf := ring.NewStringRing(OUTPUT_LINES)
	stdout := io.MultiWriter(logBuf, td.NewLogStream(ctx, logNameStdout, td.Info))
	stderr := io.MultiWriter(logBuf, td.NewLogStream(ctx, logNameStderr, td.Error))
	if parent != nil && parent != root {
		stdout = io.MultiWriter(stdout, parent.stdout)
		stderr = io.MultiWriter(stderr, parent.stderr)
	}
	return &Step{
		ctx:      ctx,
		stdout:   stdout,
		stderr:   stderr,
		logBuf:   logBuf,
		name:     name,
		parent:   parent,
		root:     root,
		children: map[string]*Step{},
	}
}

// StartChild creates a new step as a direct child of this step.
func (s *Step) StartChild(props *td.StepProperties) *Step {
	ctx := td.StartStep(s.ctx, props)
	child := newStep(ctx, props.Name, s, s.root)
	s.children[props.Name] = child
	return child
}

// FindChild finds the descendant of this step with the given name. Returns nil
// if no active descendant step exists.
func (s *Step) FindChild(name string) *Step {
	if found, ok := s.children[name]; ok {
		return found
	}
	for _, child := range s.children {
		found := child.FindChild(name)
		if found != nil {
			return found
		}
	}
	return nil
}

// Fail this step.
func (s *Step) Fail() {
	msg := strings.Join(s.logBuf.GetAll(), "")
	if msg == "" {
		msg = "Step failed with no output"
	}
	_ = td.FailStep(s.ctx, errors.New(msg))
}

// End this step and any of its active descendants.
func (s *Step) End() {
	s.Recurse(func(s *Step) {
		// Mark this step as finished.
		td.EndStep(s.ctx)
		// Remove this step from the parent's children map.
		if s.parent != nil {
			delete(s.parent.children, s.name)
		}
	})
}

// stringToByteLine converts the given string to a slice of bytes and appends
// a newline.
func stringToByteLine(s string) []byte {
	b := make([]byte, len(s)+1)
	copy(b, s)
	b[len(b)-1] = '\n'
	return b
}

// Stdout writes the given string to the stdout streams for this step and all of
// its ancestors except for the root step, which automatically receives the raw
// output of the command. Note that no newline is appended, so if you are using
// bufio.ScanLines to tokenize log output and then calling Step.Stdout to attach
// logs to each step, the newlines which were originally present in the log
// stream will be lost.
func (s *Step) Stdout(msg string) {
	if _, err := s.stdout.Write([]byte(msg)); err != nil {
		sklog.Errorf("Failed to write log output: %s", err)
	}
}

// StdoutLn writes the given string, along with a trailing newline, to the
// stdout streams for this step and all of its ancestors except for the root
// step, which automatically receives the raw output of the command.
func (s *Step) StdoutLn(msg string) {
	if _, err := s.stdout.Write(stringToByteLine(msg)); err != nil {
		sklog.Errorf("Failed to write log output: %s", err)
	}
}

// Stderr writes the given string to the stderr streams for this step and all of
// its ancestors except for the root step, which automatically receives the raw
// output of the command. Note that no newline is appended, so if you are using
// bufio.ScanLines to tokenize log output and then calling Step.Stdout to attach
// logs to each step, the newlines which were originally present in the log
// stream will be lost.
func (s *Step) Stderr(msg string) {
	if _, err := s.stderr.Write([]byte(msg)); err != nil {
		sklog.Errorf("Failed to write log output: %s", err)
	}
}

// StderrLn writes the given string, along with a trailing newline, to the
// stderr streams for this step and all of its ancestors except for the root
// step, which automatically receives the raw output of the command.
func (s *Step) StderrLn(msg string) {
	if _, err := s.stderr.Write(stringToByteLine(msg)); err != nil {
		sklog.Errorf("Failed to write log output: %s", err)
	}
}

// Leaves returns all active non-root Steps with no children.
func (s *Step) Leaves() []*Step {
	if s != s.root && len(s.children) == 0 {
		return []*Step{s}
	}
	var rv []*Step
	for _, child := range s.children {
		leaves := child.Leaves()
		if len(leaves) > 0 {
			rv = append(rv, leaves...)
		}
	}
	return rv
}

// Recurse runs the given function on this Step and each of its descendants,
// in bottom-up order, ie. the func runs for the children before the parent.
func (s *Step) Recurse(fn func(s *Step)) {
	for _, child := range s.children {
		child.Recurse(fn)
	}
	fn(s)
}

// StepManager emits steps during a Run. It is thread-safe.
type StepManager struct {
	mtx  sync.Mutex
	root *Step
}

// newStepManager returns a StepManager instance which creates child steps of
// the given parent step.
func newStepManager(ctx context.Context) *StepManager {
	root := newStep(ctx, "", nil, nil)
	root.root = root
	return &StepManager{
		root: root,
	}
}

// CurrentStep returns the current step or nil if there is no active step. If
// there is more than one active step, one is arbitrarily chosen.
func (s *StepManager) CurrentStep() *Step {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	for _, step := range s.root.Leaves() {
		// Ignore the root step.
		if step != s.root {
			return step
		}
	}
	return nil
}

// StartStep starts a step as a child of the root step.
func (s *StepManager) StartStep(props *td.StepProperties) *Step {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.root.StartChild(props)
}

// FindStep finds the active step with the given name. Returns nil if no
// matching step is found.
func (s *StepManager) FindStep(name string) *Step {
	return s.root.FindChild(name)
}

// Run runs the given command in the given working directory. A root-level step
// is automatically created and inherits the raw output (stdout and stderr,
// separately) and result of the command. Run calls the provided function for
// every token in the stdout stream of the command, as defined by the given
// SplitFunc. This function receives a StepManager which may be used to generate
// sub-steps based on the tokens. Stderr is sent to both the root step and to
// each active sub-step at the time that output is received; note that, since
// stdout and stderr are independent streams, there is no guarantee that stderr
// related to a given sub-step will actually appear in the stderr stream for
// that sub-step. The degree of consistency will depend on the operating system
// and the sub-process itself. Therefore, Run will be most useful and consistent
// for applications whose output is highly structured, with any errors sent to
// stdout as part of this structure. See `go test --json` for a good example.
// Note that this inconsistency applies to the raw stderr stream but not to
// calls to Step.Stdout(), Step.Stderr(), etc.
func Run(ctx context.Context, cwd string, cmdLine []string, split bufio.SplitFunc, handleToken func(*StepManager, string) error) error {
	ctx = td.StartStep(ctx, td.Props(strings.Join(cmdLine, " ")))
	defer td.EndStep(ctx)

	// Create the StepManager.
	sm := newStepManager(ctx)

	// Set up the command.
	cmd := exec.CommandContext(ctx, cmdLine[0], cmdLine[1:]...)
	cmd.Dir = cwd
	cmd.Env = td.GetEnv(ctx)

	// Helper function for streaming output.
	var wg sync.WaitGroup
	stream := func(r io.Reader, w io.Writer, split bufio.SplitFunc, handle func(string)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(io.TeeReader(r, w))
			scanner.Split(split)
			for scanner.Scan() {
				handle(scanner.Text())
			}
		}()
	}

	// Handle stdout.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return td.FailStep(ctx, err)
	}
	var parseErr error
	stream(stdout, sm.root.stdout, split, func(tok string) {
		if err := handleToken(sm, tok); err != nil {
			parseErr = skerr.Wrapf(err, "Failed handling token %q", tok)
			sklog.Error(parseErr.Error())
		}
	})

	// Handle stderr. Attempt to direct it to the appropriate sub-step.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return td.FailStep(ctx, err)
	}
	stream(stderr, sm.root.stderr, bufio.ScanLines, func(tok string) {
		for _, step := range sm.root.Leaves() {
			step.StderrLn(tok)
		}
	})

	// Start the command.
	if err := cmd.Start(); err != nil {
		return td.FailStep(ctx, skerr.Wrapf(err, "Failed to start command"))
	}

	// Wait for the command to finish.
	err = cmd.Wait()
	wg.Wait()

	// If any steps are still active, mark them finished.
	sm.root.Recurse(func(s *Step) {
		// If the command failed, we can't know for sure which step was
		// the cause, so we fail any active steps.
		if err != nil {
			s.Fail()
		}
		s.End()
	})
	if err != nil {
		return td.FailStep(ctx, err)
	}
	if parseErr != nil {
		return td.FailStep(ctx, parseErr)
	}
	return nil
}
