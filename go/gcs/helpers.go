package gcs

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
)

// This file implements utility functions for accessing data in Google Storage.

var (
	// dirMap maps dataset name to a slice with Google Storage subdirectory and file prefix.
	dirMap = map[string][]string{
		"skps":  {"pics-json-v2", "bench_"},
		"micro": {"stats-json-v2", "microbench2_"},
	}

	trybotDataPath = regexp.MustCompile(`^[a-z]*[/]?([0-9]{4}/[0-9]{2}/[0-9]{2}/[0-9]{2}/[0-9a-zA-Z-]+-Trybot/[0-9]+/[0-9]+)$`)
)

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

// DeleteAllFilesInDir deletes all the files in a given folder.  If processes is set to > 1,
// that many go routines will be spun up to delete the file simultaneously. Otherwise, it will
// be done one one process.
func DeleteAllFilesInDir(s *storage.Client, bucket, folder string, processes int) error {
	if processes <= 0 {
		processes = 1
	}
	errCount := int32(0)
	var wg sync.WaitGroup
	toDelete := make(chan string, 1000)
	for i := 0; i < processes; i++ {
		wg.Add(1)
		go deleteHelper(s, bucket, &wg, toDelete, &errCount)
	}
	del := func(item *storage.ObjectAttrs) {
		toDelete <- item.Name
	}
	if err := AllFilesInDir(s, bucket, folder, del); err != nil {
		return err
	}
	close(toDelete)
	wg.Wait()
	if errCount > 0 {
		return fmt.Errorf("There were one or more problems when deleting files in folder %q", folder)
	}
	return nil

}

// deleteHelper spins and waits for work to come in on the toDelete channel.  When it does, it
// uses the storage client to delete the file from the given bucket.
func deleteHelper(s *storage.Client, bucket string, wg *sync.WaitGroup, toDelete <-chan string, errCount *int32) {
	defer wg.Done()
	for file := range toDelete {
		if err := s.Bucket(bucket).Object(file).Delete(context.Background()); err != nil {
			// Ignore 404 errors on deleting, as they are already gone.
			if !strings.Contains(err.Error(), "statuscode 404") {
				sklog.Errorf("Problem deleting gs://%s/%s: %s", bucket, file, err)
				atomic.AddInt32(errCount, 1)
			}
		}
	}
}

// Write the given content to the given object in Google Storage.
func WriteObj(o *storage.ObjectHandle, content []byte) (err error) {
	w := o.NewWriter(context.Background())
	w.ObjectAttrs.ContentEncoding = "gzip"
	if err := util.WithGzipWriter(w, func(w io.Writer) error {
		_, err := w.Write(content)
		return err
	}); err != nil {
		_ = w.CloseWithError(err) // Always returns nil, according to docs.
		return err
	}
	return w.Close()
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
		cancel()
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
