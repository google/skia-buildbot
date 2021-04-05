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

func (b *Bazel) Cleanup(ctx context.Context) error {
	return os_steps.RemoveAll(ctx, b.cacheDir)
}
