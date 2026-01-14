package extract

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// Extract scans the "_bazel_testlogs" directory inside the workspaceDir, and extracts any
// screenshots taken by Puppeteer tests into targetDir.
func Extract(workspaceDir, targetDir string) error {
	// Resolve the //_bazel_testlogs symlink. Necessary because filepath.Walk() ignores symlinks.
	bazelTestlogsDir, err := filepath.EvalSymlinks(filepath.Join(workspaceDir, "_bazel_testlogs"))
	if err != nil {
		return skerr.Wrap(err)
	}

	// Bazel <8: find all outputs.zip files under //_bazel_testlogs, which contain the undeclared outputs
	// produced by all tests.
	var allOutputsZipPaths []string
	// Bazel 8 stopped zipping undeclared test outputs and instead just deposits them directly in the
	// outputs folder.
	var allUnzippedScreenshots []string
	if err := filepath.Walk(bazelTestlogsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return skerr.Wrap(err)
		}
		if strings.HasSuffix(path, "/test.outputs/outputs.zip") {
			allOutputsZipPaths = append(allOutputsZipPaths, path)
		}
		if strings.Contains(path, "/test.outputs/puppeteer-test-screenshots/") && strings.HasSuffix(path, ".png") {
			allUnzippedScreenshots = append(allUnzippedScreenshots, path)
		}
		return nil
	}); err != nil {
		return skerr.Wrap(err)
	}

	// Inspect each outputs.zip file for Puppeteer screenshots. Extract them into the output directory
	// if any are found.
	for _, zipFilePath := range allOutputsZipPaths {
		if err := extractPuppeteerScreenshotsFromOutputsZip(zipFilePath, targetDir); err != nil {
			return skerr.Wrap(err)
		}
	}

	// Copy all screenshots into the output directory.
	for _, screenshotPath := range allUnzippedScreenshots {
		fileName := filepath.Base(screenshotPath)
		destPath := filepath.Join(targetDir, fileName)
		if err := util.CopyFile(screenshotPath, destPath); err != nil {
			return skerr.Wrap(err)
		}
		sklog.Infof("Copied screenshot: %s\n", destPath)
	}

	return nil
}

// extractPuppeteerScreenshotsFromOutputsZip inspects an outputs.zip file looking for screenshots
// taken by a Puppeteer test, and extracts the screenshots inside the output directory if any are
// found.
//
// This function makes the following assumptions:
//
//   - All screenshots produced by a Puppeteer test will be found inside a
//     "puppeteer-test-screenshots" directory within the test's outputs.zip file.
//   - All screenshots are PNG files (*.png)
//   - Puppeteer tests are the only tests in our codebase that produce undeclared outputs following
//     the above conventions.
//
// An alternative approach is to find all Puppeteer tests via a Bazel query (e.g.
// "bazel query 'attr(generator_function, sk_element_puppeteer_test, //...)'"), but this can be
// slow. Inspecting all outputs.zip files inside the //_bazel_testlogs directory is much faster.
func extractPuppeteerScreenshotsFromOutputsZip(zipFilePath, targetDir string) error {
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
		outputPath := filepath.Join(targetDir, screenshotFileName)
		if err := extractFileFromZipArchive(file, outputPath); err != nil {
			return skerr.Wrap(err)
		}
		sklog.Infof("Extracted screenshot: %s\n", outputPath)
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
