package frontend

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/functionnamefinder"
	"go.skia.org/infra/fuzzer/go/fuzz"
	"go.skia.org/infra/go/gs"
	"google.golang.org/cloud/storage"
)

var completedCounter int32

// LoadFromGoogleStorage pulls all fuzzes out of GCS and loads them into memory.
// The fuzzes are streamed into memory as they are fetched.  The partial result is available
// via fuzz.FuzzSummary() and fuzz.FuzzDetails() prior to this function returning.
func LoadFromGoogleStorage(storageClient *storage.Client, finder functionnamefinder.Finder) error {
	reports, err := getBinaryReportsFromGS(storageClient, fmt.Sprintf("binary_fuzzes/%s/bad/", config.Common.SkiaVersion.Hash))
	if err != nil {
		return err
	}

	for report := range reports {
		if finder != nil {
			report.DebugStackTrace.LookUpFunctions(finder)
			report.ReleaseStackTrace.LookUpFunctions(finder)
		}
		fuzz.NewBinaryFuzzFound(report)
	}
	glog.Info("All fuzzes loaded from Google Storage")

	return nil
}

type fuzzPackage struct {
	FuzzType        string
	FuzzName        string
	DebugDumpName   string
	DebugErrName    string
	ReleaseDumpName string
	ReleaseErrName  string
}

// getBinaryReportsFromGS pulls all files in baseFolder from the skia-fuzzer bucket and
// groups them by fuzz.  It parses these groups of files into a FuzzReportBinary and returns
// the slice of all reports generated in this way.
func getBinaryReportsFromGS(storageClient *storage.Client, baseFolder string) (<-chan fuzz.FuzzReportBinary, error) {
	reports := make(chan fuzz.FuzzReportBinary, 100)

	fuzzPackages, err := fetchFuzzPackages(storageClient, baseFolder)
	if err != nil {
		close(reports)
		return reports, err
	}

	toDownload := make(chan fuzzPackage, len(fuzzPackages))
	completedCounter = 0

	var wg sync.WaitGroup
	for i := 0; i < config.FrontEnd.NumDownloadProcesses; i++ {
		wg.Add(1)
		go download(storageClient, toDownload, reports, &wg)
	}

	for _, d := range fuzzPackages {
		toDownload <- d
	}
	close(toDownload)

	go func() {
		wg.Wait()
		close(reports)
	}()

	return reports, nil
}

// fetchFuzzPackages scans for all fuzzes in the given folder and returns a
// slice of all of the metadata for each fuzz, as a fuzz package.  It returns
// error if it cannot access Google Storage.
func fetchFuzzPackages(storageClient *storage.Client, baseFolder string) (fuzzPackages []fuzzPackage, err error) {
	var debugDump, debugErr, releaseDump, releaseErr string
	isInitialized := false
	currFuzzFolder := "" // will be something like binary_fuzzes/bad/skp/badbeef
	currFuzzName := ""
	currFuzzType := ""

	err = gs.AllFilesInDir(storageClient, config.GS.Bucket, baseFolder, func(item *storage.ObjectAttrs) {
		// Assumption, files are sorted alphabetically and have the structure
		// [baseFolder]/[filetype]/[fuzzname]/[fuzzname][suffix]
		// where suffix is one of _debug.dump, _debug.err, _release.dump or _release.err
		name := item.Name
		if name == baseFolder || strings.Count(name, "/") <= 4 {
			return
		}

		if !isInitialized || !strings.HasPrefix(name, currFuzzFolder) {
			if isInitialized {
				fuzzPackages = append(fuzzPackages, fuzzPackage{
					FuzzType:        currFuzzType,
					FuzzName:        currFuzzName,
					DebugDumpName:   debugDump,
					DebugErrName:    debugErr,
					ReleaseDumpName: releaseDump,
					ReleaseErrName:  releaseErr,
				})
			} else {
				isInitialized = true
			}

			parts := strings.Split(name, "/")
			currFuzzFolder = strings.Join(parts[0:5], "/")
			currFuzzType = parts[3]
			currFuzzName = parts[4]
			// reset for next one
			debugDump, debugErr, releaseDump, releaseErr = "", "", "", ""
		}
		if strings.HasSuffix(name, "_debug.dump") {
			debugDump = name
		} else if strings.HasSuffix(name, "_debug.err") {
			debugErr = name
		} else if strings.HasSuffix(name, "_release.dump") {
			releaseDump = name
		} else if strings.HasSuffix(name, "_release.err") {
			releaseErr = name
		}
	})
	if err != nil {
		return fuzzPackages, err
	}
	if currFuzzName != "" {
		fuzzPackages = append(fuzzPackages, fuzzPackage{
			FuzzType:        currFuzzType,
			FuzzName:        currFuzzName,
			DebugDumpName:   debugDump,
			DebugErrName:    debugErr,
			ReleaseDumpName: releaseDump,
			ReleaseErrName:  releaseErr,
		})
	}
	return fuzzPackages, nil
}

// emptyStringOnError returns a string of the passed in bytes or empty string if err is nil.
func emptyStringOnError(b []byte, err error) string {
	if err != nil {
		glog.Warningf("Ignoring error when fetching file contents: %v", err)
		return ""
	}
	return string(b)
}

// download waits for fuzzPackages to appear on the toDownload channel and then downloads
// the four pieces of the package.  It then parses them into a FuzzReportBinary and sends
// the binary to the passed in channel.  When there is no more work to be done, this function.
// returns and writes out true to the done channel.
func download(storageClient *storage.Client, toDownload <-chan fuzzPackage, reports chan<- fuzz.FuzzReportBinary, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range toDownload {
		debugDump := emptyStringOnError(gs.FileContentsFromGS(storageClient, config.GS.Bucket, job.DebugDumpName))
		debugErr := emptyStringOnError(gs.FileContentsFromGS(storageClient, config.GS.Bucket, job.DebugErrName))
		releaseDump := emptyStringOnError(gs.FileContentsFromGS(storageClient, config.GS.Bucket, job.ReleaseDumpName))
		releaseErr := emptyStringOnError(gs.FileContentsFromGS(storageClient, config.GS.Bucket, job.ReleaseErrName))
		reports <- fuzz.ParseBinaryReport(job.FuzzType, job.FuzzName, debugDump, debugErr, releaseDump, releaseErr)
		atomic.AddInt32(&completedCounter, 1)
		if completedCounter%100 == 0 {
			glog.Infof("%d fuzzes downloaded", completedCounter)
		}
	}
}
