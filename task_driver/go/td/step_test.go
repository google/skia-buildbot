package td

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

func TestDefer(t *testing.T) {
	unittest.MediumTest(t)

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
		assert.Equal(t, 1, len(s.Exceptions))
		assert.Equal(t, "Caught panic: halp", s.Exceptions[0])
		got++
		return true
	})
	assert.Equal(t, 3, got)

	// Verify that our defer works properly.
	var id string
	res = RunTestSteps(t, false, func(ctx context.Context) error {
		// This is an example of a function which runs as a step.
		return Do(ctx, Props("parent"), func(ctx context.Context) error {
			return func(ctx context.Context) error {
				ctx = StartStep(ctx, Props("should fail"))
				defer EndStep(ctx)

				// Actual work would go here.
				id = getStep(ctx).Id
				err := fmt.Errorf("whoops")
				return FailStep(ctx, err)
			}(ctx)
		})
	})
	// The top-level step should not have inherited the sub-step result,
	// since we did not call FailStep for "parent".
	assert.Equal(t, STEP_RESULT_SUCCESS, res.Result)
	// Find the actual failed step, ensure that it has the error.
	s, err := res.findStep(id)
	assert.NoError(t, err)
	assert.Equal(t, STEP_RESULT_FAILURE, s.Result)
	assert.Equal(t, 1, len(s.Errors))
	assert.Equal(t, "whoops", s.Errors[0])
}

func TestExec(t *testing.T) {
	unittest.MediumTest(t)

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

func TestFatal(t *testing.T) {
	unittest.SmallTest(t)

	err := errors.New("FATAL")
	checkErr := func(s *StepReport) {
		assert.Equal(t, 1, len(s.Errors))
		assert.EqualError(t, err, s.Errors[0])
	}
	checkErrs := func(s *StepReport) {
		checkErr(s)
		s.Recurse(func(s *StepReport) bool {
			checkErr(s)
			return true
		})
	}
	checkExc := func(s *StepReport) {
		assert.Equal(t, 1, len(s.Exceptions))
		assert.EqualError(t, err, s.Exceptions[0])
	}
	checkExcs := func(s *StepReport) {
		checkExc(s)
		s.Recurse(func(s *StepReport) bool {
			checkExc(s)
			return true
		})
	}

	// When Fatal is called in a non-infra step, all parent steps get an error.
	s := RunTestSteps(t, true, func(ctx context.Context) error {
		return Do(ctx, nil, func(ctx context.Context) error {
			return Do(ctx, Props("non-infra step"), func(ctx context.Context) error {
				Fatal(ctx, err)
				return nil
			})
		})
	})
	checkErrs(s)

	// When Fatal is called in an infra step, all parent steps get an exception.
	s = RunTestSteps(t, true, func(ctx context.Context) error {
		return Do(ctx, nil, func(ctx context.Context) error {
			return Do(ctx, Props("infra step").Infra(), func(ctx context.Context) error {
				Fatal(ctx, err)
				return nil
			})
		})
	})
	checkExcs(s)

	// Check the case where we call Fatal() after a failed subprocess but
	// still want to perform deferred cleanup.
	ranCleanup := false
	s = RunTestSteps(t, true, func(ctx context.Context) error {
		defer func() {
			util.LogErr(Do(ctx, Props("cleanup").Infra(), func(ctx context.Context) error {
				ranCleanup = true
				return nil
			}))
		}()

		if _, err := exec.RunSimple(ctx, "false"); err != nil {
			Fatal(ctx, err)
			return err
		}
		return nil
	})
	assert.Equal(t, 1, len(s.Errors))
	assert.Equal(t, "Command exited with exit status 1: false; Stdout+Stderr:\n", s.Errors[0])
	assert.True(t, ranCleanup)

	// Check the case where we call Fatal() after an infra step failed whose
	// parent is not an infra step.
	s = RunTestSteps(t, true, func(ctx context.Context) error {
		return Do(ctx, Props("non-infra step"), func(ctx context.Context) error {
			if err := Do(ctx, Props("infra step").Infra(), func(ctx context.Context) error {
				return errors.New("Infra Failure")
			}); err != nil {
				Fatal(ctx, err)
			}
			return nil
		})
	})
	assert.Equal(t, 1, len(s.Exceptions))
	assert.Equal(t, "Infra Failure", s.Exceptions[0])
}
