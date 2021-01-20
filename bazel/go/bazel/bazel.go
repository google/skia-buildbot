// Package bazel contains utilities for Go tests running under Bazel.
package bazel

import (
	"os"
	"path/filepath"
)

// InBazel returns true if running under Bazel (e.g. "bazel test", "bazel run"), or false otherwise.
func InBazel() bool {
	// See https://docs.bazel.build/versions/master/test-encyclopedia.html#initial-conditions.
	return os.Getenv("TEST_WORKSPACE") != ""
}

// RunfilesDir returns the path to the directory under which a Bazel-built binary or test can find
// its runfiles (e.g. files included in the "data" attribute of *_test targets) using relative
// paths.
func RunfilesDir() string {
	// See https://docs.bazel.build/versions/master/skylark/rules.html#runfiles-location and
	// https://docs.bazel.build/versions/master/test-encyclopedia.html#initial-conditions.
	return filepath.Join(os.Getenv("RUNFILES_DIR"), os.Getenv("TEST_WORKSPACE"))
}
