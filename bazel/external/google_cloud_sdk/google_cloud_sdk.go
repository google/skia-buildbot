package google_cloud_sdk

import (
	"go.skia.org/infra/bazel/go/bazel"
)

var runfilePath = ""

// Find returns the path to the `gcloud` binary downloaded by Bazel.
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding external Bazel repository.
func Find() (string, error) {
	return bazel.FindExecutable("gcloud", runfilePath)
}
