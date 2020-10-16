package td

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

// mockExec mocks out subprocesses named "true" with a success result and all
// others with a failure. Returns the new context and a counter indicating how
// many times the run function was called.
func mockExec(ctx context.Context) (context.Context, *int) {
	mockRun := &exec.CommandCollector{}
	runCount := 0
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		runCount++
		if cmd.Name == "true" {
			return nil
		}
		return errors.New("Command exited with exit status 1: ")
	})
	return WithExecRunFn(ctx, mockRun.Run), &runCount
}

func TestDefer(t *testing.T) {
	unittest.MediumTest(t)

	// Verify that we handle panics properly.
	res := RunTestSteps(t, true, func(ctx context.Context) error {
		panic("halp")
	})
	require.Equal(t, res.Result, STEP_RESULT_EXCEPTION)
	res = RunTestSteps(t, true, func(ctx context.Context) error {
		return Do(ctx, nil, func(ctx context.Context) error {
			return Do(ctx, nil, func(ctx context.Context) error {
				panic("halp")
			})
		})
	})
	got := 0
	res.Recurse(func(s *StepReport) bool {
		require.Equal(t, s.Result, STEP_RESULT_EXCEPTION)
		require.Equal(t, 1, len(s.Exceptions))
		require.Equal(t, "Caught panic: halp", s.Exceptions[0])
		got++
		return true
	})
	require.Equal(t, 3, got)

	// Verify that our defer works properly.
	var id string
	res = RunTestSteps(t, false, func(ctx context.Context) error {
		// This is an example of a function which runs as a step.
		return Do(ctx, Props("parent"), func(ctx context.Context) error {
			return func(ctx context.Context) error {
				ctx = StartStep(ctx, Props("should fail"))
				defer EndStep(ctx)

				// Actual work would go here.
				id = getCtx(ctx).step.Id
				err := fmt.Errorf("whoops")
				return FailStep(ctx, err)
			}(ctx)
		})
	})
	// The top-level step should not have inherited the sub-step result,
	// since we did not call FailStep for "parent".
	require.Equal(t, STEP_RESULT_SUCCESS, res.Result)
	// Find the actual failed step, ensure that it has the error.
	s, err := res.findStep(id)
	require.NoError(t, err)
	require.Equal(t, STEP_RESULT_FAILURE, s.Result)
	require.Equal(t, 1, len(s.Errors))
	require.Equal(t, "whoops", s.Errors[0])
}

func TestExec(t *testing.T) {
	unittest.MediumTest(t)

	// Basic tests around executing subprocesses.
	_ = RunTestSteps(t, false, func(ctx context.Context) error {
		mockExecCtx, counter := mockExec(ctx)

		// Simple command.
		_, err := exec.RunSimple(mockExecCtx, "true")
		require.NoError(t, err)
		require.Equal(t, 1, *counter)

		// Verify that we get an error if the command fails.
		_, err = exec.RunCwd(mockExecCtx, ".", "false")
		require.Contains(t, err.Error(), "Command exited with exit status 1: ")
		require.Equal(t, 2, *counter)

		// Ensure that we collect stdout.
		out, err := exec.RunCwd(ctx, ".", "python", "-c", "print 'hello world'")
		require.NoError(t, err)
		require.True(t, strings.Contains(out, "hello world"))
		require.Equal(t, 2, *counter) // Not using the mock for this test case.

		// Ensure that we collect stdout and stderr.
		out, err = exec.RunCwd(ctx, ".", "python", "-c", "import sys; print 'stdout'; print >> sys.stderr, 'stderr'")
		require.NoError(t, err)
		require.True(t, strings.Contains(out, "stdout"))
		require.True(t, strings.Contains(out, "stderr"))
		require.Equal(t, 2, *counter) // Not using the mock for this test case.
		return nil
	})
}

