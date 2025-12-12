package rules_go

import (
	"go.skia.org/infra/bazel/go/bazel"
)

var runfilePath = ""

// Find returns the path to the `go` binary provided by rules_go[1].
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding external Bazel repository.
//
// [1] https://github.com/bazelbuild/rules_go
func Find() (string, error) {
	return bazel.FindExecutable("go", runfilePath)
}
