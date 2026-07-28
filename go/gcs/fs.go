package gcs

import (
	"context"
	"io/fs"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/fileutil/browser"
	"google.golang.org/api/iterator"
)

// FS is a GCS-backed fs.FS implementation.
type FS struct {
	ctx    context.Context
	client *storage.Client
	bucket string
}

// NewFS returns a new GCS-backed fs.FS implementation.
func NewFS(ctx context.Context, client *storage.Client, bucket string) *FS {
	return &FS{
		ctx:    ctx,
		client: client,
		bucket: bucket,
	}
}

// Open implements fs.FS.
func (g *FS) Open(name string) (fs.File, error) {
	r, err := g.client.Bucket(g.bucket).Object(name).NewReader(g.ctx)
	if err != nil {
		return nil, err
	}
	return &gcsFile{Reader: r, name: name}, nil
}

// gcsFile implements fs.File.
type gcsFile struct {
	*storage.Reader
	name string
}

// Stat implements fs.File.
func (f *gcsFile) Stat() (fs.FileInfo, error) {
	baseName := f.name
	lastSlash := strings.LastIndex(f.name, "/")
	if lastSlash != -1 {
		baseName = f.name[lastSlash+1:]
	}
	return &gcsFileInfo{
		name:    baseName,
		size:    f.Reader.Attrs.Size,
		updated: f.Reader.Attrs.LastModified,
		isDir:   false,
	}, nil
}

type gcsDirEntry struct {
	name    string
	isDir   bool
	size    int64
	updated time.Time
}

func (d *gcsDirEntry) Name() string { return d.name }
func (d *gcsDirEntry) IsDir() bool  { return d.isDir }
func (d *gcsDirEntry) Type() fs.FileMode {
	if d.isDir {
		return fs.ModeDir
	}
	return 0
}
func (d *gcsDirEntry) Info() (fs.FileInfo, error) {
	return &gcsFileInfo{name: d.name, size: d.size, updated: d.updated, isDir: d.isDir}, nil
}

type gcsFileInfo struct {
	name    string
	size    int64
	updated time.Time
	isDir   bool
}

func (fi *gcsFileInfo) Name() string { return fi.name }
func (fi *gcsFileInfo) Size() int64  { return fi.size }
func (fi *gcsFileInfo) Mode() fs.FileMode {
	if fi.isDir {
		return fs.ModeDir
	}
	return 0
}
func (fi *gcsFileInfo) ModTime() time.Time { return fi.updated }
func (fi *gcsFileInfo) IsDir() bool        { return fi.isDir }
func (fi *gcsFileInfo) Sys() interface{}   { return nil }

// ReadDir implements fs.ReadDirFS.
func (g *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	return g.ReadDirPrefix(name, 100000)
}

// ReadDirPrefix implements browser.PrefixReader.
// It queries Google Cloud Storage (GCS) for objects and directories starting with the
// specified prefix, using "/" as a delimiter to find immediate children, and limits
// the results as specified.
func (g *FS) ReadDirPrefix(prefix string, limit int) ([]fs.DirEntry, error) {
	dirPart := ""
	lastSlash := strings.LastIndex(prefix, "/")
	if lastSlash != -1 {
		dirPart = prefix[:lastSlash+1]
	}

	q := &storage.Query{
		Prefix:    prefix,
		Delimiter: "/",
	}
	it := g.client.Bucket(g.bucket).Objects(g.ctx, q)
	var entries []fs.DirEntry
	count := 0

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		if attrs.Prefix != "" {
			entries = append(entries, &gcsDirEntry{
				name:  strings.TrimPrefix(attrs.Prefix, dirPart),
				isDir: true,
			})
		} else if attrs.Name != "" {
			if attrs.Name == prefix && strings.HasSuffix(prefix, "/") {
				continue
			}
			entries = append(entries, &gcsDirEntry{
				name:    strings.TrimPrefix(attrs.Name, dirPart),
				isDir:   false,
				size:    attrs.Size,
				updated: attrs.Updated,
			})
		}

		count++
		if count >= limit {
			break
		}
	}
	return entries, nil
}

// Assert that FS implements fs.FS and browser.PrefixReader.
var _ fs.FS = &FS{}
var _ browser.PrefixReader = &FS{}
