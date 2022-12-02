package cockroachdb

import (
	"os/exec"
	"path/filepath"
	"runtime"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/skerr"
)

// FindCockroach returns the path to the `cockroach` binary downloaded by Bazel.
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding external Bazel repository.
func FindCockroach() (string, error) {
	if !bazel.InBazelTest() {
		return exec.LookPath("cockroach")
	}
	if runtime.GOOS == "linux" {
		return filepath.Join(bazel.RunfilesDir(), "external", "cockroachdb_linux", "cockroach"), nil
	}
	return "", skerr.Fmt("unsupported runtime.GOOS: %q", runtime.GOOS)
}
