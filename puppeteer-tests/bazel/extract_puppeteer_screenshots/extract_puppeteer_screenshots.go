// extract_puppeteer_screenshots extracts Puppeteer screenshots into a user-specified directory.
//
// Usage:
//
//	$ bazel run //:extract_puppeteer_screenshots -- --output_dir=<output directory>
//
// Under Bazel, Puppeteer tests save screenshots inside $TEST_UNDECLARED_OUTPUTS_DIR, which is set
// by the "bazel test" command. Screenshots, and any other undeclared outputs of a test, can be
// found under //_bazel_testlogs bundled as a single .zip file per test target.
//
// For example, if we run a Puppeteer test with "bazel test //my_app:puppeteer_test", then any
// screenshots will be found inside //_bazel_testlogs/my_app/puppeteer_test/test.outputs/outputs.zip.
//
// See https://docs.bazel.build/versions/master/test-encyclopedia.html#initial-conditions to learn
// more about undeclared test outputs.
package main

import (
	"flag"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/puppeteer-tests/bazel/extract_puppeteer_screenshots/extract"
)

func main() {
	outputDir := flag.String("output_dir", "", "Directory inside which to extract screenshots.")
	flag.Parse()
	if *outputDir == "" {
		sklog.Fatal("Flag --output_dir is required.\n")
	}

	// Get the path to the repository root (and ensure we are running under Bazel).
	workspaceDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if workspaceDir == "" {
		sklog.Fatal("The BUILD_WORKSPACE_DIRECTORY environment variable is not set. Are you running this program via Bazel?")
	}

	// Validate and compute the absolute path of the output directory. We change into workspaceDir
	// to ensure that the absolute path of the output directory is computed relative to workspaceDir.
	if err := os.Chdir(workspaceDir); err != nil {
		sklog.Fatalf("Could not change into the workspace directory: %s", err)
	}
	outputDirAbsPath, err := filepath.Abs(*outputDir)
	if err != nil {
		sklog.Fatalf("Invalid path: \"%s\"\n", *outputDir)
	}
	if _, err := os.Stat(outputDirAbsPath); os.IsNotExist(err) {
		sklog.Fatalf("Directory \"%s\" does not exist.\n", outputDirAbsPath)
	}

	if err := extract.Extract(workspaceDir, outputDirAbsPath); err != nil {
		sklog.Fatalf("Could not extract screenshots: %s", err)
	}
}
