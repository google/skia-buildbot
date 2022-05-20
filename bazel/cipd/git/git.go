package git

import (
	"path/filepath"
	"runtime"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/skerr"
)

// FindGit returns the path to the Git binary found in the corresponding CIPD package.
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding CIPD package, which Bazel will download as needed.
func FindGit() (string, error) {
	if runtime.GOOS == "windows" {
		return filepath.Join(bazel.RunfilesDir(), "external", "git_win", "bin", "git.exe"), nil
	} else if runtime.GOOS == "linux" {
		return filepath.Join(bazel.RunfilesDir(), "external", "git_linux", "bin", "git"), nil
	}
	return "", skerr.Fmt("unsupported runtime.GOOS: %q", runtime.GOOS)
}
