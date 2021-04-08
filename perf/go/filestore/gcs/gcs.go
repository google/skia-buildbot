// Package gcs implements http.FileSystem using Google Cloud Storage.
package gcs

import (
	"context"
	"net/http"
	"net/url"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

// filesystem implements http.FileSystem on Google Cloud Storage.
type filesystem struct {
	client *storage.Client
}

func New(ctx context.Context, local bool) (*filesystem, error) {
	ts, err := auth.NewDefaultTokenSource(local, storage.ScopeReadOnly)
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

// Open implements http.FileSystem.
func (f *filesystem) Open(name string) (http.File, error) {
	u, err := url.Parse(name)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse source file location.")
	}
	if u.Host == "" || u.Path == "" {
		return nil, skerr.Fmt("Invalid source location: %q", name)
	}
	sklog.Infof("Host: %q Path: %q", u.Host, u.Path)

	reader, err := f.client.Bucket(u.Host).Object(u.Path[1:]).NewReader(context.Background())
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get reader for source file location")
	}
	return reader, nil
}

// Assert that *filesystem implements http.FileSystem.
var _ http.FileSystem = (*filesystem)(nil)
