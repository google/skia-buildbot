package frontend

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/testtools"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/testutils/unittest"
)

// TestNoop exercises the test harness, but does not make any assertions on the state of the file
// system after running Gazelle.
//
// TODO(lovisolo): Delete once we have at least one real test.
func TestNoop(t *testing.T) {
	unittest.BazelOnlyTest(t)

	inputFiles := []testtools.FileSpec{
		{Path: "WORKSPACE"}, // Gazelle requires that a WORKSPACE file exists, even if it's empty.
	}
	expectedOutputFiles := []testtools.FileSpec{}

	test(t, inputFiles, expectedOutputFiles)
}

// test runs Gazelle on a temporary directory with the given input files, and asserts that Gazelle
// generated the expected output files.
func test(t *testing.T, inputFiles, expectedOutputFiles []testtools.FileSpec) {
	gazelleAbsPath := filepath.Join(bazel.RunfilesDir(), "bazel/gazelle/frontend/gazelle_frontend_test_binary_/gazelle_frontend_test_binary")

	// Write the input files to a temporary directory.
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
