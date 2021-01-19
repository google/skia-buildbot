package repo_root

import (
	"fmt"
	"os"
	"strings"

	"go.skia.org/infra/bazel/go/bazel"
)

// Get returns the path to the workspace's root directory.
//
// Under Bazel, it returns the path to the runfiles directory. Test targets must include any
// required files under their "data" attribute for said files to be included in the runfiles
// directory.
//
// Outside of Bazel, it returns the path to the repo checkout's root directory.  Note that this will
// return an error if the CWD is not inside a checkout, so this cannot run on production servers.
func Get() (string, error) {
	if bazel.InBazel() {
		return bazel.RunfilesDir(), nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	prefixes := []string{"go.skia.org/infra", "buildbot"}
	for _, prefix := range prefixes {
		if strings.Contains(dir, prefix) {
			return strings.Split(dir, prefix)[0] + prefix, nil
		}
	}
	// If this function is used outside of tests, please remove the following.
	if d := os.Getenv("WORKSPACE_DIR"); d != "" {
		return d, nil
	}
	return "", fmt.Errorf("No repo root found; are we running inside a checkout?")
}
