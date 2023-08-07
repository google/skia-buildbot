// Package ephemeral_storage has utilities for logging ephemeral (/tmp) disk usage.
package ephemeral_storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	typeTag = "ephemeral-usage"
)

type file struct {
	Name string
	Size int64
}

// report will be serialed to stdout so it gets picked up as a structured log by
// StackDriver.
type report struct {
	// Type is a const to make it easier to filter these structured log entries.
	Type       string
	TempDir    string // Usually "/tmp", or os.TempDir() if "/tmp" does not exist (e.g. on Windows).
	TotalBytes int64
	TotalFiles int64
	Files      []file
}

// UsageViaStructuredLogging emits JSON to stdout describing the usage of /tmp.
//
// The JSON emitted on a single line will be picked up by StackDriver as a structured log.
func UsageViaStructuredLogging(ctx context.Context) error {
	// Note we use "/tmp" and not $TMPDIR, because if $TMPDIR is set then it's
	// probably not using ephemeral storage.
	//
	// Other potential directories to walk are listed here:
	//
	// https://cloud.google.com/container-optimized-os/docs/concepts/disks-and-filesystem#working_with_the_file_system
	//
	// If /tmp does not exist, we fall back to os.TempDir(). This has the benefit of being compatible
	// with Windows.
	tempDir := "/tmp"
	if fileInfo, err := os.Stat(tempDir); err != nil || !fileInfo.IsDir() {
		tempDir = os.TempDir()
	}

	report := report{
		Type:    typeTag,
		TempDir: tempDir,
		Files:   []file{}, // Serialize to at least [] in JSON.
	}

	var totalSize int64
	var totalFiles int64

	if err := ctx.Err(); err != nil {
		return skerr.Wrap(err)
	}

	err := filepath.Walk(tempDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return skerr.Wrap(err)
		}
		if info.IsDir() {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return skerr.Wrap(err)
		}
		report.Files = append(report.Files, file{
			Name: path,
			Size: info.Size(),
		})
		totalSize += info.Size()
		totalFiles++
		return nil
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	report.TotalBytes = totalSize
	report.TotalFiles = totalFiles

	b, err := json.Marshal(report)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Print as a single line to stdout so that it gets picked up by StackDriver
	// as a structured log.
	fmt.Println(string(b))

	return nil
}

// Start does a period call to UsageViaStructuredLogging(). It does not return,
// so it should be run as a Go routine.
//
// If the context is cancelled then Start will return.
func Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case <-ticker.C:
			if err := UsageViaStructuredLogging(ctx); err != nil {
				sklog.Errorf("UsageViaStructuredLogging failed with %s", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
