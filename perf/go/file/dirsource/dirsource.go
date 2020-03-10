// Package dirsource implements the file.Source interface for filesystem
// directories.
//
// It is only appropriate for using in tests and demo mode at this time.
package dirsource

import (
	"context"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/file"
)

// channelSize is the buffer size of the file.File channel.
const channelSize = 10

// DirSource implements the file.Source interface using a filesystem directory.
//
// Caveats: Currently only walks the directory and emits a file.File for each
// file found. It does not watch the directory for changes. It also uses
// modified time in place of created time. This implementation is only useful
// for tests and demo mode.
type DirSource struct {
	dir     string
	started bool
}

// New return a new instance of DirSource.
func New(dir string) (*DirSource, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &DirSource{
		dir: absDir,
	}, nil
}

// Start implements the file.Source interface.
func (d *DirSource) Start(_ context.Context) (<-chan file.File, error) {
	if d.started {
		return nil, skerr.Fmt("Start can only be called once.")
	}
	d.started = true

	ret := make(chan file.File, channelSize)
	go func() {
		defer close(ret)
		err := filepath.Walk(d.dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return skerr.Wrap(err)
			}
			f, err := os.Open(path)
			if err != nil {
				return skerr.Wrap(err)
			}
			if info.IsDir() {
				return nil
			}
			ret <- file.File{
				Name:     path,
				Contents: f,
				Created:  info.ModTime(), // Obviously not created time.
			}
			return nil
		})
		if err != nil {
			sklog.Errorf("Error walking the path %q: %s", d.dir, err)
		}
	}()

	return ret, nil
}
