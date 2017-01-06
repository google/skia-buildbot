package storage

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/fuzzer/go/data"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

type GCSClient struct {
	client *storage.Client
	bucket string
}

func NewFuzzerGCSClient(s *storage.Client, bucket string) *GCSClient {
	return &GCSClient{
		client: s,
		bucket: bucket,
	}
}

type GCSFileGetter interface {
	GetFileContents(path string) ([]byte, error)
}

type GCSFileSetter interface {
	SetFileContents(path, encoding string, contents []byte) error
	FileWriter(path, encoding string) io.WriteCloser
}

type GCSFolderOperator interface {
	ExecuteOnAllFilesInFolder(folder string, callback func(item *storage.ObjectAttrs)) error
	DeleteAllFilesInDir(folder string, processes int) error
}

type GCSFuzzNameGetter interface {
	GetAllFuzzNamesInFolder(folder string) (hashes []string, err error)
}

type GCSFuzzGetter interface {
	DownloadAllFuzzes(downloadPath, category, revision, architecture, fuzzType string, processes int) ([]string, error)
}

type GCSReportGetter interface {
	GetReportsFromGS(baseFolder, category, architecture string, whitelist []string, processes int) (<-chan data.FuzzReport, error)
}

func (g *GCSClient) GetFileContents(path string) ([]byte, error) {
	return gs.FileContentsFromGS(g.client, g.bucket, path)
}

// GetAllFuzzNamesInFolder returns all the fuzz names in a given GCS folder.  It basically
// returns a list of all files that don't end with a .dump or .err, or error
// if there was a problem.
func (g *GCSClient) GetAllFuzzNamesInFolder(name string) (hashes []string, err error) {
	filter := func(item *storage.ObjectAttrs) {
		name := item.Name
		fuzzHash := name[strings.LastIndex(name, "/")+1:]
		if !IsNameOfFuzz(fuzzHash) {
			return
		}
		hashes = append(hashes, fuzzHash)
	}

	if err = gs.AllFilesInDir(g.client, g.bucket, name, filter); err != nil {
		return hashes, fmt.Errorf("Problem getting fuzzes from folder %s: %s", name, err)
	}
	return hashes, nil
}

func (g *GCSClient) SetFileContents(path, encoding string, contents []byte) error {
	w := g.FileWriter(path, encoding)
	defer util.Close(w)

	if n, err := w.Write(contents); err != nil {
		return fmt.Errorf("There was a problem uploading %s.  Only uploaded %d bytes: %s", path, n, err)
	}
	return nil
}

func (g *GCSClient) FileWriter(path, encoding string) io.WriteCloser {
	w := g.client.Bucket(g.bucket).Object(path).NewWriter(context.Background())
	w.ObjectAttrs.ContentEncoding = encoding
	return w
}

func (g *GCSClient) ExecuteOnAllFilesInFolder(folder string, callback func(item *storage.ObjectAttrs)) error {
	return gs.AllFilesInDir(g.client, g.bucket, folder, callback)
}

func (g *GCSClient) DeleteAllFilesInDir(folder string, processes int) error {
	return gs.DeleteAllFilesInDir(g.client, g.bucket, folder, processes)
}

