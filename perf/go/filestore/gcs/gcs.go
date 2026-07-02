// Package gcs implements fs.FS using Google Cloud Storage.
package gcs

import (
	"context"
	"errors"
	"io/fs"
	"net/url"
	"os"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

var (
	// ErrNotImplemented is returned from Stat().
	ErrNotImplemented = errors.New("Not Implemented.")
)

// filesystem implements fs.FS on Google Cloud Storage.
type filesystem struct {
	client *storage.Client
}

// New returns an instance of *filesystem.
func New(ctx context.Context) (*filesystem, error) {
	ts, err := google.DefaultTokenSource(ctx, storage.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get TokenSource")
	}
	sklog.Info("About to init GCS.")
	client, err := storage.NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to authenicate to cloud storage")
	}

	return &filesystem{
		client: client,
	}, nil
}

func parseNameIntoBucketAndPath(name string) (string, string, error) {
	u, err := url.Parse(name)
	if err != nil {
		return "", "", skerr.Wrapf(err, "Failed to parse source file location.")
	}
	if u.Host == "" || u.Path == "" {
		return "", "", skerr.Fmt("Invalid source location: %q", name)
	}
	path := u.Path
	if len(path) > 1 {
		path = u.Path[1:]
	}
	return u.Host, path, nil
}

// file implements fs.File for a *storage.Reader.
type file struct {
	*storage.Reader
}

// Stat implements fs.File.
func (f *file) Stat() (os.FileInfo, error) {
	// Perf never uses Stat(), so don't bother implementing it.
	return nil, ErrNotImplemented
}

// Open implements http.FileSystem.
func (f *filesystem) Open(name string) (fs.File, error) {
	bucket, path, err := parseNameIntoBucketAndPath(name)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse source file location.")
	}

	reader, err := f.client.Bucket(bucket).Object(path).NewReader(context.Background())
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get reader for source file location")
	}
	return &file{
		Reader: reader,
	}, nil
}

type objectIterator interface {
	Next() (*storage.ObjectAttrs, error)
}

// FindFileByPrefix implements prefix searching for Google Cloud Storage.
func (f *filesystem) FindFileByPrefix(ctx context.Context, namePrefix string) (string, error) {
	bucket, pathPrefix, err := parseNameIntoBucketAndPath(namePrefix)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to parse source file location.")
	}

	it := f.client.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: pathPrefix})
	return extractLatestObject(it, bucket, namePrefix)
}

func extractLatestObject(it objectIterator, bucket, namePrefix string) (string, error) {
	var lastAttrs *storage.ObjectAttrs
	var count int
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "", skerr.Wrapf(err, "Failed to find object with prefix %q", namePrefix)
		}
		lastAttrs = attrs
		count++
	}

	if lastAttrs == nil {
		return "", fs.ErrNotExist
	}

	if count > 1 {
		sklog.Infof("Found %d files matching prefix %q. Using the latest one: %q", count, namePrefix, lastAttrs.Name)
	}

	return "gs://" + bucket + "/" + lastAttrs.Name, nil
}

// Assert that *filesystem implements http.FileSystem.
var _ fs.FS = (*filesystem)(nil)
