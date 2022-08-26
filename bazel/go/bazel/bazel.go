// Package bazel contains utilities for Go tests running under Bazel.
package bazel

import (
	"os"
	"path/filepath"
)

// InBazelTest returns true if running under Bazel (e.g. "bazel test"), or false otherwise.
func InBazelTest() bool {
	// See https://docs.bazel.build/versions/master/test-encyclopedia.html#initial-conditions.
	return os.Getenv("TEST_WORKSPACE") != ""
}

// InBazelTestOnRBE returns true if running under Bazel on RBE (e.g. "bazel test --config=remote"),
// or false otherwise.
func InBazelTestOnRBE() bool {
	// The BAZEL_REMOTE environment variable is set in //.bazelrc when running with --config=remote.
	return InBazelTest() && os.Getenv("BAZEL_REMOTE") == "true"
}

// RunfilesDir returns the path to the directory under which a Bazel-built binary or test can find
// its runfiles (e.g. files included in the "data" attribute of *_test targets) using relative
// paths.
func RunfilesDir() string {
	// See https://bazel.build/extending/rules#runfiles_location and
	// https://bazel.build/reference/test-encyclopedia#initial-conditions.
	return filepath.Join(os.Getenv("RUNFILES_DIR"), os.Getenv("TEST_WORKSPACE"))
}
