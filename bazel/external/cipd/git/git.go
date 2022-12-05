package git

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/skerr"
)

// FindGit returns the path to the Git binary found in the corresponding CIPD package.
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding CIPD package, which Bazel will download as needed.
func FindGit() (string, error) {
	if !bazel.InBazelTest() {
		return exec.LookPath("git")
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(bazel.RunfilesDir(), "external", "git_amd64_windows", "bin", "git.exe"), nil
	} else if runtime.GOOS == "linux" {
		return filepath.Join(bazel.RunfilesDir(), "external", "git_amd64_linux", "bin", "git"), nil
	}
	return "", skerr.Fmt("unsupported runtime.GOOS: %q", runtime.GOOS)
}

func UseGitFinder(ctx context.Context) context.Context {
	return git_common.WithGitFinder(ctx, FindGit)
}
