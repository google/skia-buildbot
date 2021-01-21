package golang

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/test2json"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/lib/dirs"
	"go.skia.org/infra/task_driver/go/td"
)

func TestWithEnv(t *testing.T) {
	unittest.SmallTest(t)

	_ = td.RunTestSteps(t, false, func(ctx context.Context) error {
		// Mock the run function with one which asserts that it has the
		// correct environment.
		wd := "."
		ctx = WithEnv(ctx, wd)
		mockRun := &exec.CommandCollector{}
		runCount := 0
		mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
			runCount++

			// Misc variables.
			require.True(t, util.In(fmt.Sprintf("GOCACHE=%s", filepath.Join(dirs.Cache(wd), "go_cache")), cmd.Env))
			require.True(t, util.In("GOFLAGS=-mod=readonly", cmd.Env))
			require.True(t, util.In(fmt.Sprintf("GOROOT=%s", filepath.Join("go", "go")), cmd.Env))
			require.True(t, util.In(fmt.Sprintf("GOPATH=%s", filepath.Join(wd, "gopath")), cmd.Env))

			// We don't override any default vars except PATH.
			for _, v := range td.BASE_ENV {
				if !strings.HasPrefix(v, "PATH=") {
					require.True(t, util.In(v, cmd.Env))
				}
			}

			// PATH.
			PATH := ""
			for _, v := range cmd.Env {
				if strings.HasPrefix(v, "PATH=") {
					PATH = v
					break
				}
			}
			require.NotEqual(t, PATH, "")
			pathSplit := strings.Split(strings.SplitN(PATH, "=", 2)[1], string(os.PathListSeparator))
			expectPaths := []string{
				filepath.Join(wd, "go", "go", "bin"),
				filepath.Join(wd, "gopath", "bin"),
				filepath.Join(wd, "gcloud_linux", "bin"),
				filepath.Join(wd, "protoc", "bin"),
				filepath.Join(wd, "node", "node", "bin"),
			}
			for _, expectPath := range expectPaths {
				require.True(t, util.In(expectPath, pathSplit))
			}
			return nil
		})
		ctx = td.WithExecRunFn(ctx, mockRun.Run)

		_, err := exec.RunCwd(ctx, wd, "true")
		require.NoError(t, err)
		require.Equal(t, 1, runCount)

		_, err = Go(ctx, "blahblah")
		require.NoError(t, err)
		require.Equal(t, 2, runCount)

		return nil
	})
}

// executeAndExpectResult runs "go test" with the given test content (see
// test2json package for examples) and ensures that the "go test" command and
// all of its descendants have the given expected result.
func executeAndExpectResult(t *testing.T, content test2json.TestContent, expectResult td.StepResult) *td.StepReport {
	// For compatibility with Bazel: the "go" command fails if HOME is not set.
	testutils.SetUpFakeHomeDir(t, "golang_test")

	d, cleanup, err := test2json.SetupTest(content)
	require.NoError(t, err)
	defer cleanup()

	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		return Test(ctx, d, "./...")
	})

	found := map[string]bool{}
	res.Recurse(func(s *td.StepReport) bool {
		// Ignore the root-level step.
		if s.Name == "fake-test-task" {
			return true
		}
		require.Equal(t, expectResult, s.Result, s.Name)
		found[s.Name] = true
		// TODO(borenet): Verify that the test logs made it to the step.
		return true
	})
	require.True(t, found["go test --json ./..."])
	require.True(t, found[test2json.PackageFullPath])
	require.True(t, found[test2json.TestName])
	return res
}

func TestRunTestSteps_FailingTestHasFailureResult(t *testing.T) {
	unittest.MediumTest(t)

	res := executeAndExpectResult(t, test2json.ContentFail, td.STEP_RESULT_FAILURE)

	// Verify that the step for the failed test has the expected log snippet
	// in its Errors field.
	res.Recurse(func(s *td.StepReport) bool {
		if s.Name == test2json.TestName {
			require.Equal(t, 1, len(s.Errors))
			require.True(t, strings.Contains(s.Errors[0], test2json.FailText))
		}
		return true
	})
}

func TestRunTestSteps_PassingTestHasSuccessResult(t *testing.T) {
	unittest.MediumTest(t)

	executeAndExpectResult(t, test2json.ContentPass, td.STEP_RESULT_SUCCESS)
}

func TestRunTestSteps_SkippedTestHasSuccessResult(t *testing.T) {
	unittest.MediumTest(t)

	executeAndExpectResult(t, test2json.ContentPass, td.STEP_RESULT_SUCCESS)
}

func TestRunTestSteps_NestedTestHasSuccessResultAndNestedSteps(t *testing.T) {
	unittest.MediumTest(t)

	res := executeAndExpectResult(t, test2json.ContentNested, td.STEP_RESULT_SUCCESS)

	// Verify that we have the correct tree of steps.

	// Root step, created by td.RunTestSteps.
	rootStep := res
	require.Equal(t, rootStep.Name, "fake-test-task")
	require.Len(t, rootStep.Steps, 1)

	// "go test" step.
	goTestStep := res.Steps[0]
	require.Equal(t, goTestStep.Name, "go test --json ./...")
	require.Len(t, goTestStep.Steps, 1)

	// Package step.
	pkgStep := goTestStep.Steps[0]
	require.Equal(t, pkgStep.Name, test2json.PackageFullPath)
	require.Len(t, pkgStep.Steps, 1)

	// Test step.
	testStep := pkgStep.Steps[0]
	require.Equal(t, testStep.Name, test2json.TestName)
	require.Len(t, testStep.Steps, 1)

	// Nested step 1.
	nestedStep1 := testStep.Steps[0]
	require.Equal(t, nestedStep1.Name, "1")
	require.Len(t, nestedStep1.Steps, 1)

	// Nested step 2.
	nestedStep2 := nestedStep1.Steps[0]
	require.Equal(t, nestedStep2.Name, "2")
	require.Len(t, nestedStep2.Steps, 1)

	// Nested step 3.
	nestedStep3 := nestedStep2.Steps[0]
	require.Equal(t, nestedStep3.Name, "3")
	require.Len(t, nestedStep3.Steps, 0)
}
