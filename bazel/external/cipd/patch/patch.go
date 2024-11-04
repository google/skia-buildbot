package patch

import (
	"os/exec"
	"path/filepath"
	"runtime"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/skerr"
)

// FindPatch returns the path to the Git binary found in the corresponding CIPD package.
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding CIPD package, which Bazel will download as needed.
func FindPatch() (string, error) {
	if !bazel.InBazel() {
		return exec.LookPath("patch")
	}
	if runtime.GOOS == "linux" {
		return filepath.Join(bazel.RunfilesDir(), "external", "patch_amd64_linux", "patch"), nil
	}
	return "", skerr.Fmt("unsupported runtime.GOOS: %q", runtime.GOOS)
}
