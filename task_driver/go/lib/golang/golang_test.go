package golang

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
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

func TestTest(t *testing.T) {
	unittest.SmallTest(t)

	pkg := "my-pkg"
	type testCase struct {
		events []test2json.Event
		expect *td.StepReport
	}
	testCases := []testCase{
		{
			events: []test2json.Event{
				{
					Action:  test2json.ACTION_RUN,
					Package: pkg,
					Test:    "MyTest",
				},
				{
					Action:  test2json.ACTION_RUN,
					Package: pkg,
					Test:    "Test2",
				},
				{
					Action:  test2json.ACTION_FAIL,
					Package: pkg,
					Test:    "Test2",
				},
				{
					Action:  test2json.ACTION_PASS,
					Package: pkg,
					Test:    "MyTest",
				},
				{
					Action:  test2json.ACTION_FAIL,
					Package: pkg,
				},
			},
			expect: &td.StepReport{
				StepProperties: &td.StepProperties{
					Name: "fake-test-task",
				},
				Result: td.STEP_RESULT_SUCCESS,
				Steps: []*td.StepReport{
					{
						StepProperties: &td.StepProperties{
							Name: "go test --json 0",
						},
						Result: td.STEP_RESULT_SUCCESS,
						Steps: []*td.StepReport{
							{
								StepProperties: &td.StepProperties{
									Name: pkg,
								},
							},
						},
					},
				},
			},
		},
	}

	wd := "."
	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		idx, err := strconv.Atoi(cmd.Args[len(cmd.Args)-1])
		require.NoError(t, err)
		events := testCases[idx].events
		for _, e := range events {
			sklog.Errorf("%s %s %s", e.Action, e.Package, e.Test)
			b, err := json.Marshal(e)
			require.NoError(t, err)
			_, err = cmd.Stdout.Write(append(b, byte('\n')))
			require.NoError(t, err)
		}
		return nil
	})

	for idx, tc := range testCases {
		res := td.RunTestSteps(t, false, func(ctx context.Context) error {
			ctx = td.WithExecRunFn(ctx, mockRun.Run)
			return Test(ctx, wd, strconv.Itoa(idx))
		})
		// Sanitize the results.
		res.Recurse(func(sr *td.StepReport) bool {
			sr.Id = ""
			sr.Data = nil
			sr.Environ = nil
			sr.Parent = ""
			return true
		})
		assertdeep.Equal(t, tc.expect, res)
	}

}
