// Package gs implements utility for accessing data in Google Storage.
package gcs

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
)

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

// DownloadHelper provides convenience methods for downloading binaries by SHA1
// sum.
type DownloadHelper struct {
	bucket  string
	s       *storage.Client
	subdir  string
	workdir string
}

// NewDownloadHelper returns a DownloadHelper instance.
func NewDownloadHelper(s *storage.Client, gsBucket, gsSubdir, workdir string) *DownloadHelper {
	return &DownloadHelper{
		bucket:  gsBucket,
		s:       s,
		subdir:  gsSubdir,
		workdir: workdir,
	}
}

// Download downloads the given binary from Google Storage.
func (d *DownloadHelper) Download(name, hash string) error {
	sklog.Infof("Downloading new binary for %s...", name)
	filepath := path.Join(d.workdir, name)
	object := hash
	if d.subdir != "" {
		object = d.subdir + "/" + object
	}
	resp, err := d.s.Bucket(d.bucket).Object(object).NewReader(context.Background())
	if err != nil {
		return fmt.Errorf("Download helper can't get reader for %s: %s", name, err)
	}
	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("Download helper cannot create filepath %s: %s", filepath, err)
	}
	defer util.Close(f)
	if _, err := io.Copy(f, resp); err != nil {
		return fmt.Errorf("Download helper can't download %s: %s", name, err)
	}
	if err := f.Chmod(0755); err != nil {
		return err
	}
	return nil
}

// MaybeDownload downloads the given binary from Google Storage if necessary.
func (d *DownloadHelper) MaybeDownload(name, hash string) error {
	filepath := path.Join(d.workdir, name)
	f, err := os.Open(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return d.Download(name, hash)
		} else {
			return fmt.Errorf("Failed to open %s: %s", filepath, err)
		}
	}
	defer util.Close(f)
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("Failed to stat %s: %s", filepath, err)
	}
	if info.Mode() != 0755 {
		sklog.Infof("Binary %s is not executable.", filepath)
		return d.Download(name, hash)
	}

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return fmt.Errorf("Failed to read %s: %s", filepath, err)
	}
	sha1sum := sha1.Sum(contents)
	sha1str := fmt.Sprintf("%x", sha1sum)
	if sha1str != hash {
		sklog.Infof("Binary %s is out of date:\nExpect: %s\nGot:    %s", filepath, hash, sha1str)
		return d.Download(name, hash)
	}
	return nil
}

// Close should be called when finished with the DownloadHelper.
func (d *DownloadHelper) Close() error {
	return d.s.Close()
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
