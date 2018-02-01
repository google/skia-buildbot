package storage

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/data"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/sklog"
)

// GetReportsFromGCS fetches all fuzz reports in the baseFolder from Google Storage. It returns a
// channel through which all reports will be sent. The channel will be closed when finished. An
// optional whitelist can be included, in which case only the fuzzes whose names are on the list
// will be downloaded.  The category is needed to properly parse the downloaded files to make
// the FuzzReports.  The downloading will use as many processes as specified, to speed things up.
func GetReportsFromGCS(s *storage.Client, baseFolder, category, architecture string, whitelist []string, processes int) (<-chan data.FuzzReport, error) {
	reports := make(chan data.FuzzReport, 10000)

	fuzzPackages, err := fetchFuzzPackages(s, baseFolder, category, architecture)
	if err != nil {
		close(reports)
		return reports, err
	}

	toDownload := make(chan fuzzPackage, len(fuzzPackages))
	completedCounter := int32(0)

	var wg sync.WaitGroup
	for i := 0; i < processes; i++ {
		wg.Add(1)
		go download(s, toDownload, reports, &completedCounter, &wg)
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
func fetchFuzzPackages(s *storage.Client, baseFolder, category, architecture string) (fuzzPackages []fuzzPackage, err error) {
	fuzzNames, err := common.GetAllFuzzNamesInFolder(s, baseFolder)
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

// download waits for fuzzPackages to appear on the toDownload channel and then downloads
// the four pieces of the package.  It then parses them into a BinaryFuzzReport and sends
// the binary to the passed in channel.  When there is no more work to be done, this function.
// returns and writes out true to the done channel.
func download(s *storage.Client, toDownload <-chan fuzzPackage, reports chan<- data.FuzzReport, completedCounter *int32, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range toDownload {
		p := data.GCSPackage{
			Name:             job.FuzzName,
			FuzzCategory:     job.FuzzCategory,
			FuzzArchitecture: job.FuzzArchitecture,
			Files: map[string]data.OutputFiles{
				"ASAN_DEBUG": {
					Key: "ASAN_DEBUG",
					Content: map[string]string{
						"stderr": emptyStringOnError(gcs.FileContentsFromGCS(s, config.GCS.Bucket, job.DebugASANName)),
					},
				},
				"CLANG_DEBUG": {
					Key: "CLANG_DEBUG",
					Content: map[string]string{
						"stdout": emptyStringOnError(gcs.FileContentsFromGCS(s, config.GCS.Bucket, job.DebugDumpName)),
						"stderr": emptyStringOnError(gcs.FileContentsFromGCS(s, config.GCS.Bucket, job.DebugErrName)),
					},
				},
				"ASAN_RELEASE": {
					Key: "ASAN_RELEASE",
					Content: map[string]string{
						"stderr": emptyStringOnError(gcs.FileContentsFromGCS(s, config.GCS.Bucket, job.ReleaseASANName)),
					},
				},
				"CLANG_RELEASE": {
					Key: "CLANG_RELEASE",
					Content: map[string]string{
						"stdout": emptyStringOnError(gcs.FileContentsFromGCS(s, config.GCS.Bucket, job.ReleaseDumpName)),
						"stderr": emptyStringOnError(gcs.FileContentsFromGCS(s, config.GCS.Bucket, job.ReleaseErrName)),
					},
				},
			},
		}

		reports <- data.ParseReport(p)
		atomic.AddInt32(completedCounter, 1)
		if *completedCounter%100 == 0 {
			sklog.Infof("%d fuzzes downloaded", *completedCounter)
		}
	}
}
