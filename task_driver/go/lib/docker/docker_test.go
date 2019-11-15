// Package docker is for running Dockerfiles.
package docker

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
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
	}{
		{
			name: "success",
			args: args{
				tag: "success",
			},
			subSteps:             8,
			timeout:              time.Minute,
			expected:             td.STEP_RESULT_SUCCESS,
			expectedFirstSubStep: td.STEP_RESULT_SUCCESS,
			wantErr:              false,
		},
		{
			name: "failure",
			args: args{
				tag: "failure",
			},
			subSteps:             1,
			timeout:              time.Minute,
			expected:             td.STEP_RESULT_SUCCESS,
			expectedFirstSubStep: td.STEP_RESULT_EXCEPTION,
			wantErr:              true,
		},
		{
			name: "failure_no_output",
			args: args{
				tag: "failure_no_output",
			},
			subSteps:             1,
			timeout:              time.Minute,
			expected:             td.STEP_RESULT_SUCCESS,
			expectedFirstSubStep: td.STEP_RESULT_EXCEPTION,
			wantErr:              true,
		},
		{
			name: "timeout",
			args: args{
				tag: "timeout",
			},
			subSteps:             1,
			timeout:              time.Millisecond,
			expected:             td.STEP_RESULT_SUCCESS,
			expectedFirstSubStep: td.STEP_RESULT_EXCEPTION,
			wantErr:              true,
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

			if err := Build(ctx, ".", tt.args.tag); (err != nil) != tt.wantErr {
				t.Errorf("Build() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Ensure that we got the expected step results.
			results := tr.EndRun(false, nil)
			fmt.Println("========EXPECTED========")
			fmt.Println(tt.subSteps)
			fmt.Println(tt.expected)
			fmt.Println(tt.expectedFirstSubStep)
			fmt.Println("========RESULTS========")
			fmt.Println(results.Result)
			fmt.Println(results.Steps)
			for _, s := range results.Steps {
				fmt.Println(s.Name)
				fmt.Println(s.Steps)
				fmt.Println(s.Result)
			}

			//require.Equal(t, stepResult.Result, expect[idx])
			assert.Equal(t, tt.subSteps, len(results.Steps))
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
			assert.Equal(t, []string{"login", "-u", "oauth2accesstoken", "-p", "token", "https://gcr.io"}, cmd.Args)
			assert.Equal(t, "", cmd.Dir)
			return nil
		})
		ctx = td.WithExecRunFn(ctx, mockRun.Run)

		err := Login(ctx, "token", "https://gcr.io")
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
			assert.Equal(t, []string{"push", "https://gcr.io/skia-public/skia-release:123"}, cmd.Args)
			assert.Equal(t, "", cmd.Dir)
			return nil
		})
		ctx = td.WithExecRunFn(ctx, mockRun.Run)

		err := Push(ctx, "https://gcr.io/skia-public/skia-release:123")
		require.NoError(t, err)

		return nil
	})
}
