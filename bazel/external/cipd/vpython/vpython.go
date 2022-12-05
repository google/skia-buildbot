package vpython

import (
	"path/filepath"
	"runtime"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/skerr"
)

// FindVPython3 returns the path to the vpython3 binary found in the corresponding CIPD package.
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding CIPD package, which Bazel will download as needed.
func FindVPython3() (string, error) {
	if runtime.GOOS == "linux" {
		return filepath.Join(bazel.RunfilesDir(), "external", "vpython_amd64_linux", "vpython3"), nil
	}
	return "", skerr.Fmt("unsupported runtime.GOOS: %q", runtime.GOOS)
}
