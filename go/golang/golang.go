package golang

import (
	"os/exec"

	"go.skia.org/infra/bazel/external/rules_go"
	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/skerr"
)

// FindGo returns the path to the `go` binary. When running under Bazel
// (`bazel run //path/to:target` or `bazel test //path/to:target`), it will return the path to
// the `go` binary downloaded by rules_go[1]. Otherwise, it will return the path to the host
// system's `go` binary. We aim to remove all uses of the system's `go` binary, in favor of the
// binary downloaded by Bazel.
//
// [1] https://github.com/bazelbuild/rules_go
func FindGo() (string, error) {
	if bazel.InBazelTest() {
		goBin, err := rules_go.FindGo()
		if err != nil {
			return "", skerr.Wrapf(err, "Failed to find go in Bazel runfiles")
		}
		return goBin, nil
	}

	goBin, err := exec.LookPath("go")
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to find go in $PATH")
	}
	return goBin, nil
}
