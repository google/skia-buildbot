// Package docker is for running Dockerfiles.
package docker

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
			subSteps:             7,
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
			subSteps:             0,
			timeout:              time.Minute,
			expected:             td.STEP_RESULT_SUCCESS,
			expectedFirstSubStep: td.STEP_RESULT_FAILURE,
			wantErr:              true,
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

			assert.Equal(t, tt.subSteps, len(results.Steps[0].Steps))
			assert.Equal(t, tt.expected, results.Result)
			assert.Equal(t, tt.expectedFirstSubStep, results.Steps[0].Result)
		})
	}

}
