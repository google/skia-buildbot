package golang

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
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
			assert.True(t, util.In(fmt.Sprintf("GOCACHE=%s", filepath.Join(dirs.Cache(wd), "go_cache")), cmd.Env))
			assert.True(t, util.In("GOFLAGS=-mod=readonly", cmd.Env))
			assert.True(t, util.In(fmt.Sprintf("GOROOT=%s", filepath.Join("go", "go")), cmd.Env))
			assert.True(t, util.In(fmt.Sprintf("GOPATH=%s", filepath.Join(wd, "gopath")), cmd.Env))

			// We don't override any default vars except PATH.
			for _, v := range td.BASE_ENV {
				if !strings.HasPrefix(v, "PATH=") {
					assert.True(t, util.In(v, cmd.Env))
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
			assert.NotEqual(t, PATH, "")
			pathSplit := strings.Split(strings.SplitN(PATH, "=", 2)[1], string(os.PathListSeparator))
			expectPaths := []string{
				filepath.Join(wd, "go", "go", "bin"),
				filepath.Join(wd, "gopath", "bin"),
				filepath.Join(wd, "gcloud_linux", "bin"),
				filepath.Join(wd, "protoc", "bin"),
				filepath.Join(wd, "node", "node", "bin"),
			}
			for _, expectPath := range expectPaths {
				assert.True(t, util.In(expectPath, pathSplit))
			}
			return nil
		})
		ctx = td.WithExecRunFn(ctx, mockRun.Run)

		_, err := exec.RunCwd(ctx, wd, "true")
		assert.NoError(t, err)
		assert.Equal(t, 1, runCount)

		_, err = Go(ctx, "blahblah")
		assert.NoError(t, err)
		assert.Equal(t, 2, runCount)

		return nil
	})
}
