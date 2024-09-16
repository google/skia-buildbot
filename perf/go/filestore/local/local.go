// Package local implements fs.FS for local files
package local

import (
	"io/fs"
	"os"
	"path/filepath"
)

// filesystem implements fs.FS on local filesystem.
type filesystem struct {
	// The root directory for the local fs.
	rootDir string
	// The dirfs instance to traverse the fs relative to the root directory.
	fs fs.FS
}

// New returns an instance of *filesystem.
func New(root string) (*filesystem, error) {
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &filesystem{
		rootDir: rootPath,
		fs:      os.DirFS(rootPath),
	}, nil
}

// Open implements fs.FS.
func (f *filesystem) Open(name string) (fs.File, error) {
	relativePath, err := filepath.Rel(f.rootDir, name)
	if err != nil {
		return nil, err
	}

	return f.fs.Open(relativePath)
}

// Assert that *filesystem implements fs.FS.
var _ fs.FS = (*filesystem)(nil)
