// Package bazel contains utilities for Go tests running under Bazel.
package bazel

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/skerr"
)

// InBazel returns true if running under Bazel with either "bazel test" or "bazel run", or false
// otherwise.
func InBazel() bool {
	return InBazelRun() || InBazelTest()
}

// InBazelRun returns true if running under Bazel with "bazel run", or false otherwise.
func InBazelRun() bool {
	// See https://bazel.build/docs/user-manual#running-executables.
	//
	// This environment variable is not set for "bazel test" commands. Verified empirically against
	// Bazel 6.0.0.
	return os.Getenv("BUILD_WORKSPACE_DIRECTORY") != ""
}

// InBazelTest returns true if running under Bazel with "bazel test", or false otherwise.
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
//
// It panics if the program is not running under Bazel.
func RunfilesDir() string {
	if InBazelRun() {
		// PWD is the workspace subdirectory within the runfiles directory. The
		// runfiles directory itself is one level up.
		rv, err := filepath.Abs(filepath.Join(os.Getenv("PWD"), ".."))
		if err != nil {
			panic(err)
		}
		return rv
	}

	if InBazelTest() {
		// See https://bazel.build/extending/rules#runfiles_location
		return os.Getenv("RUNFILES_DIR")
	}

	panic("This program is not running under Bazel. Suggestion: Only call this function if bazel.InBazel() returns true.")
}

// FindExecutable attempts to find the executable within the runfiles directory.
// If not running in bazel, it falls back to exec.LookPath. Otherwise it just
// concatenates the runfiles directory to the provided sub-path and returns the
// resulting absolute path and an error if the path does not exist.
func FindExecutable(name, runfilePath string) (string, error) {
	if !InBazel() {
		return exec.LookPath(name)
	}
	if runfilePath == "" {
		return "", skerr.Fmt("runfilePath is empty; is there something wrong with the Bazel setup?")
	}
	rv := filepath.Join(RunfilesDir(), runfilePath)
	if _, err := os.Stat(rv); os.IsNotExist(err) {
		dir := rv
		for os.IsNotExist(err) {
			dir = filepath.Dir(dir)
			_, err = os.Stat(dir)
		}
		dirContents, err := os.ReadDir(dir)
		if err != nil {
			return "", skerr.Fmt("runfile path %s does not exist; is there something wrong with the Bazel setup? Directory %s exists but failed to read it.", rv, dir)
		}
		entries := make([]string, 0, len(dirContents))
		for _, e := range dirContents {
			entries = append(entries, e.Name())
		}
		return "", skerr.Fmt("runfile path %s does not exist; is there something wrong with the Bazel setup? Directory %s does exist with contents: %s", rv, dir, strings.Join(entries, ", "))
	}
	return rv, nil
}

// TestWorkspacePath returns the path to the directory under which files in this
// repository can be found inside of a "bazel test".
func TestWorkspaceDir() string {
	// TODO(borenet): How does this work with InBazelRun()?
	// See https://bazel.build/reference/test-encyclopedia#initial-conditions.
	return filepath.Join(RunfilesDir(), os.Getenv("TEST_WORKSPACE"))
}
