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

func TestTestFail(t *testing.T) {
	unittest.MediumTest(t)

	d, cleanup, err := test2json.SetupTest(test2json.CONTENT_FAIL)
	require.NoError(t, err)
	defer cleanup()

	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		return Test(ctx, d, "./...")
	})

	found := map[string]bool{}
	res.Recurse(func(s *td.StepReport) bool {
		if s.Name == "fake-test-task" {
			return true
		}
		require.Equal(t, td.STEP_RESULT_FAILURE, s.Result, s.Name)
		found[s.Name] = true

		if s.Name == test2json.TestName {
			require.Equal(t, 1, len(s.Errors))
			require.True(t, strings.Contains(s.Errors[0], test2json.FailText))
		}
		// TODO(borenet): Verify that the test logs made it to the step.

		return true
	})
	require.True(t, found["go test --json ./..."])
	require.True(t, found[test2json.PackageFullPath])
	require.True(t, found[test2json.TestName])
}

func TestTestPass(t *testing.T) {
	unittest.MediumTest(t)

	d, cleanup, err := test2json.SetupTest(test2json.CONTENT_PASS)
	require.NoError(t, err)
	defer cleanup()

	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		return Test(ctx, d, "./...")
	})

	found := map[string]bool{}
	res.Recurse(func(s *td.StepReport) bool {
		if s.Name == "fake-test-task" {
			return true
		}
		require.Equal(t, td.STEP_RESULT_SUCCESS, s.Result, s.Name)
		found[s.Name] = true
		// TODO(borenet): Verify that the test logs made it to the step.
		return true
	})
	require.True(t, found["go test --json ./..."])
	require.True(t, found[test2json.PackageFullPath])
	require.True(t, found[test2json.TestName])
}

func TestTestSkip(t *testing.T) {
	unittest.MediumTest(t)

	d, cleanup, err := test2json.SetupTest(test2json.CONTENT_SKIP)
	require.NoError(t, err)
	defer cleanup()

	res := td.RunTestSteps(t, false, func(ctx context.Context) error {
		return Test(ctx, d, "./...")
	})

	found := map[string]bool{}
	res.Recurse(func(s *td.StepReport) bool {
		if s.Name == "fake-test-task" {
			return true
		}
		require.Equal(t, td.STEP_RESULT_SUCCESS, s.Result, s.Name)
		found[s.Name] = true
		// TODO(borenet): Verify that the test logs made it to the step.
		return true
	})
	require.True(t, found["go test --json ./..."])
	require.True(t, found[test2json.PackageFullPath])
	require.True(t, found[test2json.TestName])
}
