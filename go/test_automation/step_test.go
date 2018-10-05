package test_automation

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

func outputFile(wd string) string {
	return filepath.Join(wd, "output.json")
}

func setup(t *testing.T) (*Step, string, func()) {
	testutils.MediumTest(t)

	wd, cleanup := testutils.TempDir(t)
	s, err := Init("skia-fake-test-project", "fake-task-id", "fake-task-name", outputFile(wd), true)
	assert.NoError(t, err)
	return s, wd, cleanup
}

func runSteps(t *testing.T, fn func(*Step) error) *stepReport {
	s, wd, cleanup := setup(t)
	defer cleanup()

	err := fn(s)
	s.Done(&err)

	var rv stepReport
	assert.NoError(t, util.WithReadFile(outputFile(wd), func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&rv)
	}))
	return &rv
}

func TestSteps(t *testing.T) {
	parent, _, cleanup := setup(t)
	defer cleanup()
	defer parent.Done(nil)

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
	// Verify that our defer works properly.
	var id string
	res := runSteps(t, func(s *Step) error {
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
	// Basic tests around executing subprocesses.
	_ = runSteps(t, func(s *Step) error {
		// Simple command.
		_, err := s.Step().RunCwd(".", "true")
		assert.NoError(t, err)

		// Verify that we get an error if the command fails.
		_, err = s.Step().RunCwd(".", "false")
		assert.EqualError(t, err, "Command exited with exit status 1: false; Stdout+Stderr:\n")

		// Ensure that we collect stdout.
		out, err := s.Step().RunCwd(".", "echo", "hello world")
		assert.NoError(t, err)
		assert.Equal(t, "hello world\n", out)

		// Ensure that we collect stdout and stderr.
		out, err = s.Step().RunCwd(".", "python", "-c", "import sys; print 'stdout'; print >> sys.stderr, 'stderr'")
		assert.NoError(t, err)
		assert.True(t, strings.Contains(out, "stdout"))
		assert.True(t, strings.Contains(out, "stderr"))
		return nil
	})
}
