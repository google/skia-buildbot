// Package file is a source of file names and contents that contain data
// for Perf to ingest.
package file

import (
	"context"
	"io"
	"time"
)

// File represents a single file.
type File struct {
	Name     string
	Contents io.ReadCloser
	Created  time.Time
}

// Source is a source of Files.
type Source interface {
	// Start begins the process of looking for new files and as they arrive they
	// are sent on the returned channel.
	//
	// Should only be called once per instance.
	Start(ctx context.Context) (<-chan File, error)
}