func TestFatal(t *testing.T) {
	unittest.SmallTest(t)

	err := errors.New("FATAL")
	checkErr := func(s *StepReport) {
		require.Equal(t, 1, len(s.Errors))
		require.EqualError(t, err, s.Errors[0])
	}
	checkErrs := func(s *StepReport) {
		checkErr(s)
		s.Recurse(func(s *StepReport) bool {
			checkErr(s)
			return true
		})
	}
	checkExc := func(s *StepReport) {
		require.Equal(t, 1, len(s.Exceptions))
		require.EqualError(t, err, s.Exceptions[0])
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
		ctx, _ = mockExec(ctx)
		if _, err := exec.RunSimple(ctx, "false"); err != nil {
			Fatal(ctx, err)
			return err
		}
		return nil
	})
	require.Equal(t, 1, len(s.Errors))
	require.Contains(t, s.Errors[0], "Command exited with exit status 1: ")
	require.True(t, ranCleanup)

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
	require.Equal(t, 1, len(s.Exceptions))
	require.Equal(t, "Infra Failure", s.Exceptions[0])
}

func TestEnv(t *testing.T) {
	unittest.MediumTest(t)

	// Verify that each step inherits the environment of its parent.
	s := RunTestSteps(t, false, func(ctx context.Context) error {
		return Do(ctx, Props("a").Env([]string{"a=a"}), func(ctx context.Context) error {
			return Do(ctx, Props("b").Env([]string{"b=b"}), func(ctx context.Context) error {
				_, err := exec.RunCommand(ctx, &exec.Command{
					Name: "python",
					Args: []string{"-c", "print 'hello world'"},
					Env:  []string{"c=c"},
				})
				return err
			})
		})
	})
	var leaf *StepReport
	s.Recurse(func(s *StepReport) bool {
		if len(s.Steps) == 0 {
			leaf = s
			return false
		}
		return true
	})
	require.NotNil(t, leaf)
	expect := MergeEnv(os.Environ(), BASE_ENV)
	expect = append(expect, "a=a", "b=b", "c=c")
	assertdeep.Equal(t, expect, leaf.StepProperties.Environ)

	var data *ExecData
	for _, d := range leaf.Data {
		ed, ok := d.(*ExecData)
		if ok {
			data = ed
			break
		}
	}
	require.NotNil(t, data)
	assertdeep.Equal(t, data.Env, expect)
}

func TestEnvMerge(t *testing.T) {
	unittest.SmallTest(t)

	tc := []struct {
		a      []string
		b      []string
		expect []string
	}{
		// Unrelated variables both show up.
		{
			expect: []string{"a=a", "b=b"},
			a:      []string{"a=a"},
			b:      []string{"b=b"},
		},
		// The second env takes precedence over the first.
		{
			expect: []string{"k=v2"},
			a:      []string{"k=v1"},
			b:      []string{"k=v2"},
		},

		// PATH gets special treatment.

		// If only one is specified, it gets preserved.
		{
			expect: []string{"PATH=p2"},
			a:      []string{},
			b:      []string{"PATH=p2"},
		},
		{
			expect: []string{"PATH=p1"},
			a:      []string{"PATH=p1"},
			b:      []string{},
		},
		// The second env takes precedence over the first.
		{
			expect: []string{"PATH=p2"},
			a:      []string{"PATH=p1"},
			b:      []string{"PATH=p2"},
		},
		// ... even if the second env defines it to be empty.
		{
			expect: []string{"PATH="},
			a:      []string{"PATH=p1"},
			b:      []string{"PATH="},
		},
		// If provided, PATH_PLACEHOLDER gets replaced by PATH from the first.
		{
			expect: []string{"PATH=p1:p2"},
			a:      []string{"PATH=p1"},
			b:      []string{fmt.Sprintf("PATH=%s:p2", PATH_PLACEHOLDER)},
		},
		{
			expect: []string{"PATH=p2:p1"},
			a:      []string{"PATH=p1"},
			b:      []string{fmt.Sprintf("PATH=p2:%s", PATH_PLACEHOLDER)},
		},
		// There's no good reason to do this, but it would work.
		{
			expect: []string{"PATH=p1:p1"},
			a:      []string{"PATH=p1"},
			b:      []string{fmt.Sprintf("PATH=%s:%s", PATH_PLACEHOLDER, PATH_PLACEHOLDER)},
		},
	}

	for _, c := range tc {
		require.Equal(t, c.expect, MergeEnv(c.a, c.b))
	}
}

