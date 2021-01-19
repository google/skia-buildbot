// Package bazel contains utilities for Go tests running under Bazel.
package bazel

import (
	"os"

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
