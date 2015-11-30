package frontend

import (
	"fmt"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/fuzz"
	"go.skia.org/infra/go/gs"
	storage "google.golang.org/api/storage/v1"
)

// LoadFromGoogleStorage pulls all fuzzes out of GCS and loads them into memory.
// Upon returning nil error, the results can be accessed via fuzz.FuzzSummary() and
// fuzz.FuzzDetails().
func LoadFromGoogleStorage(storageService *storage.Service, nameLookup fuzz.FindFunctionName) error {

	reports, err := getBinaryReportsFromGS(storageService, "binary_fuzzes/bad/")
	if err != nil {
		return err
	}

	for _, report := range reports {
		// TODO(kjlubick) : make the name lookup work
		report.DebugStackTrace.LookUpFunctions(nameLookup)
		report.ReleaseStackTrace.LookUpFunctions(nameLookup)
		fuzz.NewBinaryFuzzFound(report)
	}

	return nil
}

// getBinaryReportsFromGS pulls all files in baseFolder from the skia-fuzzer bucket and
// groups them by fuzz.  It parses these groups of files into a FuzzReportBinary and returns
// the slice of all reports generated in this way.
func getBinaryReportsFromGS(storageService *storage.Service, baseFolder string) ([]fuzz.FuzzReportBinary, error) {
	contents, err := storageService.Objects.List("skia-fuzzer").Prefix(baseFolder).Fields("nextPageToken", "items(name,size,timeCreated)").MaxResults(100000).Do()
	// Assumption, files are sorted alphabetically and have the structure
	// [baseFolder]/[filetype]/[fuzzname]/[fuzzname][suffix]
	// where suffix is one of _debug.dump, _debug.err, _release.dump or _release.err
	if err != nil {
		return nil, fmt.Errorf("Problem reading from Google Storage: %v", err)
	}

	glog.Infof("Loading %d files from gs://skia-fuzzer/%s", len(contents.Items), baseFolder)

	reports := make([]fuzz.FuzzReportBinary, 0)

	var debugDump, debugErr, releaseDump, releaseErr string
	isInitialized := false
	currFuzzFolder := "" // will be something like binary_fuzzes/bad/skp/badbeef
	currFuzzName := ""
	currFuzzType := ""
	for _, item := range contents.Items {
		name := item.Name
		if strings.Count(name, "/") <= 3 {
			continue
		}

		if !isInitialized || !strings.HasPrefix(name, currFuzzFolder) {
			if isInitialized {
				reports = append(reports, fuzz.ParseBinaryReport(currFuzzType, currFuzzName, debugDump, debugErr, releaseDump, releaseErr))
			} else {
				isInitialized = true
			}

			parts := strings.Split(name, "/")
			currFuzzFolder = strings.Join(parts[0:4], "/")
			currFuzzType = parts[2]
			currFuzzName = parts[3]
			// reset for next one
			debugDump, debugErr, releaseDump, releaseErr = "", "", "", ""

		}
		if strings.HasSuffix(name, "_debug.dump") {
			debugDump = emptyStringOnError(gs.FileContentsFromGS(storageService, "skia-fuzzer", name))
		} else if strings.HasSuffix(name, "_debug.err") {
			debugErr = emptyStringOnError(gs.FileContentsFromGS(storageService, "skia-fuzzer", name))
		} else if strings.HasSuffix(name, "_release.dump") {
			releaseDump = emptyStringOnError(gs.FileContentsFromGS(storageService, "skia-fuzzer", name))
		} else if strings.HasSuffix(name, "_release.err") {
			releaseErr = emptyStringOnError(gs.FileContentsFromGS(storageService, "skia-fuzzer", name))
		}
	}

	if currFuzzName != "" {
		reports = append(reports, fuzz.ParseBinaryReport(currFuzzType, currFuzzName, debugDump, debugErr, releaseDump, releaseErr))
	}
	glog.Info("Done loading")
	return reports, nil
}

// emptyStringOnError returns a string of the passed in bytes or empty string if err is nil.
func emptyStringOnError(b []byte, err error) string {
	if err != nil {
		glog.Infof("Ignoring error when fetching file contents: %v", err)
		return ""
	}
	return string(b)
}
