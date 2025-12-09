package rules_go

import (
	"path/filepath"
	"runtime"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/skerr"
)

// FindGo returns the path to the `go` binary provided by rules_go[1].
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding external Bazel repository.
//
// [1] https://github.com/bazelbuild/rules_go
func FindGo() (string, error) {
	if runtime.GOOS == "windows" {
		return filepath.Join(bazel.RunfilesDir(), "external", "rules_go~~go_sdk~go_sdk", "bin", "go.exe"), nil
	} else if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		return filepath.Join(bazel.RunfilesDir(), "external", "rules_go~~go_sdk~go_sdk", "bin", "go"), nil
	}
	return "", skerr.Fmt("unsupported runtime.GOOS: %q", runtime.GOOS)
}
