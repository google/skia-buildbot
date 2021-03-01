// extract_puppeteer_screenshots extracts Puppeteer screenshots into a user-specified directory.
//
// Usage:
//
//     $ bazel run //:extract_puppeteer_screenshots -- --output_dir=<output directory>
//
// Under Bazel, Puppeteer tests save screenshots inside $TEST_UNDECLARED_OUTPUTS_DIR, which is set
// by the "bazel test" command. Screenshots, and any other undeclared outputs of a test, can be
// found under //bazel-testlogs bundled as a single .zip file per test target.
//
// For example, if we run a Puppeteer test with "bazel test //my_app:puppeteer_test", then any
// screenshots will be found inside //bazel-testlogs/my_app/puppeteer_test/test.outputs/outputs.zip.
//
// See https://docs.bazel.build/versions/master/test-encyclopedia.html#initial-conditions to learn
// more about undeclared test outputs.
package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	outputDir = flag.String("output_dir", "", "Directory inside which to extract screenshots.")

	outputDirAbsPath string
)

func main() {
	flag.Parse()
	if *outputDir == "" {
		failf("Flag --output_dir is required.\n")
	}

	// If running via "bazel run", change into the directory where Bazel was invoked. This is
	// necessary to correctly compute the absolute path of the output directory.
	if os.Getenv("BUILD_WORKING_DIRECTORY") != "" {
		if err := os.Chdir(os.Getenv("BUILD_WORKING_DIRECTORY")); err != nil {
			sklog.Fatal(err)
		}
	}

	// Validate and compute the absolute path of the output directory.
	var err error
	if outputDirAbsPath, err = filepath.Abs(*outputDir); err != nil {
		failf("Invalid path: \"%s\"\n", *outputDir)
	}
	if _, err := os.Stat(*outputDir); os.IsNotExist(err) {
		failf("Directory \"%s\" does not exist.\n", *outputDir)
	}

	// If running via "bazel run", change into the workspace root directory (i.e. where the WORKSPACE
	// file is located). If not, we assume that the current working directory is the workspace root.
	if os.Getenv("BUILD_WORKSPACE_DIRECTORY") != "" {
		if err := os.Chdir(os.Getenv("BUILD_WORKSPACE_DIRECTORY")); err != nil {
			sklog.Fatal(err)
		}
	}

	// Resolve the //bazel-testlogs symlink. Necessary because filepath.Walk() ignores symlinks.
	bazelTestlogsDir, err := filepath.EvalSymlinks("bazel-testlogs")
	if err != nil {
		sklog.Fatal(err)
	}

	// Find all outputs.zip files under //bazel-testlogs, which contain the undeclared outputs
	// produced by all tests.
	var allOutputsZipPaths []string
	if err := filepath.Walk(bazelTestlogsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return skerr.Wrap(err)
		}
		if strings.HasSuffix(path, "/test.outputs/outputs.zip") {
			allOutputsZipPaths = append(allOutputsZipPaths, path)
		}
		return nil
	}); err != nil {
		sklog.Fatal(err)
	}

	// Inspect each outputs.zip file for Puppeteer screenshots. Extract them into the output directory
	// if any are found.
	for _, path := range allOutputsZipPaths {
		if err := extractPuppeteerScreenshotsFromOutputsZip(path); err != nil {
			sklog.Fatal(err)
		}
	}
}

// failf prints a message to sterr and exits with a non-zero exit code.
func failf(msg string, args ...interface{}) {
	if _, err := fmt.Fprintf(os.Stderr, msg, args...); err != nil {
		sklog.Fatal(err)
	}
	os.Exit(1)
}

// extractPuppeteerScreenshotsFromOutputsZip inspects an outputs.zip file looking for screenshots
// taken by a Puppeteer test, and extracts the screenshots inside the output directory if any are
// found.
//
// This function makes the following assumptions:
//
//   - All screenshots produced by a Puppeteer tests will be found inside a
//     "puppeteer-test-screenshots" directory within the test's outputs.zip file.
//   - All screenshots are PNG files (*.png)
//   - Puppeteer tests are the only tests in our codebase that produce undeclared outputs following
//     the above conventions.
//
// An alternative approach is to find all Puppeteer tests via a Bazel query (e.g.
// "bazel query 'attr(generator_function, sk_element_puppeteer_test, //...)'"), but this can be
// slow. Inspecting all outputs.zip files inside the //bazel-testlogs directory is much faster.
func extractPuppeteerScreenshotsFromOutputsZip(zipFilePath string) error {
	// Open the ZIP archive.
	zipFile, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(zipFile)

	// Iterate over all files inside the ZIP archive.
	for _, file := range zipFile.File {
		// Skip if the file is not a Puppeteer screenshot.
		dir, screenshotFileName := filepath.Split(file.Name)
		if dir != "puppeteer-test-screenshots/" || !strings.HasSuffix(screenshotFileName, ".png") {
			continue
		}

		// Extract screenshot into the output directory.
		outputFileName := filepath.Join(outputDirAbsPath, screenshotFileName)
		if err := extractFileFromZipArchive(file, outputFileName); err != nil {
			return skerr.Wrap(err)
		}
		fmt.Printf("Extracted screenshot: %s\n", outputFileName)
	}

	return nil
}

// extractFileFromZipArchive extracts a file inside a ZIP archive, and saves it to the outputPath.
func extractFileFromZipArchive(zippedFile *zip.File, outputPath string) error {
	// Open the file inside the ZIP archive.
	zippedFileReader, err := zippedFile.Open()
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(zippedFileReader)

	// Save it to disk.
	if err := util.WithWriteFile(outputPath, func(w io.Writer) error {
		if _, err := io.Copy(w, zippedFileReader); err != nil {
			return skerr.Wrap(err)
		}
		return nil
	}); err != nil {
		return skerr.Wrap(err)
	}

	return nil
}
