// Package docker is for running Dockerfiles.
package docker

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_driver/go/td"
)

func TestBuild(t *testing.T) {
	unittest.MediumTest(t)
	// Strip our PATH so we find our version of `docker` which is in the
	// testdata directory. Then add `/bin` to the PATH since we are running a
	// Bash shell.
	testDataDir, err := testutils.TestDataDir()
	require.NoError(t, err)
	dockerCmd = filepath.Join(testDataDir, "docker_mock")

	type args struct {
		tag string
	}
	tests := []struct {
		name          string
		args          args
		subSteps      int
		timeout       time.Duration
		expected      td.StepResult
		expectedSteps []td.StepReport
		wantErr       bool
		buildArgs     map[string]string
	}{
		{
			name: "success",
			args: args{
				tag: "success",
			},
			timeout:  time.Minute,
			expected: td.STEP_RESULT_SUCCESS,
			expectedSteps: []td.StepReport{
				{
					StepProperties: &td.StepProperties{
						Name: "Step 1/7 : FROM debian:testing-slim",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 2/7 : RUN apt-get update && apt-get upgrade -y && apt-get install -y   git   python    curl",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 3/7 : RUN mkdir -p --mode=0777 /workspace/__cache   && groupadd -g 2000 skia   && useradd -u 2000 -g 2000 --home /workspace/__cache skia",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 4/7 : ENV VPYTHON_VIRTUALENV_ROOT /workspace/__cache",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 5/7 : ENV CIPD_CACHE_DIR /workspace/__cache",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 6/7 : USER skia",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 7/7 : RUN printenv   && cd /tmp   && git clone 'https://chromium.googlesource.com/chromium/tools/depot_tools.git'   && mkdir -p /tmp/skia   && cd /tmp/skia   && export PATH=$PATH:/tmp/depot_tools   && touch noop.py   && vpython noop.py   && ls -al /tmp/depot_tools   && /tmp/depot_tools/fetch skia   && ls -al /workspace/__cache   && printenv",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
			},
			wantErr:   false,
			buildArgs: map[string]string{"arg1": "value1"},
		},
		{
			name: "failure",
			args: args{
				tag: "failure",
			},
			timeout:  time.Minute,
			expected: td.STEP_RESULT_FAILURE,
			expectedSteps: []td.StepReport{
				{
					StepProperties: &td.StepProperties{
						Name: "Step 1/7 : FROM debian:testing-slim",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 2/7 : RUN apt-get update && apt-get upgrade -y && apt-get install -y   git   python    curl",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 3/7 : RUN mkdir -p --mode=0777 /workspace/__cache   && groupadd -g 2000 skia   && useradd -u 2000 -g 2000 --home /workspace/__cache skia",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 4/7 : ENV VPYTHON_VIRTUALENV_ROOT /workspace/__cache",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 5/7 : ENV CIPD_CACHE_DIR /workspace/__cache",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 6/7 : USER skia",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 7/7 : RUN printenv   && cd /tmp   && git clone 'https://chromium.googlesource.com/chromium/tools/depot_tools.git'   && mkdir -p /tmp/skia   && cd /tmp/skia   && export PATH=$PATH:/tmp/depot_tools   && touch noop.py   && vpython noop.py   && ls -al /tmp/depot_tools   && /tmp/depot_tools/fetch skia   && ls -al /workspace/__cache   && printenv",
					},
					Result: td.STEP_RESULT_FAILURE,
				},
			},
			wantErr:   true,
			buildArgs: nil,
		},
		{
			name: "failure_no_output",
			args: args{
				tag: "failure_no_output",
			},
			timeout:       time.Minute,
			expected:      td.STEP_RESULT_FAILURE,
			expectedSteps: []td.StepReport{},
			wantErr:       true,
			buildArgs:     nil,
		},
		{
			name: "timeout",
			args: args{
				tag: "timeout",
			},
			timeout:  100 * time.Millisecond,
			expected: td.STEP_RESULT_FAILURE,
			expectedSteps: []td.StepReport{
				{
					StepProperties: &td.StepProperties{
						Name: "Step 1/7 : FROM debian:testing-slim",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 2/7 : RUN apt-get update && apt-get upgrade -y && apt-get install -y   git   python    curl",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 3/7 : RUN mkdir -p --mode=0777 /workspace/__cache   && groupadd -g 2000 skia   && useradd -u 2000 -g 2000 --home /workspace/__cache skia",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 4/7 : ENV VPYTHON_VIRTUALENV_ROOT /workspace/__cache",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 5/7 : ENV CIPD_CACHE_DIR /workspace/__cache",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 6/7 : USER skia",
					},
					Result: td.STEP_RESULT_SUCCESS,
				},
				{
					StepProperties: &td.StepProperties{
						Name: "Step 7/7 : RUN printenv   && cd /tmp   && git clone 'https://chromium.googlesource.com/chromium/tools/depot_tools.git'   && mkdir -p /tmp/skia   && cd /tmp/skia   && export PATH=$PATH:/tmp/depot_tools   && touch noop.py   && vpython noop.py   && ls -al /tmp/depot_tools   && /tmp/depot_tools/fetch skia   && ls -al /workspace/__cache   && printenv",
					},
					Result: td.STEP_RESULT_FAILURE,
				},
			},
			wantErr:   true,
			buildArgs: nil,
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

			if err := BuildHelper(ctx, ".", tt.args.tag, "test_config_dir", tt.buildArgs); (err != nil) != tt.wantErr {
				t.Errorf("Build() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Ensure that we got the expected step results.
			results := tr.EndRun(false, nil)

			// The root step is always successful, since we don't
			// do td.FailStep or td.Fatal.
			require.Equal(t, td.STEP_RESULT_SUCCESS, results.Result)

			// We should have exactly one sub-step, which is the
			// "docker run" as a whole.
			require.Equal(t, 1, len(results.Steps))
			dockerRun := results.Steps[0]
			require.Equal(t, tt.expected, results.Steps[0].Result)

			// Individual build steps.
			require.Equal(t, len(tt.expectedSteps), len(dockerRun.Steps))
			for idx, expectResult := range tt.expectedSteps {
				require.Equal(t, expectResult.Name, dockerRun.Steps[idx].Name)
				require.Equal(t, expectResult.Result, dockerRun.Steps[idx].Result)
			}
		})
	}

}

func TestLogin(t *testing.T) {
	unittest.SmallTest(t)

	_ = td.RunTestSteps(t, false, func(ctx context.Context) error {
		mockRun := &exec.CommandCollector{}
		mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
			require.Equal(t, dockerCmd, cmd.Name)
			require.Equal(t, []string{"--config", "test_config_dir", "login", "-u", "oauth2accesstoken", "--password-stdin", "https://gcr.io"}, cmd.Args)
			require.Equal(t, "", cmd.Dir)
			require.Equal(t, strings.NewReader("token"), cmd.Stdin)
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
			require.Equal(t, dockerCmd, cmd.Name)
			require.Equal(t, []string{"--config", "test_config_dir", "run", "--volume", "/tmp/test:/OUT", "--env", "SKIP_BUILD=1", "https://gcr.io/skia-public/skia-release:123", "test_cmd"}, cmd.Args)
			return nil
		})
		ctx = td.WithExecRunFn(ctx, mockRun.Run)

		err := Run(ctx, "https://gcr.io/skia-public/skia-release:123", "test_config_dir", []string{"test_cmd"}, []string{"/tmp/test:/OUT"}, []string{"SKIP_BUILD=1"})
		require.NoError(t, err)

		return nil
	})
}

func TestPull(t *testing.T) {
	unittest.SmallTest(t)

	_ = td.RunTestSteps(t, false, func(ctx context.Context) error {
		mockRun := &exec.CommandCollector{}
		mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
			require.Equal(t, dockerCmd, cmd.Name)
			require.Equal(t, []string{"--config", "test_config_dir", "pull", "https://gcr.io/skia-public/skia-release:123"}, cmd.Args)
			require.Equal(t, "", cmd.Dir)
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
			require.Equal(t, dockerCmd, cmd.Name)
			require.Equal(t, []string{"--config", "test_config_dir", "push", "https://gcr.io/skia-public/skia-release:123"}, cmd.Args)
			require.Equal(t, ".", cmd.Dir)
			_, err := cmd.CombinedOutput.Write([]byte(`The push refers to repository [gcr.io/skia-public/linux-run]
d75098a9b75c: Preparing
22f92a22a0a1: Preparing
7c18f43554d6: Preparing
c90191647a49: Preparing
22f92a22a0a1: Layer already exists
c90191647a49: Layer already exists
7c18f43554d6: Layer already exists
d75098a9b75c: Layer already exists
latest: digest: sha256:9f856122da361de5a737c4dd5b0ef582df3e051e6586d971a068e72657ddd0d2 size: 1166`))
			return err
		})
		ctx = td.WithExecRunFn(ctx, mockRun.Run)

		sha256, err := Push(ctx, "https://gcr.io/skia-public/skia-release:123", "test_config_dir")
		require.NoError(t, err)
		require.Equal(t, "sha256:9f856122da361de5a737c4dd5b0ef582df3e051e6586d971a068e72657ddd0d2", sha256)

		return nil
	})
}
