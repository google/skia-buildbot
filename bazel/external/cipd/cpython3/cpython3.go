package cpython3

import (
	"path/filepath"
	"runtime"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/skerr"
)

// FindPython311 returns the path to the python3.8 binary found in the corresponding CIPD package.
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding CIPD package, which Bazel will download as needed.
func FindPython311() (string, error) {
	if runtime.GOOS == "linux" {
		return filepath.Join(bazel.RunfilesDir(), "external", "cpython3_amd64_linux", "bin", "python3.11"), nil
	}
	return "", skerr.Fmt("unsupported runtime.GOOS: %q", runtime.GOOS)
}
