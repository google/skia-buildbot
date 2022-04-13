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
func New(ctx context.Context, local bool) (*filesystem, error) {
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

// Assert that *filesystem implements http.FileSystem.
var _ fs.FS = (*filesystem)(nil)
