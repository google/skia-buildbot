package vfs

import (
	"context"
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
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, skerr.Wrap(err)
	}
	if st.IsDir() {
		return &localDir{localFile{file: f}}, nil
	}
	return &localFile{file: f}, nil
}

// localFile is an implementation of File which wraps an os.File.
type localFile struct {
	file *os.File
}

// Stat implements File.
func (f *localFile) Stat(_ context.Context) (os.FileInfo, error) {
	return f.file.Stat()
}

// Read implements File.
func (f *localFile) Read(_ context.Context, buf []byte) (int, error) {
	return f.file.Read(buf)
}

// Close implements File.
func (f *localFile) Close(_ context.Context) error {
	return f.file.Close()
}

// Ensure that localFile implements File.
var _ File = &localFile{}

// localDir is an implementation of ReadDirFile for a local directory.
type localDir struct {
	localFile
}

// ReadDir implements ReadDirFile.
func (d *localDir) ReadDir(ctx context.Context, n int) ([]os.FileInfo, error) {
	return d.file.Readdir(n)
}

// Ensure th at localDir implements ReadDirFile.
var _ ReadDirFile = &localDir{}
