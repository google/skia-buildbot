package task_driver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils"
)

func TestDefer(t *testing.T) {
	testutils.MediumTest(t)

	// Verify that we handle panics properly.
	res := RunTestSteps(t, true, func(ctx context.Context) error {
		panic("halp")
	})
	assert.Equal(t, res.Result, STEP_RESULT_EXCEPTION)
	res = RunTestSteps(t, true, func(ctx context.Context) error {
		return Do(ctx, nil, func(ctx context.Context) error {
			return Do(ctx, nil, func(ctx context.Context) error {
				panic("halp")
			})
		})
	})
	got := 0
	res.Recurse(func(s *StepReport) bool {
		assert.Equal(t, s.Result, STEP_RESULT_EXCEPTION)
		got++
		return true
	})
	assert.Equal(t, 3, got)

	// Verify that our defer works properly.
	var id string
	res = RunTestSteps(t, false, func(ctx context.Context) error {
		// This is an example of a function which runs as a step.
		return Do(ctx, Opts(Name("parent")), func(ctx context.Context) error {
			return func(ctx context.Context) (err error) {
				ctx = StartStep(ctx, Name("should fail"))
				defer FinishStep(ctx, &err)

				// Actual work would go here.
				id = getStep(ctx).Id
				return fmt.Errorf("whoops")
			}(ctx)
		})
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
	_ = RunTestSteps(t, false, func(ctx context.Context) error {
		// Simple command.
		_, err := exec.RunSimple(ctx, "true")
		assert.NoError(t, err)

		// Verify that we get an error if the command fails.
		_, err = exec.RunCwd(ctx, ".", "false")
		assert.EqualError(t, err, "Command exited with exit status 1: false; Stdout+Stderr:\n")

		// Ensure that we collect stdout.
		out, err := exec.RunCwd(ctx, ".", "echo", "hello world")
		assert.NoError(t, err)
		assert.Equal(t, "hello world\n", out)

		// Ensure that we collect stdout and stderr.
		out, err = exec.RunCwd(ctx, ".", "python", "-c", "import sys; print 'stdout'; print >> sys.stderr, 'stderr'")
		assert.NoError(t, err)
		assert.True(t, strings.Contains(out, "stdout"))
		assert.True(t, strings.Contains(out, "stderr"))
		return nil
	})
}
