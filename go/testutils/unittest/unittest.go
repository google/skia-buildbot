package unittest

import (
	"runtime"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/sktest"
)

// BazelOnlyTest is a function which should be called at the beginning of tests
// which should only run under Bazel (e.g. via "bazel test ...").
func BazelOnlyTest(t sktest.TestingT) {
	if !bazel.InBazelTest() {
		t.Skip("Not running Bazel tests from outside Bazel.")
	}
}

// LinuxOnlyTest is a function which should be called at the beginning of a test
// which should only run on Linux.
func LinuxOnlyTest(t sktest.TestingT) {
	if runtime.GOOS != "linux" {
		t.Skip("Not running Linux-only tests.")
	}
}
