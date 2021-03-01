// Utility program to extract Puppeteer screenshots into a user-specified directory.
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
)

var (
	outputDir = flag.String("output_dir", "", "Directory inside which to extract screenshots.")
)

func main() {
	flag.Parse()
	if *outputDir == "" {
		sklog.Fatal("Flag --output_dir is required.")
	}

	// If running under Bazel, "cd" into the workspace root directory (i.e. where the WORKSPACE file
	// is located). If not, we assume that the current working directory is the workspace root.
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
		if err := extractPuppeteerScreenshotsIfAny(path); err != nil {
			sklog.Fatal(err)
		}
	}
}

// extractPuppeteerScreenshotsIfAny inspects an outputs.zip file looking for screenshots taken by a
// Puppeteer test, and extracts the screenshots inside the output directory if any are found.
//
// This function makes the following assumptions:
//
//   - All screenshots produced by a Puppeteer tests will be found inside a
//     "puppeteer-test-screenshots" directory within the test's outputs.zip file.
//   - All screenshots are PNG files (*.png)
//   - Puppeteer tests are the only tests that produce undeclared outputs following the above
//     conventions.
//
// An alternative approach is to find all Puppeteer tests via a Bazel query (e.g.
// "bazel query 'attr(generator_function, sk_element_puppeteer_test, //...)'"), but this can be
// slow. Inspecting all outputs.zip files inside the //bazel-testlogs directory is much faster.
func extractPuppeteerScreenshotsIfAny(zipFilePath string) error {
	// Open the ZIP archive.
	zipFile, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer func() {
		if err := zipFile.Close(); err != nil {
			sklog.Error(err)
		}
	}()

	// Iterate over all files inside the ZIP archive.
	for _, file := range zipFile.File {
		// Skip if the file is not a Puppeteer screenshot.
		dir, screenshotFileName := filepath.Split(file.Name)
		if dir != "puppeteer-test-screenshots/" || !strings.HasSuffix(screenshotFileName, ".png") {
			continue
		}

		// Extract screenshot into the output directory.
		outputFileName := filepath.Join(*outputDir, screenshotFileName)
		if err := extractScreenshot(file, outputFileName); err != nil {
			return skerr.Wrap(err)
		}
		fmt.Printf("Extracted screenshot: %s\n", outputFileName)
	}

	return nil
}

// extractScreenshot extracts a file inside a ZIP archive to the outputPath.
func extractScreenshot(file *zip.File, outputPath string) error {
	// Open the file inside the ZIP archive.
	fileReader, err := file.Open()
	if err != nil {
		return skerr.Wrap(err)
	}
	defer func() {
		if err := fileReader.Close(); err != nil {
			sklog.Error(err)
		}
	}()

	// Create the output file.
	outputFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
	if err != nil {
		return skerr.Wrap(err)
	}
	defer func() {
		if err := outputFile.Close(); err != nil {
			sklog.Error(err)
		}
	}()

	// Extract the screenshot.
	if _, err := io.Copy(outputFile, fileReader); err != nil {
		return skerr.Wrap(err)
	}

	return nil
}
