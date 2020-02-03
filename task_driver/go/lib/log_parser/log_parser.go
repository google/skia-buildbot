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
)

// Step represents a step in a tree of steps generated during Run.
type Step struct {
	ctx      context.Context
	log      io.Writer
	logBuf   *ring.StringRing
	name     string
	parent   *Step
	children map[string]*Step
}

// newStep returns a Step instance.
func newStep(ctx context.Context, name string, log io.Writer, parent *Step) *Step {
	logBuf, err := ring.NewStringRing(OUTPUT_LINES)
	if err != nil {
		// NewStringRing only returns an error if OUTPUT_LINES is invalid.
		panic(err)
	}
	return &Step{
		ctx:      ctx,
		log:      log,
		logBuf:   logBuf,
		name:     name,
		parent:   parent,
		children: map[string]*Step{},
	}
}

// StartChild creates a new step as a direct child of this step.
func (s *Step) StartChild(props *td.StepProperties) *Step {
	ctx := td.StartStep(s.ctx, props)
	log := td.NewLogStream(ctx, "stdout+stderr", td.Info)
	child := newStep(ctx, props.Name, log, s)
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
	// TODO(borenet): We may not be scanning lines, so joining with newlines
	// may not produce the correct output.
	msg := strings.Join(s.logBuf.GetAll(), "\n")
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

// Log writes the given log for this step and all of its ancestors.
func (s *Step) Log(b []byte) {
	_, err := s.log.Write(b)
	if err != nil {
		sklog.Errorf("Failed to write logs for step %q: %s", s.name, err)
	}
	s.logBuf.Put(string(b))
	if s.parent != nil {
		s.parent.Log(b)
	}
}

// Leaves returns all active Steps with no children.
func (s *Step) Leaves() []*Step {
	if len(s.children) == 0 {
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
func newStepManager(root context.Context, log io.Writer) *StepManager {
	return &StepManager{
		root: newStep(root, "", log, nil),
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

// Run runs the given command in the given working directory. It calls the
// provided function to emit sub-steps.
func Run(ctx context.Context, cwd string, cmdLine []string, split bufio.SplitFunc, handleToken func(*StepManager, string) error) error {
	ctx = td.StartStep(ctx, td.Props(strings.Join(cmdLine, " ")))
	defer td.EndStep(ctx)

	// Set up the command.
	cmd := exec.CommandContext(ctx, cmdLine[0], cmdLine[1:]...)
	cmd.Dir = cwd
	cmd.Env = td.GetEnv(ctx)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return td.FailStep(ctx, err)
	}
	// TODO(borenet): There's a good chance that the output of subprocesses
	// is racy, which could cause us to attribute log output to the wrong
	// step.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return td.FailStep(ctx, err)
	}

	stream := func(r io.Reader) <-chan string {
		tokens := make(chan string)
		go func() {
			scanner := bufio.NewScanner(r)
			scanner.Split(split)
			for scanner.Scan() {
				tokens <- scanner.Text()
			}
			close(tokens)
		}()
		return tokens
	}
	stdoutTokens := stream(stdout)
	stderrTokens := stream(stderr)

	// Start the command.
	if err := cmd.Start(); err != nil {
		return td.FailStep(ctx, skerr.Wrapf(err, "Failed to start command"))
	}

	// parseErr records any errors that occur while parsing output.
	var parseErr error

	// Parse the output of the command and create sub-steps.
	scanner := bufio.NewScanner(stdout)
	scanner.Split(split)
	sm := newStepManager(ctx, td.NewLogStream(ctx, "stdout+stderr", td.Info))
	for {
		var token string
		select {
		case tok, ok := <-stdoutTokens:
			if ok {
				token = tok
			} else {
				stdoutTokens = nil
			}
		case tok, ok := <-stderrTokens:
			if ok {
				token = tok
			} else {
				stderrTokens = nil
			}
		}
		if token != "" {
			sm.root.Log([]byte(token))
			if err := handleToken(sm, token); err != nil {
				parseErr = skerr.Wrapf(err, "Failed handling token %q", token)
				sklog.Error(parseErr.Error())
			}
		}
		if stdoutTokens == nil && stderrTokens == nil {
			break
		}
	}

	// Wait for the command to finish.
	err = cmd.Wait()
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
