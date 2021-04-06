package bazel

import (
	"context"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
)

type Bazel struct {
	cacheDir          string
	local             bool
	rbe               bool
	rbeCredentialFile string
	workspace         string
}

func New(ctx context.Context, workspace string, local, rbe bool, rbeCredentialFile string) (*Bazel, error) {
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
		return nil, err
	}
	absCredentialFile, err := os_steps.Abs(ctx, rbeCredentialFile)
	if err != nil {
		return nil, err
	}
	return &Bazel{
		cacheDir:          cacheDir,
		rbe:               rbe,
		rbeCredentialFile: absCredentialFile,
		workspace:         workspace,
	}, nil
}

func (b *Bazel) Bazel(ctx context.Context, args ...string) (string, error) {
	cmd := []string{"bazel", "--output_user_root=" + b.cacheDir}
	cmd = append(cmd, args...)
	return exec.RunCwd(ctx, b.workspace, cmd...)
}

func (b *Bazel) remoteCmd(ctx context.Context, subcmd string, args ...string) error {
	cmd := []string{subcmd}
	if b.rbe {
		cmd = append(cmd, "--config=remote")
		if b.rbeCredentialFile != "" {
			cmd = append(cmd, "--google_credentials="+b.rbeCredentialFile)
		}
	}
	cmd = append(cmd, args...)
	_, err := b.Bazel(ctx, cmd...)
	return err
}

func (b *Bazel) Build(ctx context.Context, args ...string) error {
	return b.remoteCmd(ctx, "build", args...)
}

func (b *Bazel) Clean(ctx context.Context) error {
	_, err := b.Bazel(ctx, "clean")
	return err
}

func (b *Bazel) Test(ctx context.Context, args ...string) error {
	return b.remoteCmd(ctx, "test", args...)
}

func (b *Bazel) Cleanup(ctx context.Context) error {
	// Clean up the temporary Bazel cache directory when running locally,
	// because during development, we do not want to leave behind a ~10GB Bazel
	// cache directory under /tmp after each run.
	//
	// This is not necessary under Swarming because the temporary directory will
	// be cleaned up automatically.
	if b.local {
		if err := os_steps.RemoveAll(ctx, b.cacheDir); err != nil {
			return err
		}
	}
	return nil
}
