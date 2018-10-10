package task_driver

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestSteps(t *testing.T) {
	testutils.MediumTest(t)
	tr := InitForTesting(t)
	defer tr.Cleanup()

	parent := tr.Root()

	// Verify that we panic if steps aren't propertly started/stopped.
	s := parent.Step()
	validBeforeStart := []func(){
		func() {
			s.Name("hi")
		},
		func() {
			s.Infra()
		},
		func() {
			s.Env([]string{"K=V"})
		},
	}
	validAfterStart := []func(){
		func() {
			s.Data(nil)
		},
		func() {
			s.Ctx()
		},
		func() {
			s.HttpClient(http.DefaultClient)
		},
		func() {
			// Note that this changes the state of s, which
			// needs to be handled in the test cases below.
			s.Done(nil)
		},
	}
	// The step hasn't started yet.
	assert.False(t, s.IsRunning())
	assert.False(t, s.IsDone())
	for _, fn := range validBeforeStart {
		assert.NotPanics(t, fn)
	}
	for _, fn := range validAfterStart {
		assert.Panics(t, fn)
	}
	// The step is running.
	s.Start()
	assert.True(t, s.IsRunning())
	assert.False(t, s.IsDone())
	assert.Panics(t, func() {
		s.Start()
	})
	for _, fn := range validBeforeStart {
		assert.Panics(t, fn)
	}
	for _, fn := range validAfterStart {
		assert.NotPanics(t, fn)
	}
	// The last func marked the step as done. Now all funcs should panic.
	assert.False(t, s.IsRunning())
	assert.True(t, s.IsDone())
	for _, fn := range validBeforeStart {
		assert.Panics(t, fn)
	}
	for _, fn := range validAfterStart {
		assert.Panics(t, fn)
	}
}

func TestDefer(t *testing.T) {
	testutils.MediumTest(t)

	// Verify that our defer works properly.
	var id string
	res := RunTestSteps(t, func(s *Step) error {
		// This is an example of a function which runs as a step.
		return func(s *Step) (err error) {
			s = s.Step().Name("should fail").Start()
			defer s.Done(&err)

			// Actual work would go here.
			id = s.StepProperties.Id
			return fmt.Errorf("whoops")
		}(s)
	})
	// The top-level step should have inherited the sub-step result, since
	// we returned the error from the sub-step.
	assert.Equal(t, res.Result, STEP_RESULT_FAILED)
	assert.Equal(t, res.Error, "whoops")
	// Find the actual failed step, ensure that it has the error.
	s, err := res.findStep(id)
	assert.NoError(t, err)
	assert.NotNil(t, s.StepResult)
	assert.Equal(t, s.Result, STEP_RESULT_FAILED)
	assert.Equal(t, s.Error, "whoops")
}

func TestExec(t *testing.T) {
	testutils.MediumTest(t)

	// Basic tests around executing subprocesses.
	_ = RunTestSteps(t, func(s *Step) error {
		// Simple command.
		_, err := s.RunSimple("true")
		assert.NoError(t, err)

		// Verify that we get an error if the command fails.
		_, err = s.RunCwd(".", "false")
		assert.EqualError(t, err, "Command exited with exit status 1: false; Stdout+Stderr:\n")

		// Ensure that we collect stdout.
		out, err := s.RunCwd(".", "echo", "hello world")
		assert.NoError(t, err)
		assert.Equal(t, "hello world\n", out)

		// Ensure that we collect stdout and stderr.
		out, err = s.RunCwd(".", "python", "-c", "import sys; print 'stdout'; print >> sys.stderr, 'stderr'")
		assert.NoError(t, err)
		assert.True(t, strings.Contains(out, "stdout"))
		assert.True(t, strings.Contains(out, "stderr"))
		return nil
	})
}
