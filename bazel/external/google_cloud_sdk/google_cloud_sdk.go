package google_cloud_sdk

import (
	"path/filepath"
	"runtime"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/skerr"
)

// FindGcloud returns the path to the `gcloud` binary downloaded by Bazel.
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding external Bazel repository.
func FindGcloud() (string, error) {
	if runtime.GOOS == "linux" {
		return filepath.Join(bazel.RunfilesDir(), "external", "google_cloud_sdk", "google-cloud-sdk", "bin", "gcloud"), nil
	}
	return "", skerr.Fmt("unsupported runtime.GOOS: %q", runtime.GOOS)
}
