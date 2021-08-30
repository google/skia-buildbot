package vfs

/*
Package vfs provides interfaces for dealing with virtual file systems.

The interfaces here are taken io/fs, except they include a Context, which may be
used for things like HTTP requests.
*/

import (
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/skerr"
)

// FS represents a virtual filesystem.
type FS interface {
	// Open the given path. If the path is a directory, implementations should
	// return a ReadDirFile.
	Open(ctx context.Context, name string) (File, error)

	// Close causes any resources associated with the FS to be cleaned up.
	Close(ctx context.Context) error
}

// File represents a virtual file.
type File interface {
	// Close the File.
	Close(ctx context.Context) error
	// Read behaves like io.Reader. It should return an error if this is a
	// directory.
	Read(ctx context.Context, buf []byte) (int, error)
	// Stat returns FileInfo associated with the File.
	Stat(ctx context.Context) (fs.FileInfo, error)

	// ReadDir returns the contents of the File if it is a directory, and
	// returns an error otherwise. Shouold behave the same as os.File.Readdir.
	ReadDir(ctx context.Context, n int) ([]fs.FileInfo, error)
}

// ReuseContextFile is a File which reuses the same Context for all calls. This
// is useful for passing the File into library functions which do not use
// Contexts.
type ReuseContextFile struct {
	File
	ctx context.Context
}

// Close closes the ReuseContextFile.
func (f *ReuseContextFile) Close() error {
	return f.File.Close(f.ctx)
}

// Read reads from the ReuseContextFile.
func (f *ReuseContextFile) Read(buf []byte) (int, error) {
	return f.File.Read(f.ctx, buf)
}

// Stat returns the fs.FileInfo describing the ReuseContextFile.
func (f *ReuseContextFile) Stat() (fs.FileInfo, error) {
	return f.File.Stat(f.ctx)
}

// WithContext returns a ReuseContextFile which wraps the given File.
func WithContext(ctx context.Context, f File) *ReuseContextFile {
	return &ReuseContextFile{
		File: f,
		ctx:  ctx,
	}
}

// ReadFile is analogous to ioutil.ReadFile.
func ReadFile(ctx context.Context, fs FS, path string) (rv []byte, rvErr error) {
	f, err := fs.Open(ctx, path)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer func() {
		closeErr := f.Close(ctx)
		if rvErr == nil {
			rvErr = closeErr
		}
	}()
	wrapFile := WithContext(ctx, f)
	return ioutil.ReadAll(wrapFile)
}

// ReadDir is analogous to ioutil.ReadDir.
func ReadDir(ctx context.Context, fs FS, path string) (rv []fs.FileInfo, rvErr error) {
	f, err := fs.Open(ctx, path)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer func() {
		closeErr := f.Close(ctx)
		if rvErr == nil {
			rvErr = closeErr
		}
	}()
	return f.ReadDir(ctx, -1)
}

// Stat is analogous to os.Stat.
func Stat(ctx context.Context, fs FS, path string) (rv fs.FileInfo, rvErr error) {
	f, err := fs.Open(ctx, path)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer func() {
		closeErr := f.Close(ctx)
		if rvErr == nil {
			rvErr = closeErr
		}
	}()
	return f.Stat(ctx)
}

// Walk is analogous to filepath.Walk.
func Walk(ctx context.Context, fs FS, root string, walkFn filepath.WalkFunc) error {
	// This implementation is basically copied from filepath.Walk.
	info, err := Stat(ctx, fs, root)
	if err != nil {
		err = walkFn(root, info, err)
	} else {
		err = walk(ctx, fs, root, info, walkFn)
	}
	if err == filepath.SkipDir {
		return nil
	}
	return err
}

// walk is analogous to filepath.walk.
func walk(ctx context.Context, fs FS, fp string, info fs.FileInfo, walkFn filepath.WalkFunc) error {
	// This implementation is basically copied from filepath.walk.
	if !info.IsDir() {
		return walkFn(fp, info, nil)
	}

	infos, err := ReadDir(ctx, fs, fp)
	var names []string
	if err == nil {
		names = make([]string, 0, len(infos))
		for _, fi := range infos {
			names = append(names, fi.Name())
		}
	}
	err1 := walkFn(fp, info, err)
	// If err != nil, walk can't walk into this directory.
	// err1 != nil means walkFn want walk to skip this directory or stop walking.
	// Therefore, if one of err and err1 isn't nil, walk will return.
	if err != nil || err1 != nil {
		// The caller's behavior is controlled by the return value, which is decided
		// by walkFn. walkFn may ignore err and return nil.
		// If walkFn returns SkipDir, it will be handled by the caller.
		// So walk should return whatever walkFn returns.
		return err1
	}

	for _, name := range names {
		filename := path.Join(fp, name)
		fileInfo, err := Stat(ctx, fs, filename)
		if err != nil {
			if err := walkFn(filename, fileInfo, err); err != nil && err != filepath.SkipDir {
				return err
			}
		} else {
			err = walk(ctx, fs, filename, fileInfo, walkFn)
			if err != nil {
				if !fileInfo.IsDir() || err != filepath.SkipDir {
					return err
				}
			}
		}
	}
	return nil
}

// FileInfo implements fs.FileInfo by simply filling out the return values for
// all of the methods.
type FileInfo struct {
	Name    string
	Size    int64
	Mode    os.FileMode
	ModTime time.Time
	IsDir   bool
	Sys     interface{}
}

// Get returns an fs.FileInfo backed by this FileInfo.
func (fi FileInfo) Get() *FileInfoImpl {
	return &FileInfoImpl{fi}
}

// FileInfoImpl implements fs.FileInfo.
type FileInfoImpl struct {
	FileInfo
}

// Name implements fs.FileInfo.
func (fi *FileInfoImpl) Name() string {
	return fi.FileInfo.Name
}

// Size implements fs.FileInfo.
func (fi *FileInfoImpl) Size() int64 {
	return fi.FileInfo.Size
}

// Mode implements fs.FileInfo.
func (fi *FileInfoImpl) Mode() os.FileMode {
	return fi.FileInfo.Mode
}

// ModTime implements fs.FileInfo.
func (fi *FileInfoImpl) ModTime() time.Time {
	return fi.FileInfo.ModTime
}

// IsDir implements fs.FileInfo.
func (fi *FileInfoImpl) IsDir() bool {
	return fi.FileInfo.IsDir
}

// Sys implements fs.FileInfo.
func (fi *FileInfoImpl) Sys() interface{} {
	return fi.FileInfo.Sys
}

// Ensure that FileInfoImpl implements fs.FileInfo.
var _ fs.FileInfo = &FileInfoImpl{}
