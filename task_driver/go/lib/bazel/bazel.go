package bazel

import (
	"context"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

type Bazel struct {
	cacheDir          string
	local             bool
	rbeCredentialFile string
	workspace         string
}

func New(ctx context.Context, workspace string, local bool, rbeCredentialFile string) (*Bazel, func(), error) {
	// cacheDir is a temporary directory for the Bazel cache.
	//
	// We cannot use the default Bazel cache location ($HOME/.cache/bazel)
	// because:
	//
	//  - The cache can be large (>10G).
	//  - Swarming bots have limited storage space on the root partition (15G).
	//  - Because the above, the Bazel build fails with a "no space left on
	//    device" error.
	//  - The Bazel cache under $HOME/.cache/bazel lingers after the tryjob
	//    completes, causing the Swarming bot to be quarantined due to low disk
	//    space.
	//  - Generally, it's considered poor hygiene to leave a bot in a different
	//    state.
	//
	// The temporary directory created by the below function call lives under
	// /mnt/pd0, which has significantly more storage space, and will be wiped
	// after the tryjob completes.
	//
	// Reference: https://docs.bazel.build/versions/master/output_directories.html#current-layout.
	cacheDir, err := os_steps.TempDir(ctx, "", "bazel-user-cache-*")
	if err != nil {
		return nil, nil, err
	}
	absCredentialFile, err := os_steps.Abs(ctx, rbeCredentialFile)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		// Clean up the temporary Bazel cache directory when running locally,
		// because during development, we do not want to leave behind a ~10GB Bazel
		// cache directory under /tmp after each run.
		//
		// This is not necessary under Swarming because the temporary directory will
		// be cleaned up automatically.
		if local {
			if err := os_steps.RemoveAll(ctx, cacheDir); err != nil {
				td.Fatal(ctx, err)
			}
		}
	}
	return &Bazel{
		cacheDir:          cacheDir,
		rbeCredentialFile: absCredentialFile,
		workspace:         workspace,
	}, cleanup, nil
}

func (b *Bazel) Do(ctx context.Context, subCmd string, args ...string) (string, error) {
	cmd := []string{"bazel", "--output_user_root=" + b.cacheDir, subCmd}
	cmd = append(cmd, args...)
	return exec.RunCwd(ctx, b.workspace, cmd...)
}

func (b *Bazel) DoOnRBE(ctx context.Context, subCmd string, args ...string) (string, error) {
	cmd := []string{"--config=remote"}
	if b.rbeCredentialFile != "" {
		cmd = append(cmd, "--google_credentials="+b.rbeCredentialFile)
	}
	cmd = append(cmd, args...)
	return b.Do(ctx, subCmd, cmd...)
}
