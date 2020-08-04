package gitiles

import (
	"bytes"
	"context"
	"io"
	"os"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vfs"
)

// VFS returns a vfs.FS using Gitiles at the given revision.
func (r *Repo) VFS(ctx context.Context, ref string) (*FS, error) {
	hash, err := r.ResolveRef(ctx, ref)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &FS{
		repo: r,
		hash: hash,
	}, nil
}

// FS implements vfs.FS using Gitiles for a particular revision.
type FS struct {
	repo *Repo
	hash string
}

// Open implements vfs.FS.
func (fs *FS) Open(_ context.Context, name string) (vfs.File, error) {
	return &File{
		repo: fs.repo,
		hash: fs.hash,
		name: name,
	}, nil
}

// Close implements vfs.FS.
func (fs *FS) Close(_ context.Context) error {
	return nil
}

// Ensure that gitilesFS implements vfs.FS.
var _ vfs.FS = &FS{}

// File implements vfs.File using Gitiles for a particular revision.
type File struct {
	repo *Repo
	hash string
	name string

	// These are cached to avoid repeated requests.
	cachedFileInfo os.FileInfo
	cachedContents []byte

	// reader is used for repeated calls to Read().
	reader io.Reader
}

// get retrieves the contents and FileInfo for this File if they are not already
// cached.
func (f *File) get(ctx context.Context) (os.FileInfo, []byte, error) {
	if f.cachedFileInfo == nil || f.cachedContents == nil {
		fi, contents, err := f.repo.ReadObject(ctx, f.name, f.hash)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		f.cachedFileInfo = fi
		f.cachedContents = contents
	}
	return f.cachedFileInfo, f.cachedContents, nil
}

// Close implements vfs.File.
func (f *File) Close(_ context.Context) error {
	return nil
}

// Read implements vfs.File.
func (f *File) Read(ctx context.Context, buf []byte) (int, error) {
	if f.reader == nil {
		_, contents, err := f.get(ctx)
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
func (f *File) Stat(ctx context.Context) (os.FileInfo, error) {
	rv, _, err := f.get(ctx)
	return rv, skerr.Wrap(err)
}

// ReadDir implements vfs.File.
func (f *File) ReadDir(ctx context.Context, n int) ([]os.FileInfo, error) {
	_, contents, err := f.get(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv, err := git.ParseDir(contents)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if n > 0 {
		rv = rv[:n]
	}
	return rv, nil
}

// Ensure that gitilesFile implements vfs.File.
var _ vfs.File = &File{}
