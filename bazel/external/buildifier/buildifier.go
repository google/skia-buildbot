package buildifier

import (
	"os"
	"path/filepath"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/skerr"
)

// FindBuildifier returns the path to the platform-specific buildifier binary downloaded by Bazel.
//
// Calling this function from any Go package will automatically establish a Bazel dependency on the
// corresponding external Bazel repository.
func FindBuildifier() (string, error) {
	buildifierPath, err := filepath.Abs(filepath.Join(bazel.RunfilesDir(),
		"external", "buildifier_prebuilt~", "buildifier", "buildifier"))
	if err != nil {
		return "", skerr.Wrap(err)
	}
	if _, err := os.Stat(buildifierPath); err != nil {
		return "", skerr.Wrapf(err, "Are you running this binary via Bazel? ")
	}
	return buildifierPath, nil
}

// MustFindBuildifier is like FindBuildifier, but it panics on any error.
func MustFindBuildifier() string {
	p, err := FindBuildifier()
	if err != nil {
		panic(err)
	}
	return p
}
