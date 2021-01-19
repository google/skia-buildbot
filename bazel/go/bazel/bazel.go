// Package bazel contains utilities for Go tests running under Bazel.
package bazel

import (
	"os"
	"path/filepath"

	"go.skia.org/infra/go/sktest"
)

// InBazel returns true if running under Bazel (e.g. "bazel test", "bazel run"), or false otherwise.
func InBazel() bool {
	// See https://docs.bazel.build/versions/master/test-encyclopedia.html#initial-conditions.
	return os.Getenv("TEST_WORKSPACE") != ""
}

// BazelTest skips a test, unless it's running under Bazel (e.g. "bazel test ...").
func BazelTest(t sktest.TestingT) {
	if !InBazel() {
		t.Skip("Not running Bazel tests from outside Bazel.")
	}
}

// RunfilesDir returns the path to the directory under which a Bazel-built binary or test can find
// its runfiles (e.g. files included in the "data" attribute of *_test targets) using relative
// paths.
func RunfilesDir() string {
	// See https://docs.bazel.build/versions/master/skylark/rules.html#runfiles-location and
	// https://docs.bazel.build/versions/master/test-encyclopedia.html#initial-conditions.
	return filepath.Join(os.Getenv("RUNFILES_DIR"), os.Getenv("TEST_WORKSPACE"))
}
