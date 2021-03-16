package frontend

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/testtools"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

var gazelleBin = flag.String("gazelle_bin", "", "Path to the test-only Gazelle binary.")

func TestTSLibrary(t *testing.T) {
	unittest.BazelOnlyTest(t)

	inputFiles := []testtools.FileSpec{
		{Path: "WORKSPACE"},
		{Path: "hello/hello.ts", Content: "console.log('Hello, world!)"},
	}

	expectedOutputFiles := []testtools.FileSpec{
		{Path: "hello/BUILD.bazel", Content: `load("//infra-sk:index.bzl", "ts_library")

ts_library(
    name = "hello",
    srcs = ["hello.ts"],
    visibility = ["//visibility:public"],
)
`,
		},
	}

	test(t, inputFiles, expectedOutputFiles)
}

// test runs Gazelle on a temporary directory with the given input files, and asserts that Gazelle
// generated the expected output files.
func test(t *testing.T, inputFiles, expectedOutputFiles []testtools.FileSpec) {
	flag.Parse()
	gazelleAbsPath, err := filepath.Abs(*gazelleBin)
	require.NoError(t, err)

	// Create the input files.
	dir, cleanup := testtools.CreateFiles(t, inputFiles)
	defer cleanup()

	// Run Gazelle.
	cmd := exec.Command(gazelleAbsPath, "--frontend_unit_test")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	// Assert that Gazelle generated the expected files.
	testtools.CheckFiles(t, dir, expectedOutputFiles)
}
