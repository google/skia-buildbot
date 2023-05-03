package gitiles

import (
	"bytes"
	"context"
	"os"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vfs"
)

// New returns a vfs.FS using Gitiles at the given revision.
func New(ctx context.Context, repo gitiles.GitilesRepo, ref string) (*FS, error) {
	hash, err := repo.ResolveRef(ctx, ref)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &FS{
		repo:            repo,
		hash:            hash,
		cachedFileInfos: map[string]os.FileInfo{},
		cachedContents:  map[string][]byte{},
		changes:         map[string][]byte{},
	}, nil
}

// FS implements vfs.FS using Gitiles for a particular revision.
type FS struct {
	repo            gitiles.GitilesRepo
	hash            string
	cachedFileInfos map[string]os.FileInfo
	cachedContents  map[string][]byte
	changes         map[string][]byte
}

// Open implements vfs.FS.
func (fs *FS) Open(ctx context.Context, name string) (vfs.File, error) {
	cachedContents, ok := fs.changes[name]
	if !ok {
		cachedContents, ok = fs.cachedContents[name]
		if !ok {
			fi, contents, err := fs.repo.ReadObject(ctx, name, fs.hash)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			fs.cachedFileInfos[name] = fi
			fs.cachedContents[name] = contents
			cachedContents = contents
		}
	}
	cpy := make([]byte, len(cachedContents))
	copy(cpy, cachedContents)
	buf := bytes.NewBuffer(cpy)
	return &File{
		fs:             fs,
		repo:           fs.repo,
		hash:           fs.hash,
		name:           name,
		cachedFileInfo: fs.cachedFileInfos[name],
		cachedContents: cachedContents,
		buf:            buf,
	}, nil
}

// Create implements vfs.FS.
func (fs *FS) Create(ctx context.Context, name string) (vfs.File, error) {
	f, err := fs.Open(ctx, name)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	f.(*File).buf.Truncate(0)
	return f, nil
}

// Close implements vfs.FS.
func (fs *FS) Close(_ context.Context) error {
	return nil
}

// BaseCommit returns the commit referenced by this FS.
func (fs *FS) BaseCommit() string {
	return fs.hash
}

// Changes returns any changes to the FS.
func (fs *FS) Changes() map[string][]byte {
	// Create a new map, comparing the original contents to the updated ones.
	rv := make(map[string][]byte, len(fs.changes))
	for file, changedBytes := range fs.changes {
		if !bytes.Equal(changedBytes, fs.cachedContents[file]) {
			rv[file] = changedBytes
		}
	}
	return rv
}

// Ensure that gitiles.FS implements vfs.FS.
var _ vfs.FS = &FS{}

// File implements vfs.File using Gitiles for a particular revision.
type File struct {
	fs   *FS
	repo gitiles.GitilesRepo
	hash string
	name string

	// These are cached to avoid repeated requests.
	cachedFileInfo os.FileInfo
	cachedContents []byte
	buf            *bytes.Buffer
}

// Close implements vfs.File.
func (f *File) Close(_ context.Context) error {
	if f.cachedContents != nil && f.buf != nil {
		updatedContents := f.buf.Bytes()
		if !bytes.Equal(f.cachedContents, updatedContents) {
			f.fs.changes[f.name] = updatedContents
		}
	}
	return nil
}

// Read implements vfs.File.
func (f *File) Read(ctx context.Context, buf []byte) (int, error) {
	return f.buf.Read(buf)
}

// Stat implements vfs.File.
func (f *File) Stat(ctx context.Context) (os.FileInfo, error) {
	return f.cachedFileInfo, nil
}

// ReadDir implements vfs.File.
func (f *File) ReadDir(ctx context.Context, n int) ([]os.FileInfo, error) {
	rv, err := git.ParseDir(f.buf.Bytes())
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if n > 0 {
		rv = rv[:n]
	}
	return rv, nil
}

// Write implements vfs.File.
func (f *File) Write(ctx context.Context, b []byte) (int, error) {
	return f.buf.Write(b)
}

// Ensure that gitiles.File implements vfs.File.
var _ vfs.File = &File{}
