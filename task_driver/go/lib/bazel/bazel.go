package bazel

import (
	"context"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
)

type Bazel struct {
	cacheDir             string
	remoteCredentialFile string
	workspace            string
}

func New(ctx context.Context, workspace, remoteCredentialFile string) (*Bazel, error) {
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
	absCredentialFile, err := os_steps.Abs(ctx, remoteCredentialFile)
	if err != nil {
		return nil, err
	}
	return &Bazel{
		cacheDir:             cacheDir,
		remoteCredentialFile: absCredentialFile,
		workspace:            workspace,
	}, nil
}

func (b *Bazel) Bazel(ctx context.Context, args ...string) (string, error) {
	cmd := []string{"bazel", "--output_user_root=" + b.cacheDir}
	cmd = append(cmd, args...)
	return exec.RunCwd(ctx, b.workspace, cmd...)
}

func (b *Bazel) Build(ctx context.Context, args ...string) error {
	cmd := []string{"build"}
	if b.remoteCredentialFile != "" {
		cmd = append(cmd,
			"--config=remote",
			"--google_credentials="+b.remoteCredentialFile,
		)
	}
	cmd = append(cmd, args...)
	_, err := b.Bazel(ctx, cmd...)
	return err
}

func (b *Bazel) Clean(ctx context.Context) error {
	_, err := b.Bazel(ctx, "clean")
	return err
}
