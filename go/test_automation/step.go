package test_automation

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pborman/uuid"
	"go.skia.org/infra/go/exec"
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
	Id      string `json:"id"`
	Name    string `json:"name"`
	IsInfra bool   `json:"isInfra"`
	Parent  string `json:"parent,omitempty"`
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
	if len(name) > MAX_STEP_NAME_CHARS {
		name = name[:MAX_STEP_NAME_CHARS]
	}
	s.StepProperties.Name = name
	return s
}

// Mark the Step as infra-specific.
func (s *Step) Infra() *Step {
	s.IsInfra = true
	return s
}

// Start the Step.
func (s *Step) Start() *Step {
	s.run.emitter.Start(s)
	s.result = nil
	return s
}

// Return true iff the Step has been started and has not yet finished.
func (s *Step) IsRunning() bool {
	return s.result == nil
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
//	defer s.Done()
//
// After Done() is called, no more work can be associated with this Step.
func (s *Step) Done() {
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
		s.finish(&StepResult{
			Result: STEP_RESULT_SUCCESS,
		})
	}
}

// Mark the Step as failed with the given error. After Fail() is called, no more
// work can be associated with this step. Returns the passed-in error so that
// callers can do:
//
//	if err := doSomething(); err != nil {
//		return s.Fail(err)
//	}
//
func (s *Step) Fail(err error) error {
	s.finish(&StepResult{
		Result: STEP_RESULT_FAILED,
		Error:  err.Error(),
	})
	return err
}

// Attach the given Data to this Step. The Step must be running.
func (s *Step) Data(d interface{}) *Step {
	if !s.IsRunning() {
		panic("Data() called on Step which is not running")
	}
	s.run.emitter.AddStepData(s.Id, d)
	return s
}

// execData is extra Step data generated when executing commands through the
// exec package.
type execData struct {
	Cmd []string `json:"command"`
}

// Return a context.Context associated with this Step. Any calls to exec which
// use this Context will be attached to the Step.
func (s *Step) Ctx() context.Context {
	return exec.NewContext(context.Background(), func(cmd *exec.Command) error {
		s.Data(&execData{
			Cmd: append([]string{cmd.Name}, cmd.Args...),
		})
		return exec.DefaultRun(cmd)
		// TODO(borenet): stdout/stderr should probably get added as
		// step data. Better yet, we should stream the logs as they're
		// written. Not sure if we should do that by sending lots of
		// messages, or if we should have a separate log receiver.
	})
}

// httpData is extra Step data generated when sending HTTP requests.
type httpData http.Request

// httpTransport is an http.RoundTripper which wraps another http.RoundTripper
// to record data about the requests it sends.
type httpTransport struct {
	s  *Step
	rt http.RoundTripper
}

// See documentation for http.RoundTripper.
func (t *httpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.s.Data(req)
	// TODO(borenet): Record response?
	return t.rt.RoundTrip(req)
}

// Return an http.Client which wraps the given http.Client to record data about
// the requests it sends.
func (s *Step) HttpClient(c *http.Client) *http.Client {
	c.Transport = &httpTransport{
		s:  s,
		rt: c.Transport,
	}
	return c
}

// Do is a convenience function which runs the given function as a Step. It
// handles calls to Start(), Done(), and Fail(), in the case of a returned error.
func (s *Step) Do(fn func(*Step) error) error {
	s.Start()
	defer s.Done()
	err := fn(s)
	if err != nil {
		s.Fail(err)
	}
	return err
}
