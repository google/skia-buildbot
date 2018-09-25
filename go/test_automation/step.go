package test_automation

import (
	"context"
	"fmt"

	"github.com/pborman/uuid"
)

const (
	MAX_STEP_NAME_CHARS = 100

	STEP_RESULT_SUCCESS     = "SUCCESS"
	STEP_RESULT_FAILED      = "FAILED"
	STEP_RESULT_EXCEPTION   = "EXCEPTION"
	STEP_RESULT_NOT_STARTED = "NOT_STARTED"
)

// Step represents a single action taken inside of a test automation run.
type Step struct {
	Id       string
	StepName string
	IsInfra  bool

	result *StepResult
	run    *Run
	data   []interface{}
}

// StepResult contains the results of a Step.
type StepResult struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// Create a new Step.
func (r *Run) Step() *Step {
	return &Step{
		Id: uuid.New(), // TODO(borenet): Come up with a more systematic ID.
		result: &StepResult{
			Result: STEP_RESULT_NOT_STARTED,
			Error:  "Step not yet started.",
		},
		run: r,
	}
}

// Apply the given name to the Step.
func (s *Step) Name(name string) *Step {
	s.StepName = name
	if len(s.StepName) > MAX_STEP_NAME_CHARS {
		s.StepName = s.StepName[:MAX_STEP_NAME_CHARS]
	}
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
	if s.result != nil {
		panic("finish() called on Step which is not running")
	}
	s.result = res
	s.run.emitter.Finish(s.result)
}

// Mark the Step as finished. If the step has already been finished, eg. via
// Fail(), no action is taken. This is intended to be used in a defer, eg.
//
//	s := r.Step().Start()
//	defer s.Done()
//
// After Done() is called, no more work can be associated with this Step.
func (s *Step) Done() {
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
// work can be associated with this step.
func (s *Step) Fail(err error) {
	s.finish(&StepResult{
		Result: STEP_RESULT_FAILED,
		Error:  err.Error(),
	})
}

// Attach the given Data to this Step. The Step must be running.
func (s *Step) Data(d interface{}) *Step {
	if !s.IsRunning() {
		panic("Data() called on Step which is not running")
	}
	s.data = append(s.data, d)
	s.run.emitter.AddStepData(d)
	return s
}

// Return a context.Context associated with this Step. Any calls to exec which
// use this Context will be attached to the Step.
func (s *Step) Ctx() context.Context {
	return s.run.Ctx()
}

// Do is a convenience function which runs the given function as a Step. It
// handles calls to Start(), Done(), and Fail(), in the case of a returned error.
func (s *Step) Do(fn func() error) error {
	s.Start()
	defer s.Done()
	err := fn()
	if err != nil {
		s.Fail(err)
	}
	return err
}
