package common

// TODO(kjlubick): Move this to package storage, where possible/reasonable.
import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/sklog"
)

// ExtractFuzzNameFromPath turns a path name into a fuzz name by stripping off all but the
// last piece from the path.
func ExtractFuzzNameFromPath(path string) (name string) {
	return path[strings.LastIndex(path, "/")+1:]
}

// ExtractFuzzNamesFromPaths turns all path names into just fuzz names, by extracting the
// last piece of the path.
func ExtractFuzzNamesFromPaths(paths []string) (names []string) {
	names = make([]string, 0, len(paths))
	for _, path := range paths {
		names = append(names, ExtractFuzzNameFromPath(path))
	}
	return names
}

// GetAllFuzzNamesInFolder returns all the fuzz names in a given GCS folder.  It basically
// returns a list of all files that don't end with a .dump or .err, or error
// if there was a problem.
func GetAllFuzzNamesInFolder(s *storage.Client, name string) (hashes []string, err error) {
	filter := func(item *storage.ObjectAttrs) {
		name := item.Name
		fuzzHash := name[strings.LastIndex(name, "/")+1:]
		if !IsNameOfFuzz(fuzzHash) {
			return
		}
		hashes = append(hashes, fuzzHash)
	}

	if err = gs.AllFilesInDir(s, config.GS.Bucket, name, filter); err != nil {
		return hashes, fmt.Errorf("Problem getting fuzzes from folder %s: %s", name, err)
	}
	return hashes, nil
}

// IsNameOfFuzz returns true if the GCS file name given is a fuzz, which is basically if it doesn't
// have a . in it.
func IsNameOfFuzz(name string) bool {
	return name != "" && !strings.Contains(name, ".")
}

// DownloadAllFuzzes downloads all fuzzes of a given type "bad", "grey" at the specified revision
// and returns a slice of all the paths on disk where they are.
func DownloadAllFuzzes(s *storage.Client, downloadPath, category, revision, architecture, fuzzType string, processes int) ([]string, error) {
	completedCount := int32(0)
	var wg sync.WaitGroup
	toDownload := make(chan string, 1000)
	for i := 0; i < processes; i++ {
		go download(s, toDownload, downloadPath, &wg, &completedCount)
	}
	fuzzPaths := []string{}

	download := func(item *storage.ObjectAttrs) {
		name := item.Name
		if !IsNameOfFuzz(name) {
			return
		}
		fuzzHash := name[strings.LastIndex(name, "/")+1:]
		if fuzzHash == "" {
			return
		}
		fuzzPaths = append(fuzzPaths, filepath.Join(downloadPath, fuzzHash))
		toDownload <- item.Name
	}
	if err := gs.AllFilesInDir(s, config.GS.Bucket, fmt.Sprintf("%s/%s/%s/%s", category, revision, architecture, fuzzType), download); err != nil {
		return nil, fmt.Errorf("Problem iterating through all files: %s", err)
	}
	close(toDownload)
	wg.Wait()

	return fuzzPaths, nil
}

// download starts a go routine that waits for files to download from Google Storage and downloads
// them to downloadPath.  When it is done (on error or when the channel is closed), it signals to
// the WaitGroup that it is done. It also logs the progress on downloading the fuzzes.
func download(storageClient *storage.Client, toDownload <-chan string, downloadPath string, wg *sync.WaitGroup, completedCounter *int32) {
	wg.Add(1)
	defer wg.Done()
	for file := range toDownload {
		hash := file[strings.LastIndex(file, "/")+1:]
		onDisk := filepath.Join(downloadPath, hash)
		if !fileutil.FileExists(onDisk) {
			contents, err := gs.FileContentsFromGS(storageClient, config.GS.Bucket, file)
			if err != nil {
				sklog.Warningf("Problem downloading fuzz %s, continuing anyway: %s", file, err)
				continue
			}
			if err = ioutil.WriteFile(onDisk, contents, 0644); err != nil && !os.IsExist(err) {
				sklog.Warningf("Problem writing fuzz to %s, continuing anyway: %s", onDisk, err)
			}
		}
		atomic.AddInt32(completedCounter, 1)
		if *completedCounter%100 == 0 {
			sklog.Infof("%d fuzzes downloaded", *completedCounter)
		}
	}
}
