package bazel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/skerr"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
)

// Bazel provides a Task Driver API for working with Bazel (via the Bazelisk launcher, see
// https://github.com/bazelbuild/bazelisk).
type Bazel struct {
	rbeCredentialFile string
	workspace         string
}

type BazelOptions struct {
	CachePath           string
	RepositoryCachePath string
}

// EnsureBazelRCFile makes sure the user .bazelrc file exists and matches the provided
// configuration. This makes it easy for all subsequent calls to Bazel use the right command
// line args, even if Bazel is not invoked directly from task_driver (e.g. from a Makefile).
func EnsureBazelRCFile(ctx context.Context, bazelOpts BazelOptions) error {
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

	// https://docs.bazel.build/versions/main/guide.html#where-are-the-bazelrc-files
	// We go for the user's .bazelrc file instead of the system one because the
	// swarming user does not have access to write to /etc/bazel.bazelrc
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return skerr.Wrap(err)
	}
	userBazelRCLocation := filepath.Join(homeDir, ".bazelrc")
	bazelRCContent := strings.Join(bazelRCLines, "\n")
	return os_steps.WriteFile(ctx, userBazelRCLocation, []byte(bazelRCContent), 0666)
}

// New returns a new Bazel instance.
func New(ctx context.Context, workspace, rbeCredentialFile string, opts BazelOptions) (*Bazel, error) {
	if err := EnsureBazelRCFile(ctx, opts); err != nil {
		return nil, skerr.Wrap(err)
	}

	absCredentialFile := ""
	var err error
	if rbeCredentialFile != "" {
		absCredentialFile, err = os_steps.Abs(ctx, rbeCredentialFile)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return &Bazel{
		rbeCredentialFile: absCredentialFile,
		workspace:         workspace,
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
