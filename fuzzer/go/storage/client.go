package storage

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/sklog"
)

// FuzzerGCSClient is the interface for all fuzzer-specific Google Cloud Storage (GCS)
// interactions. It embeds gcs.GCSClient to extend that functionality.
// See also go/fuzzer/tests.NewMockGCSClient() for a mock.
type FuzzerGCSClient interface {
	gcs.GCSClient
	// GetAllFuzzNamesInFolder returns all the fuzz names in a given GCS folder.  It basically
	// returns a list of all files that don't end with a .dump or .err, or error
	// if there was a problem.
	GetAllFuzzNamesInFolder(name string) (hashes []string, err error)
	// DeleteAllFilesInFolder recursively deletes anything in the given folder, i.e.
	// that starts with the given prefix. It can run on multiple go routines if processes
	// is set to > 1.
	DeleteAllFilesInFolder(folder string, processes int) error

	// DownloadAllFuzzes downloads all fuzzes of a given type "bad", "grey" at the specified
	// revision and returns a slice of all the paths on disk where they are. It can run on
	// multiple go routines if processes is set to > 1.
	DownloadAllFuzzes(downloadToPath, category, revision, architecture, fuzzType string, processes int) ([]string, error)
}

type fuzzerclient struct {
	gcs.GCSClient
}

func NewFuzzerGCSClient(s *storage.Client, bucket string) FuzzerGCSClient {
	return &fuzzerclient{gcsclient.New(s, bucket)}
}

func (g *fuzzerclient) GetAllFuzzNamesInFolder(name string) (hashes []string, err error) {
	filter := func(item *storage.ObjectAttrs) {
		name := item.Name
		fuzzHash := name[strings.LastIndex(name, "/")+1:]
		if !common.IsNameOfFuzz(fuzzHash) {
			return
		}
		hashes = append(hashes, fuzzHash)
	}

	if err = g.AllFilesInDirectory(context.Background(), name, filter); err != nil {
		return hashes, fmt.Errorf("Problem getting fuzzes from folder %s: %s", name, err)
	}
	return hashes, nil
}

// See the FuzzerGCSClient for more information about DeleteAllFilesInFolder.
func (g *fuzzerclient) DeleteAllFilesInFolder(folder string, processes int) error {
	if processes <= 0 {
		processes = 1
	}
	errCount := int32(0)
	var wg sync.WaitGroup
	toDelete := make(chan string, 1000)
	for i := 0; i < processes; i++ {
		wg.Add(1)
		go g.deleteHelper(&wg, toDelete, &errCount)
	}
	del := func(item *storage.ObjectAttrs) {
		toDelete <- item.Name
	}
	if err := g.AllFilesInDirectory(context.Background(), folder, del); err != nil {
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
func (g *fuzzerclient) deleteHelper(wg *sync.WaitGroup, toDelete <-chan string, errCount *int32) {
	defer wg.Done()
	for file := range toDelete {
		if err := g.DeleteFile(context.Background(), file); err != nil {
			// Ignore 404 errors on deleting, as they are already gone.
			if !strings.Contains(err.Error(), "statuscode 404") {
				sklog.Errorf("Problem deleting gs://%s/%s: %s", g.Bucket(), file, err)
				atomic.AddInt32(errCount, 1)
			}
		}
	}
}

// See the FuzzerGCSClient for more information about DownloadAllFuzzes.
func (g *fuzzerclient) DownloadAllFuzzes(downloadPath, category, revision, architecture, fuzzType string, processes int) ([]string, error) {
	completedCount := int32(0)
	var wg sync.WaitGroup
	toDownload := make(chan string, 1000)
	for i := 0; i < processes; i++ {
		wg.Add(1)
		go g.downloadHelper(toDownload, downloadPath, &wg, &completedCount)
	}
	fuzzPaths := []string{}

	download := func(item *storage.ObjectAttrs) {
		name := item.Name
		if !common.IsNameOfFuzz(name) {
			return
		}
		fuzzHash := name[strings.LastIndex(name, "/")+1:]
		if fuzzHash == "" {
			return
		}
		fuzzPaths = append(fuzzPaths, filepath.Join(downloadPath, fuzzHash))
		toDownload <- item.Name
	}
	if err := g.AllFilesInDirectory(context.Background(), fmt.Sprintf("%s/%s/%s/%s", category, revision, architecture, fuzzType), download); err != nil {
		return nil, fmt.Errorf("Problem iterating through all files: %s", err)
	}
	close(toDownload)
	wg.Wait()

	return fuzzPaths, nil
}

// download starts a go routine that waits for files to download from Google Storage and downloads
// them to downloadPath.  When it is done (on error or when the channel is closed), it signals to
// the WaitGroup that it is done. It also logs the progress on downloading the fuzzes.
func (g *fuzzerclient) downloadHelper(toDownload <-chan string, downloadPath string, wg *sync.WaitGroup, completedCounter *int32) {
	defer wg.Done()
	for file := range toDownload {
		hash := file[strings.LastIndex(file, "/")+1:]
		onDisk := filepath.Join(downloadPath, hash)
		if !fileutil.FileExists(onDisk) {
			contents, err := g.GetFileContents(context.Background(), file)
			if err != nil {
				sklog.Warningf("Problem downloading fuzz %s, continuing anyway: %s", file, err)
				continue
			}
			if err = ioutil.WriteFile(onDisk, contents, 0644); err != nil && !os.IsExist(err) {
				sklog.Warningf("Problem writing fuzz to %s, continuing anyway: %s", onDisk, err)
			}
		}
		i := atomic.AddInt32(completedCounter, 1)
		if i%100 == 0 {
			sklog.Infof("%d fuzzes downloaded", i)
		}
	}
}
