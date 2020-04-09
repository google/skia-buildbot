package gcs

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/untar"
	"go.skia.org/infra/go/util"
)

// This file implements utility functions for accessing data in Google Storage.

// RequestForStorageURL returns an http.Request for a given Cloud Storage URL.
// This is workaround of a known issue: embedded slashes in URLs require use of
// URL.Opaque property
func RequestForStorageURL(url string) (*http.Request, error) {
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("HTTP new request error: %s", err)
	}
	schemePos := strings.Index(url, ":")
	queryPos := strings.Index(url, "?")
	if queryPos == -1 {
		queryPos = len(url)
	}
	r.URL.Opaque = url[schemePos+1 : queryPos]
	return r, nil
}

// FileContentsFromGCS returns the contents of a file in the given bucket or an error.
func FileContentsFromGCS(s *storage.Client, bucketName, fileName string) ([]byte, error) {
	response, err := s.Bucket(bucketName).Object(fileName).NewReader(context.Background())
	if err != nil {
		return nil, err
	}
	defer util.Close(response)
	return ioutil.ReadAll(response)
}

// AllFilesInDir synchronously iterates through all the files in a given Google Storage folder.
// The callback function is called on each item in the order it is in the bucket.
// It returns an error if the bucket or folder cannot be accessed.
func AllFilesInDir(s *storage.Client, bucket, folder string, callback func(item *storage.ObjectAttrs)) error {
	total := 0
	q := &storage.Query{Prefix: folder, Versions: false}
	it := s.Bucket(bucket).Objects(context.Background(), q)
	for obj, err := it.Next(); err != iterator.Done; obj, err = it.Next() {
		if err != nil {
			return fmt.Errorf("Problem reading from Google Storage: %v", err)
		}
		total++
		callback(obj)
	}
	return nil
}

// SplitGSPath takes a GCS path and splits it into a <bucket,path> pair.
// It assumes the format: {bucket_name}/{path_within_bucket}.
func SplitGSPath(path string) (string, string) {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) > 1 {
		return parts[0], parts[1]
	}
	return path, ""
}

// WithWriteFile writes to a GCS object using the given function, handling all errors. No
// compression is done on the data. See GCSClient.FileWriter for details on the parameters.
func WithWriteFile(client GCSClient, ctx context.Context, path string, opts FileWriteOptions, fn func(io.Writer) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	writer := client.FileWriter(ctx, path, opts)
	if err := fn(writer); err != nil {
		return err
	}
	return writer.Close()
}

// WithWriteFileGzip writes to a GCS object using the given function, compressing the data with gzip
// and handling all errors. See GCSClient.FileWriter for details on the parameters.
func WithWriteFileGzip(client GCSClient, ctx context.Context, path string, fn func(io.Writer) error) error {
	opts := FileWriteOptions{
		ContentEncoding: "gzip",
	}
	return WithWriteFile(client, ctx, path, opts, func(w io.Writer) error {
		return util.WithGzipWriter(w, fn)
	})
}

// DownloadAndExtractTarGz downloads the gzip-compressed tarball and extracts
// the files to the given destination directory.
func DownloadAndExtractTarGz(ctx context.Context, s *storage.Client, gcsBucket, gcsPath, dest string) error {
	// Set up the readers to stream the tarball from GCS.
	r, err := s.Bucket(gcsBucket).Object(gcsPath).NewReader(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(r)
	return skerr.Wrap(untar.Untar(r, dest))
}
