package rules_python

import (
	"go.skia.org/infra/bazel/go/bazel"
)

var runfilePath = ""

// Find returns the path to the `python3` binary provided by rules_python[1].
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding external Bazel repository.
//
// [1] https://github.com/bazelbuild/rules_python
func Find() (string, error) {
	return bazel.FindExecutable("python3", runfilePath)
}
