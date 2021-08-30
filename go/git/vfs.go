package git

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vfs"
)

// VFS returns a vfs.FS using Git for the given revision.
func (g GitDir) VFS(ctx context.Context, ref string) (*FS, error) {
	hash, err := g.RevParse(ctx, "--verify", ref+"^{commit}")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &FS{
		g:    g,
		hash: hash,
	}, nil
}

// FS implements vfs.FS using Git for a particular revision.
type FS struct {
	g    GitDir
	hash string
}

// Open implements vfs.FS.
func (fs *FS) Open(_ context.Context, name string) (vfs.File, error) {
	repoRoot, err := filepath.Abs(string(fs.g))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	absPath, err := filepath.Abs(filepath.Join(string(fs.g), name))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &File{
		g:      fs.g,
		isRoot: repoRoot == absPath,
		hash:   fs.hash,
		path:   name,
	}, nil
}

// Close implements vfs.FS.
func (fs *FS) Close(_ context.Context) error {
	return nil
}

// Ensure that FS implements vfs.FS.
var _ vfs.FS = &FS{}

// File implements vfs.File using Git for a particular revision.
type File struct {
	g      GitDir
	isRoot bool
	hash   string
	path   string

	// These are cached to avoid repeated calls to Git.
	cachedFileInfo fs.FileInfo
	cachedContents []byte

	// reader is used for repeated calls to Read().
	reader io.Reader
}

// Close implements vfs.File.
func (f *File) Close(_ context.Context) error {
	return nil
}

// get retrieves the contents of this File if they are not already cached.
func (f *File) get(ctx context.Context) ([]byte, error) {
	if f.cachedContents == nil {
		path := f.path
		if f.isRoot && !strings.HasSuffix(path, "/") {
			path = path + "/"
		}
		contents, err := f.g.CatFile(ctx, f.hash, path)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		f.cachedContents = contents
	}
	return f.cachedContents, nil
}

// Read implements vfs.File.
func (f *File) Read(ctx context.Context, buf []byte) (int, error) {
	if f.reader == nil {
		contents, err := f.get(ctx)
		if err != nil {
			return 0, skerr.Wrap(err)
		}
		f.reader = bytes.NewReader(contents)
	}
	n, err := f.reader.Read(buf)
	if err == io.EOF {
		f.reader = nil
		return n, err
	}
	return n, skerr.Wrap(err)
}

// Stat implements vfs.File.
func (f *File) Stat(ctx context.Context) (fs.FileInfo, error) {
	// Special case for the repo root.
	if f.isRoot {
		return vfs.FileInfo{
			Name: f.path,
			Size: 0,
			Mode: os.ModePerm | os.ModeDir,
			// Git doesn't track modification times.
			ModTime: time.Time{},
			IsDir:   true,
			Sys:     nil,
		}.Get(), nil
	}

	// Find the file in its parent dir.
	dir, file := filepath.Split(f.path)
	infos, err := f.g.ReadDir(ctx, f.hash, dir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	for _, fi := range infos {
		if fi.Name() == file {
			return fi, nil
		}
	}
	return nil, skerr.Fmt("Unable to find %q in %q", file, dir)
}

// ReadDir implements vfs.File.
func (f *File) ReadDir(ctx context.Context, n int) ([]fs.FileInfo, error) {
	contents, err := f.get(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv, err := ParseDir(contents)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if n > 0 {
		rv = rv[:n]
	}
	return rv, nil
}

// Ensure that File implements vfs.File.
var _ vfs.File = &File{}
