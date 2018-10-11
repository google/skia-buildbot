package task_driver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/pborman/uuid"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	MAX_STEP_NAME_CHARS = 100

	STEP_RESULT_SUCCESS     = "SUCCESS"
	STEP_RESULT_FAILED      = "FAILED"
	STEP_RESULT_EXCEPTION   = "EXCEPTION"
	STEP_RESULT_NOT_STARTED = "NOT_STARTED"
)

// StepProperties are basic properties of a Step.
type StepProperties struct {
	Id      string   `json:"id"`
	Name    string   `json:"name"`
	IsInfra bool     `json:"isInfra"`
	Env     []string `json:"environment,omitempty"`
	Parent  string   `json:"parent,omitempty"`
}

// Step represents a single action taken inside of a test automation run.
type Step struct {
	*StepProperties
	result *StepResult
	run    *run
}

// StepResult contains the results of a Step.
type StepResult struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// Return a Step instance.
func newStep(id string, r *run, parent *Step) *Step {
	s := &Step{
		StepProperties: &StepProperties{
			Id: id,
		},
		result: &StepResult{
			Result: STEP_RESULT_NOT_STARTED,
			Error:  "Step not yet started.",
		},
		run: r,
	}
	if parent != nil {
		s.StepProperties.Env = parent.StepProperties.Env
		s.Parent = parent.Id
	}
	return s
}

// Create a new Step.
func (s *Step) Step() *Step {
	// TODO(borenet): Come up with a more systematic ID.
	return newStep(uuid.New(), s.run, s)
}

// Apply the given name to the Step.
func (s *Step) Name(name string) *Step {
	if s.IsRunning() || s.IsDone() {
		panic("Cannot modify a Step once it is running.")
	}
	if len(name) > MAX_STEP_NAME_CHARS {
		name = name[:MAX_STEP_NAME_CHARS]
	}
	s.StepProperties.Name = name
	return s
}

// Mark the Step as infra-specific.
func (s *Step) Infra() *Step {
	if s.IsRunning() || s.IsDone() {
		panic("Cannot modify a Step once it is running.")
	}
	s.IsInfra = true
	return s
}

// Apply the given environment variables to all commands run within this Step.
// Note that this does NOT apply the variables to the environment of this
// process, just of subprocesses spawned using s.Ctx().
func (s *Step) Env(env []string) *Step {
	if s.IsRunning() || s.IsDone() {
		panic("Cannot modify a Step once it is running.")
	}
	s.StepProperties.Env = env // TODO(borenet): Merge environments?
	return s
}

// Start the Step.
func (s *Step) Start() *Step {
	if s.IsRunning() {
		panic("Start() called on step which is already running.")
	}
	s.run.emitter.Start(s)
	s.result = nil
	return s
}

// Return true iff the Step has been started and has not yet finished.
func (s *Step) IsRunning() bool {
	return s.result == nil
}

// Return true iff the step has already finished.
func (s *Step) IsDone() bool {
	return s.result != nil && s.result.Result != STEP_RESULT_NOT_STARTED
}

// Mark the Step as finished with the given StepResult. After finish() is
// called, no more work can be associated with this Step.
func (s *Step) finish(res *StepResult) {
	if !s.IsRunning() {
		panic("finish() called on Step which is not running")
	}
	s.result = res
	s.run.emitter.Finish(s.Id, s.result)
}

// Mark the Step as finished. If the step has already been finished, eg. via
// Fail(), no action is taken. This is intended to be used in a defer, eg.
//
//	s := r.Step().Start()
//	defer s.Done(&err)
//
// After Done() is called, no more work can be associated with this Step.
func (s *Step) Done(err *error) {
	defer func() {
		if s.Id == STEP_ID_ROOT {
			s.run.Done()
		}
	}()
	if r := recover(); r != nil {
		s.finish(&StepResult{
			Result: STEP_RESULT_EXCEPTION,
			Error:  fmt.Sprintf("Caught panic: %s", r),
		})
		panic(r)
	} else if s.IsRunning() {
		if err != nil && *err != nil {
			s.finish(&StepResult{
				Result: STEP_RESULT_FAILED,
				Error:  (*err).Error(),
			})
		} else {
			s.finish(&StepResult{
				Result: STEP_RESULT_SUCCESS,
			})
		}
	} else {
		panic("Done() called on Step which is not running")
	}
}

// Attach the given Data to this Step. The Step must be running.
func (s *Step) Data(d interface{}) *Step {
	if !s.IsRunning() {
		panic("Data() called on Step which is not running")
	}
	s.run.emitter.AddStepData(s.Id, d)
	return s
}

// Do is a convenience function which runs the given function as a Step. It
// handles calls to Start() and Done() as appropriate.
func (s *Step) Do(fn func(*Step) error) (err error) {
	s.Start()
	defer s.Done(&err)
	return fn(s)
}

