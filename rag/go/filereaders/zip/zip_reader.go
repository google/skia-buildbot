package zip

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// ExtractZipData extracts the zip file content to the destination directory.
func ExtractZipData(content []byte, dest string) error {
	zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		sklog.Error(err)
		return err
	}
	extractFile := func(f *zip.File) error {
		// Construct the full path where the file will be extracted
		extractedPath := filepath.Join(dest, f.Name)

		// Handle Directory Entries
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(extractedPath, f.Mode().Perm()|0100); err != nil {
				return skerr.Fmt("failed to create directory %s: %w", extractedPath, err)
			}
			return nil
		}

		// Open the file in the archive
		rc, err := f.Open()
		if err != nil {
			return skerr.Fmt("failed to open file in zip: %w", err)
		}
		defer rc.Close()

		// Ensure the parent directory exists before creating the file
		dirPath := filepath.Dir(extractedPath)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return skerr.Fmt("failed to create parent dir: %w", err)
		}

		// Create the output file
		outFile, err := os.OpenFile(extractedPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return skerr.Fmt("failed to create output file %s: %w", extractedPath, err)
		}
		defer outFile.Close()

		// Copy file contents
		if _, err = io.Copy(outFile, rc); err != nil {
			return skerr.Fmt("failed to copy file contents: %w", err)
		}
		return nil
	}
	for _, f := range zipReader.File {
		err := extractFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}
