package repo_root

import (
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/skerr"
)

// Get returns the path to the workspace's root directory.
//
// Under Bazel, it returns the path to the runfiles directory. Test targets must
// include any required files under their "data" attribute for said files to be
// included in the runfiles directory.
//
// Outside of Bazel, it returns the path to the repo checkout's root directory.
// Note that this will return an error if the CWD is not inside a checkout, so
// this cannot run on production servers.
func Get() (string, error) {
	if bazel.InBazelTest() {
		return bazel.RunfilesDir(), nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", skerr.Wrap(err)
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
	return "", skerr.Fmt("No repo root found; are we running inside a checkout?")
}

// GetLocal returns the path to the root of the current Git repo. Only intended
// to be run on a developer machine, inside a Git checkout.
func GetLocal() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", skerr.Wrap(err)
	}
	for {
		gitDir := filepath.Join(cwd, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return cwd, nil
		}
		newCwd, err := filepath.Abs(filepath.Join(cwd, ".."))
		if err != nil {
			return "", skerr.Wrap(err)
		}
		if newCwd == cwd {
			return "", skerr.Fmt("No repo root found up to %s; are we running inside a checkout?", cwd)
		}
		cwd = newCwd
	}
}