// execData is extra Step data generated when executing commands through the
// exec package.
type execData struct {
	Cmd []string `json:"command"`
	Env []string `json:"env,omitempty"`
}

// logData is extra Step data generated for log streams.
type logData struct {
	Name     string `json:"name"`
	Id       string `json:"id"`
	Severity string `json:"severity"`
	Log      string `json:"log,omitempty"`
}

// Create an io.Writer that will act as a log stream for this Step. Callers
// probably want to use a higher-level method instead.
func (s *Step) NewLogStream(name, severity string) io.Writer {
	id := uuid.New() // TODO(borenet): Come up with a better ID.
	stream := s.run.emitter.LogStream(s.Id, id, severity)
	// Emit step data for the log stream.
	s.Data(&logData{
		Name:     name,
		Id:       id,
		Severity: severity,
	})
	return stream
}

// Return a context.Context associated with this Step. Any calls to exec which
// use this Context will be attached to the Step.
func (s *Step) Ctx() context.Context {
	if !s.IsRunning() {
		panic("Ctx() called on Step which is not running")
	}
	return exec.NewContext(context.Background(), func(cmd *exec.Command) error {
		name := strings.Join(append([]string{cmd.Name}, cmd.Args...), " ")
		return s.Step().Name(name).Do(func(s *Step) error {
			// Inherit env from the step unless it's explicitly provided.
			// TODO(borenet): Should we merge instead?
			if cmd.Env == nil {
				cmd.Env = s.StepProperties.Env
			}

			// Set up stdout and stderr streams.
			stdout := s.NewLogStream("stdout", sklog.INFO)
			if cmd.Stdout != nil {
				stdout = util.MultiWriter([]io.Writer{cmd.Stdout, stdout})
			}
			cmd.Stdout = stdout
			stderr := s.NewLogStream("stderr", sklog.ERROR)
			if cmd.Stderr != nil {
				stderr = util.MultiWriter([]io.Writer{cmd.Stderr, stderr})
			}
			cmd.Stderr = stderr

			// Collect step metadata about the command.
			d := &execData{
				Cmd: append([]string{cmd.Name}, cmd.Args...),
				Env: cmd.Env,
			}
			s.Data(d)

			// Run the command.
			return exec.DefaultRun(cmd)
		})
	})
}

// httpTransport is an http.RoundTripper which wraps another http.RoundTripper
// to record data about the requests it sends.
type httpTransport struct {
	s  *Step
	rt http.RoundTripper
}

// httpRequestData is Step data describing an http.Request. Notably, it does not
// include the request body or headers, to avoid leaking auth tokens or other
// sensitive information.
type httpRequestData struct {
	Method string   `json:"method,omitempty"`
	URL    *url.URL `json:"url,omitempty"`
}

// httpResponseData is Step data describing an http.Response. Notably, it does
// not include the response body, to avoid leaking sensitive information.
type httpResponseData struct {
	StatusCode int `json:"status,omitempty"`
}

// See documentation for http.RoundTripper.
func (t *httpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	return resp, t.s.Step().Name(req.URL.String()).Do(func(s *Step) error {
		s.Data(&httpRequestData{
			Method: req.Method,
			URL:    req.URL,
		})
		var err error
		resp, err = t.rt.RoundTrip(req)
		if resp != nil {
			s.Data(&httpResponseData{
				StatusCode: resp.StatusCode,
			})
		}
		return err
	})
}

// Return an http.Client which wraps the given http.Client to record data about
// the requests it sends.
func (s *Step) HttpClient(c *http.Client) *http.Client {
	if !s.IsRunning() {
		panic("HttpClient called on Step which is not running")
	}
	if c == nil {
		c = http.DefaultClient // TODO(borenet): Use backoff client?
	}
	if c.Transport == nil {
		c.Transport = http.DefaultTransport
	}
	c.Transport = &httpTransport{
		s:  s,
		rt: c.Transport,
	}
	return c
}

// RunCwd is a convenience wrapper around exec.RunCwd which runs the given
// command as a Step. It handles calls to Start() and Done() as appropriate.
// TODO(borenet): Is this really needed?
func (s *Step) RunCwd(cwd string, command ...string) (string, error) {
	return exec.RunCwd(s.Ctx(), cwd, command...)
}

// RunCommand is a convenience wrapper around exec.Run which runs the given
// Command as a Step. It handles calls to Start() and Done() as appropriate.
// TODO(borenet): Is this really needed?
func (s *Step) RunCommand(cmd *exec.Command) error {
	return exec.Run(s.Ctx(), cmd)
}

// RunSimple is a convenience wrapper around exec.RunSimple which runs the given
// command string as a Step. It handles calls to Start() and Done() as
// appropriate.
// TODO(borenet): Is this really needed?
func (s *Step) RunSimple(command string) (string, error) {
	return exec.RunSimple(s.Ctx(), command)
}
