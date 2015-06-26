// PDF Rasterizer
package pdf

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/util"
)

const pdfiumExecutable = "pdfium_test"

type Pdfium struct{}

func (Pdfium) String() string { return "Pdfium" }

func (Pdfium) Enabled() bool {
	return commandFound(pdfiumExecutable)
}

// Rasterize assumes that filepath.Dir(pdfInputPath) is writable
func (Pdfium) Rasterize(pdfInputPath, pngOutputPath string) error {
	if !(Pdfium{}).Enabled() {
		return fmt.Errorf("pdfium_test is missing")
	}

	// Check input
	if !fileutil.FileExists(pdfInputPath) {
		return fmt.Errorf("Path '%s' does not exist", pdfInputPath)
	}

	// Remove any files created by pdfiumExecutable
	defer func() {
		// Assume pdfInputPath has glob characters.
		matches, _ := filepath.Glob(fmt.Sprintf("%s.*.png", pdfInputPath))
		for _, match := range matches {
			util.Remove(match)
		}
	}()

	command := exec.Command(pdfiumExecutable, "--png", pdfInputPath)
	if err := command.Start(); err != nil {
		return err
	}
	go func() {
		time.Sleep(5 * time.Second)
		_ = command.Process.Kill()
	}()
	if err := command.Wait(); err != nil {
		return err
	}

	firstPagePath := fmt.Sprintf("%s.0.png", pdfInputPath)
	if !fileutil.FileExists(firstPagePath) {
		return fmt.Errorf("First rasterized page (%s) not found.", firstPagePath)
	}
	if err := os.Rename(firstPagePath, pngOutputPath); err != nil {
		return err
	}
	return nil
}
