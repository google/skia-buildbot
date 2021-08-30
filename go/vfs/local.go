package vfs

import (
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/skerr"
)

// Local returns a FS which uses the local filesystem with the given root.
// Absolute paths may only be passed to Open() if they are subdirectories of the
// given root. Relative paths must not contain ".."
func Local(root string) FS {
	return &localFS{root: root}
}

// localFS is an implementation of FS which uses the local filesystem.
type localFS struct {
	root string
}

// Open implements FS.
func (fs *localFS) Open(_ context.Context, name string) (File, error) {
	if !filepath.IsAbs(name) {
		name = filepath.Join(fs.root, name)
	}
	name, err := filepath.Abs(name)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if !strings.HasPrefix(name, fs.root) {
		return nil, skerr.Fmt("path %s is not rooted within %s", name, fs.root)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &localFile{file: f}, nil
}

// Close implements FS.
func (fs *localFS) Close(_ context.Context) error {
	return nil
}

// localFile is an implementation of File which wraps an os.File.
type localFile struct {
	file *os.File
}

// Stat implements File.
func (f *localFile) Stat(_ context.Context) (fs.FileInfo, error) {
	return f.file.Stat()
}

// Read implements File.
func (f *localFile) Read(_ context.Context, buf []byte) (int, error) {
	return f.file.Read(buf)
}

// ReadDir implements File.
func (f *localFile) ReadDir(_ context.Context, n int) ([]fs.FileInfo, error) {
	return f.file.Readdir(n)
}

// Close implements File.
func (f *localFile) Close(_ context.Context) error {
	return f.file.Close()
}

// Ensure that localFile implements File.
var _ File = &localFile{}

// TempDirFS is a FS implementation which is rooted in a temporary directory.
// Calling Close causes the directory to be deleted.
type TempDirFS struct {
	FS
	dir string
}

// Close deletes the temporary directory.
func (fs *TempDirFS) Close(ctx context.Context) error {
	if err := fs.FS.Close(ctx); err != nil {
		return skerr.Wrap(err)
	}
	return skerr.Wrap(os.RemoveAll(fs.dir))
}

// Dir returns the temporary directory.
func (fs *TempDirFS) Dir() string {
	return fs.dir
}

// TempDir returns a FS which is rooted in a temporary directory. Calling Close
// causes the directory to be deleted.
func TempDir(_ context.Context, dir, prefix string) (*TempDirFS, error) {
	tmp, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &TempDirFS{
		FS:  Local(tmp),
		dir: tmp,
	}, nil
}
