package bazel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
)

// Bazel provides a Task Driver API for working with Bazel (via the Bazelisk launcher, see
// https://github.com/bazelbuild/bazelisk).
type Bazel struct {
	cachePath           string
	rbeCredentialFile   string
	repositoryCachePath string
	workspace           string
	cleanup             func()
}

type BazelOptions struct {
	CachePath           string
	RepositoryCachePath string
}

// OverrideHomeAndWriteBazelRC masks the user .bazelrc file using the provided
// configuration by overriding HOME. This makes it easy for all subsequent calls
// to Bazel use the right command line args, even if Bazel is not invoked
// directly from task_driver (e.g. from a Makefile). Returns a function which
// should be run deferred for cleanup.
func OverrideHomeAndWriteBazelRC(ctx context.Context, bazelOpts BazelOptions) (func(), error) {
	homeDir, err := os.MkdirTemp("", "bazel-task-driver-tmp-home-")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	oldHome := os.Getenv("HOME")
	cleanup := func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			sklog.Errorf("Failed to set original HOME environment variable: %s", err)
		}
		if err := os_steps.RemoveAll(ctx, homeDir); err != nil {
			sklog.Errorf("Failed to cleanup temporary Bazel HOME directory: %s", err)
		}
	}
	if err := os.Setenv("HOME", homeDir); err != nil {
		cleanup()
		return nil, skerr.Wrap(err)
	}
	userBazelRCLocation := filepath.Join(homeDir, ".bazelrc")
	if err := WriteBazelRC(ctx, userBazelRCLocation, bazelOpts); err != nil {
		cleanup()
		return nil, skerr.Wrap(err)
	}
	return cleanup, nil
}

// WriteBazelRC writes the given BazelOptions to the given file path.
func WriteBazelRC(ctx context.Context, path string, bazelOpts BazelOptions) error {
	var bazelRCLines []string
	if bazelOpts.CachePath != "" {
		// https://docs.bazel.build/versions/main/output_directories.html#current-layout
		bazelRCLines = append(bazelRCLines, fmt.Sprintf("startup --output_user_root=%s", bazelOpts.CachePath))
	}
	if bazelOpts.RepositoryCachePath != "" {
		// Also keep the repository cache on disk so that it survives reboots.
		// https://bazel.build/docs/build#repository-cache
		bazelRCLines = append(bazelRCLines, fmt.Sprintf("build --repository_cache=%s", bazelOpts.RepositoryCachePath))
	}
	bazelRCContent := strings.Join(bazelRCLines, "\n")
	return os_steps.WriteFile(ctx, path, []byte(bazelRCContent), 0666)
}

// New returns a new Bazel instance.
func New(ctx context.Context, workspace, rbeCredentialFile string, opts BazelOptions) (*Bazel, error) {
	cleanup, err := OverrideHomeAndWriteBazelRC(ctx, opts)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	absCredentialFile := ""
	if rbeCredentialFile != "" {
		absCredentialFile, err = os_steps.Abs(ctx, rbeCredentialFile)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return &Bazel{
		cachePath:           opts.CachePath,
		rbeCredentialFile:   absCredentialFile,
		repositoryCachePath: opts.RepositoryCachePath,
		workspace:           workspace,
		cleanup:             cleanup,
	}, nil
}

// Do executes a Bazel subcommand.
func (b *Bazel) Do(ctx context.Context, subCmd string, args ...string) (string, error) {
	cmd := []string{"bazelisk", subCmd}
	cmd = append(cmd, args...)
	return exec.RunCwd(ctx, b.workspace, cmd...)
}

// DoOnRBE executes a Bazel subcommand on RBE.
func (b *Bazel) DoOnRBE(ctx context.Context, subCmd string, args ...string) (string, error) {
	// See https://bazel.build/reference/command-line-reference
	cmd := []string{
		"--config=remote",
		"--sandbox_base=/dev/shm", // Make builds faster by using a RAM disk for the sandbox.
	}
	if b.rbeCredentialFile != "" {
		cmd = append(cmd, "--google_credentials="+b.rbeCredentialFile)
	} else {
		cmd = append(cmd, "--google_default_credentials")
	}
	cmd = append(cmd, args...)
	return b.Do(ctx, subCmd, cmd...)

}

// Cleanup must be run after the caller is finished with this Bazel instance.
func (b *Bazel) Cleanup() {
	b.cleanup()
}
