// Package docker is for running Dockerfiles.
package docker

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_driver/go/td"
)

func TestBuild(t *testing.T) {
	unittest.MediumTest(t)
	// Strip our PATH so we find our version of `docker` which is in the
	// test_bin directory. Then add `/bin` to the PATH since we are running a
	// Bash shell.
	_, filename, _, _ := runtime.Caller(0)
	dockerCmd = filepath.Join(filepath.Dir(filename), "test_bin", "docker_mock")

	type args struct {
		tag string
	}
	tests := []struct {
		name                 string
		args                 args
		subSteps             int
		timeout              time.Duration
		expected             td.StepResult
		expectedFirstSubStep td.StepResult
		wantErr              bool
		buildArgs            map[string]string
	}{
		{
			name: "success",
			args: args{
				tag: "success",
			},
			subSteps:             7,
			timeout:              time.Minute,
			expected:             td.STEP_RESULT_SUCCESS,
			expectedFirstSubStep: td.STEP_RESULT_SUCCESS,
			wantErr:              false,
			buildArgs:            map[string]string{"arg1": "value1"},
		},
		{
			name: "failure",
			args: args{
				tag: "failure",
			},
			subSteps:             0,
			timeout:              time.Minute,
			expected:             td.STEP_RESULT_SUCCESS,
			expectedFirstSubStep: td.STEP_RESULT_FAILURE,
			wantErr:              true,
			buildArgs:            nil,
		},
		{
			name: "failure_no_output",
			args: args{
				tag: "failure_no_output",
			},
			subSteps:             0,
			timeout:              time.Minute,
			expected:             td.STEP_RESULT_SUCCESS,
			expectedFirstSubStep: td.STEP_RESULT_FAILURE,
			wantErr:              true,
			buildArgs:            nil,
		},
		{
			name: "timeout",
			args: args{
				tag: "timeout",
			},
			subSteps:             0,
			timeout:              time.Millisecond,
			expected:             td.STEP_RESULT_SUCCESS,
			expectedFirstSubStep: td.STEP_RESULT_FAILURE,
			wantErr:              true,
			buildArgs:            nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := td.StartTestRun(t)
			defer tr.Cleanup()

			// Root-level step.
			ctx := tr.Root()

			// Add a timeout.
			ctx, cancel := context.WithTimeout(ctx, tt.timeout)
			defer cancel()

			if err := Build(ctx, ".", tt.args.tag, "test_config_dir", tt.buildArgs); (err != nil) != tt.wantErr {
				t.Errorf("Build() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Ensure that we got the expected step results.
			results := tr.EndRun(false, nil)

			assert.Equal(t, tt.subSteps, len(results.Steps[0].Steps))
			assert.Equal(t, tt.expected, results.Result)
			assert.Equal(t, tt.expectedFirstSubStep, results.Steps[0].Result)
		})
	}

}

func TestLogin(t *testing.T) {
	unittest.SmallTest(t)

	_ = td.RunTestSteps(t, false, func(ctx context.Context) error {
		mockRun := &exec.CommandCollector{}
		mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
			assert.Equal(t, dockerCmd, cmd.Name)
			assert.Equal(t, []string{"--config", "test_config_dir", "login", "-u", "oauth2accesstoken", "--password-stdin", "https://gcr.io"}, cmd.Args)
			assert.Equal(t, "", cmd.Dir)
			assert.Equal(t, strings.NewReader("token"), cmd.Stdin)
			return nil
		})
		ctx = td.WithExecRunFn(ctx, mockRun.Run)

		err := Login(ctx, "token", "https://gcr.io", "test_config_dir")
		require.NoError(t, err)

		return nil
	})
}

func TestRun(t *testing.T) {
	unittest.SmallTest(t)

	_ = td.RunTestSteps(t, false, func(ctx context.Context) error {
		mockRun := &exec.CommandCollector{}
		mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
			assert.Equal(t, dockerCmd, cmd.Name)
			assert.Equal(t, []string{"--config", "test_config_dir", "run", "--rm", "--volume", "/tmp/test:/OUT", "--env", "SKIP_BUILD=1", "https://gcr.io/skia-public/skia-release:123", "sh", "-c", "test_cmd"}, cmd.Args)
			return nil
		})
		ctx = td.WithExecRunFn(ctx, mockRun.Run)

		err := Run(ctx, "https://gcr.io/skia-public/skia-release:123", "test_cmd", "test_config_dir", []string{"/tmp/test:/OUT"}, map[string]string{"SKIP_BUILD": "1"})
		require.NoError(t, err)

		return nil
	})
}

func TestPull(t *testing.T) {
	unittest.SmallTest(t)

	_ = td.RunTestSteps(t, false, func(ctx context.Context) error {
		mockRun := &exec.CommandCollector{}
		mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
			assert.Equal(t, dockerCmd, cmd.Name)
			assert.Equal(t, []string{"--config", "test_config_dir", "pull", "https://gcr.io/skia-public/skia-release:123"}, cmd.Args)
			assert.Equal(t, "", cmd.Dir)
			return nil
		})
		ctx = td.WithExecRunFn(ctx, mockRun.Run)

		err := Pull(ctx, "https://gcr.io/skia-public/skia-release:123", "test_config_dir")
		require.NoError(t, err)

		return nil
	})
}

func TestPush(t *testing.T) {
	unittest.SmallTest(t)

	_ = td.RunTestSteps(t, false, func(ctx context.Context) error {
		mockRun := &exec.CommandCollector{}
		mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
			assert.Equal(t, dockerCmd, cmd.Name)
			assert.Equal(t, []string{"--config", "test_config_dir", "push", "https://gcr.io/skia-public/skia-release:123"}, cmd.Args)
			assert.Equal(t, "", cmd.Dir)
			return nil
		})
		ctx = td.WithExecRunFn(ctx, mockRun.Run)

		err := Push(ctx, "https://gcr.io/skia-public/skia-release:123", "test_config_dir")
		require.NoError(t, err)

		return nil
	})
}
