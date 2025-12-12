package buildifier

import (
	"go.skia.org/infra/bazel/go/bazel"
)

var runfilePath = ""

// Find returns the path to the platform-specific buildifier binary downloaded by Bazel.
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding external Bazel repository.
func Find() (string, error) {
	return bazel.FindExecutable("buildifier", runfilePath)
}

// MustFind is like FindBuildifier, but it panics on any error.
func MustFind() string {
	p, err := Find()
	if err != nil {
		panic(err)
	}
	return p
}
