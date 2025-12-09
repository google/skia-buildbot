package cpython3

import (
	"path/filepath"
	"runtime"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/skerr"
)

// FindPythonBinary returns the path to the python binary found in the corresponding CIPD package.
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding CIPD package, which Bazel will download as needed.
func FindPythonBinary() (string, error) {
	if runtime.GOOS == "linux" {
		return filepath.Join(bazel.RunfilesDir(), "external", "_main~cipd~cpython3_amd64_linux", "bin", "python3.11"), nil
	}
	return "", skerr.Fmt("unsupported runtime.GOOS: %q", runtime.GOOS)
}