// DownloadAllFuzzes downloads all fuzzes of a given type "bad", "grey" at the specified revision
// and returns a slice of all the paths on disk where they are.
func (g *GCSClient) DownloadAllFuzzes(downloadPath, category, revision, architecture, fuzzType string, processes int) ([]string, error) {
	completedCount := int32(0)
	var wg sync.WaitGroup
	toDownload := make(chan string, 1000)
	for i := 0; i < processes; i++ {
		go g.downloadGCSFiles(toDownload, downloadPath, &wg, &completedCount)
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
	if err := gs.AllFilesInDir(g.client, g.bucket, fmt.Sprintf("%s/%s/%s/%s", category, revision, architecture, fuzzType), download); err != nil {
		return nil, fmt.Errorf("Problem iterating through all files: %s", err)
	}
	close(toDownload)
	wg.Wait()

	return fuzzPaths, nil
}

// downloadGCSFiles starts a go routine that waits for files to download from Google Storage and downloads
// them to downloadPath.  When it is done (on error or when the channel is closed), it signals to
// the WaitGroup that it is done. It also logs the progress on downloading the fuzzes.
func (g *GCSClient) downloadGCSFiles(toDownload <-chan string, downloadPath string, wg *sync.WaitGroup, completedCounter *int32) {
	wg.Add(1)
	defer wg.Done()
	for file := range toDownload {
		hash := file[strings.LastIndex(file, "/")+1:]
		onDisk := filepath.Join(downloadPath, hash)
		if !fileutil.FileExists(onDisk) {
			contents, err := gs.FileContentsFromGS(g.client, g.bucket, file)
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

// GetReportsFromGS fetches all fuzz reports in the baseFolder from Google Storage. It returns a
// channel through which all reports will be sent. The channel will be closed when finished. An
// optional whitelist can be included, in which case only the fuzzes whose names are on the list
// will be downloaded.  The category is needed to properly parse the downloaded files to make
// the FuzzReports.  The downloading will use as many processes as specified, to speed things up.
func (g *GCSClient) GetReportsFromGS(baseFolder, category, architecture string, whitelist []string, processes int) (<-chan data.FuzzReport, error) {
	reports := make(chan data.FuzzReport, 10000)

	fuzzPackages, err := g.fetchFuzzPackages(baseFolder, category, architecture)
	if err != nil {
		close(reports)
		return reports, err
	}

	toDownload := make(chan fuzzPackage, len(fuzzPackages))
	completedCounter := int32(0)

	var wg sync.WaitGroup
	for i := 0; i < processes; i++ {
		wg.Add(1)
		go g.downloadReports(toDownload, reports, &completedCounter, &wg)
	}

	for _, d := range fuzzPackages {
		if whitelist != nil {
			name := d.FuzzName
			if i := sort.SearchStrings(whitelist, name); i < len(whitelist) && whitelist[i] == name {
				// is on the whitelist
				toDownload <- d
			}
		} else {
			// no white list
			toDownload <- d
		}
	}
	close(toDownload)

	// Wait until all are done downloading to close the reports channel, but don't block
	go func() {
		wg.Wait()
		close(reports)
	}()

	return reports, nil
}

// A fuzzPackage contains all the information about a fuzz, mostly the paths to the files that
// need to be downloaded.  The use of this struct decouples the names of the files that need to be
// downloaded with the download logic.
type fuzzPackage struct {
	FuzzName         string
	FuzzCategory     string
	FuzzArchitecture string
	DebugASANName    string
	DebugDumpName    string
	DebugErrName     string
	ReleaseASANName  string
	ReleaseDumpName  string
	ReleaseErrName   string
}

// fetchFuzzPackages scans for all fuzzes in the given folder and returns a slice of all of the
// metadata for each fuzz, as a fuzz package.  It returns error if it cannot access Google Storage.
func (g *GCSClient) fetchFuzzPackages(baseFolder, category, architecture string) (fuzzPackages []fuzzPackage, err error) {
	fuzzNames, err := g.GetAllFuzzNamesInFolder(baseFolder)
	if err != nil {
		return nil, fmt.Errorf("Problem getting fuzz packages from %s: %s", baseFolder, err)
	}
	for _, fuzzName := range fuzzNames {
		prefix := fmt.Sprintf("%s/%s/%s", baseFolder, fuzzName, fuzzName)
		fuzzPackages = append(fuzzPackages, fuzzPackage{
			FuzzName:         fuzzName,
			FuzzCategory:     category,
			FuzzArchitecture: architecture,
			DebugASANName:    fmt.Sprintf("%s_debug.asan", prefix),
			DebugDumpName:    fmt.Sprintf("%s_debug.dump", prefix),
			DebugErrName:     fmt.Sprintf("%s_debug.err", prefix),
			ReleaseASANName:  fmt.Sprintf("%s_release.asan", prefix),
			ReleaseDumpName:  fmt.Sprintf("%s_release.dump", prefix),
			ReleaseErrName:   fmt.Sprintf("%s_release.err", prefix),
		})
	}
	return fuzzPackages, nil
}

// emptyStringOnError returns a string of the passed in bytes or empty string if err is nil.
func emptyStringOnError(b []byte, err error) string {
	if err != nil {
		sklog.Warningf("Ignoring error when fetching file contents: %v", err)
		return ""
	}
	return string(b)
}

// downloadReports waits for fuzzPackages to appear on the toDownload channel and then downloads
// the four pieces of the package.  It then parses them into a BinaryFuzzReport and sends
// the binary to the passed in channel.  When there is no more work to be done, this function.
// returns and writes out true to the done channel.
func (g *GCSClient) downloadReports(toDownload <-chan fuzzPackage, reports chan<- data.FuzzReport, completedCounter *int32, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range toDownload {
		p := data.GCSPackage{
			Name:             job.FuzzName,
			FuzzCategory:     job.FuzzCategory,
			FuzzArchitecture: job.FuzzArchitecture,
			Debug: data.OutputFiles{
				Asan:   emptyStringOnError(gs.FileContentsFromGS(g.client, g.bucket, job.DebugASANName)),
				Dump:   emptyStringOnError(gs.FileContentsFromGS(g.client, g.bucket, job.DebugDumpName)),
				StdErr: emptyStringOnError(gs.FileContentsFromGS(g.client, g.bucket, job.DebugErrName)),
			},
			Release: data.OutputFiles{
				Asan:   emptyStringOnError(gs.FileContentsFromGS(g.client, g.bucket, job.ReleaseASANName)),
				Dump:   emptyStringOnError(gs.FileContentsFromGS(g.client, g.bucket, job.ReleaseDumpName)),
				StdErr: emptyStringOnError(gs.FileContentsFromGS(g.client, g.bucket, job.ReleaseErrName)),
			},
		}

		reports <- data.ParseReport(p)
		atomic.AddInt32(completedCounter, 1)
		if *completedCounter%100 == 0 {
			sklog.Infof("%d fuzzes downloaded", *completedCounter)
		}
	}
}