func TestEnvInheritance(t *testing.T) {
	unittest.SmallTest(t)

	// Set up exec mock and expectations.
	runCount := 0
	expect := MergeEnv(os.Environ(), BASE_ENV)
	expect = append(expect, "a=a", "b=b", "c=c", "d=d")
	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		runCount++
		require.Equal(t, expect, cmd.Env)
		return nil
	})

	// Verify that environments are inherited properly.
	require.Equal(t, 0, runCount)
	s := RunTestSteps(t, false, func(ctx context.Context) error {
		ctx = WithExecRunFn(ctx, mockRun.Run)
		return Do(ctx, Props("a").Env([]string{"a=a", "b=a"}), func(ctx context.Context) error {
			ctx = WithEnv(ctx, []string{"b=b", "c=b"})
			return Do(ctx, Props("c").Env([]string{"c=c", "d=c"}), func(ctx context.Context) error {
				_, err := exec.RunCommand(ctx, &exec.Command{
					Name: "true",
					Env:  []string{"d=d"},
				})
				return err
			})
		})
	})
	require.Equal(t, 1, runCount)
	var leaf *StepReport
	s.Recurse(func(s *StepReport) bool {
		if len(s.Steps) == 0 {
			leaf = s
			return false
		}
		return true
	})
	require.NotNil(t, leaf)
	assertdeep.Equal(t, expect, leaf.StepProperties.Environ)

	var data *ExecData
	for _, d := range leaf.Data {
		ed, ok := d.(*ExecData)
		if ok {
			data = ed
			break
		}
	}
	require.NotNil(t, data)
	assertdeep.Equal(t, data.Env, expect)

	// Verify that multiple invocations of WithEnv get merged.
	require.Equal(t, 1, runCount)
	s = RunTestSteps(t, false, func(ctx context.Context) error {
		ctx = WithExecRunFn(ctx, mockRun.Run)
		ctx = WithEnv(ctx, []string{"a=a", "b=a"})
		ctx = WithEnv(ctx, []string{"b=b", "c=b"})
		ctx = WithEnv(ctx, []string{"c=c", "d=c"})
		_, err := exec.RunCommand(ctx, &exec.Command{
			Name: "true",
			Env:  []string{"d=d"},
		})
		return err
	})
	require.Equal(t, 2, runCount)
	leaf = nil
	s.Recurse(func(s *StepReport) bool {
		if len(s.Steps) == 0 {
			leaf = s
			return false
		}
		return true
	})
	require.NotNil(t, leaf)
	assertdeep.Equal(t, expect, leaf.StepProperties.Environ)

	data = nil
	for _, d := range leaf.Data {
		ed, ok := d.(*ExecData)
		if ok {
			data = ed
			break
		}
	}
	require.NotNil(t, data)
	assertdeep.Equal(t, data.Env, expect)
}

func TestMustGetAbsolutePathOfFlag_NonEmptyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	wd, err := os.Getwd()
	require.NoError(t, err)
	assert.NotEmpty(t, wd)

	s := RunTestSteps(t, false, func(ctx context.Context) error {
		path := MustGetAbsolutePathOfFlag(ctx, "my_dir", "some_flag")

		assert.Contains(t, path, wd)
		assert.Contains(t, path, "my_dir")
		return nil
	})
	assert.Empty(t, s.Errors)
	assert.Empty(t, s.Exceptions)
}

func TestMustGetAbsolutePathOfFlag_EmptyPath_Panics(t *testing.T) {
	unittest.SmallTest(t)

	s := RunTestSteps(t, true, func(ctx context.Context) error {
		MustGetAbsolutePathOfFlag(ctx, "", "some_flag")
		assert.Fail(t, "should not reach here")
		return nil
	})
	assert.Empty(t, s.Exceptions)
	require.Len(t, s.Errors, 1)
	assert.Contains(t, s.Errors[0], "some_flag must")
}
