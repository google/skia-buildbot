package frontend

import (
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/functionnamefinder"
	"go.skia.org/infra/fuzzer/go/fuzz"
	"go.skia.org/infra/go/gs"
	"google.golang.org/cloud/storage"
)

// LoadFromGoogleStorage pulls all fuzzes out of GCS and loads them into memory.
// Upon returning nil error, the results can be accessed via fuzz.FuzzSummary() and
// fuzz.FuzzDetails().
func LoadFromGoogleStorage(storageClient *storage.Client, finder functionnamefinder.Finder) error {
	reports, err := getBinaryReportsFromGS(storageClient, "binary_fuzzes/bad/")
	if err != nil {
		return err
	}

	for _, report := range reports {
		if finder != nil {
			report.DebugStackTrace.LookUpFunctions(finder)
			report.ReleaseStackTrace.LookUpFunctions(finder)
		}
		fuzz.NewBinaryFuzzFound(report)
	}

	return nil
}

// getBinaryReportsFromGS pulls all files in baseFolder from the skia-fuzzer bucket and
// groups them by fuzz.  It parses these groups of files into a FuzzReportBinary and returns
// the slice of all reports generated in this way.
func getBinaryReportsFromGS(storageClient *storage.Client, baseFolder string) ([]fuzz.FuzzReportBinary, error) {
	reports := make([]fuzz.FuzzReportBinary, 0)

	var debugDump, debugErr, releaseDump, releaseErr string
	isInitialized := false
	currFuzzFolder := "" // will be something like binary_fuzzes/bad/skp/badbeef
	currFuzzName := ""
	currFuzzType := ""

	err := gs.AllFilesInDir(storageClient, config.GS.Bucket, baseFolder, func(item *storage.ObjectAttrs) {
		// Assumption, files are sorted alphabetically and have the structure
		// [baseFolder]/[filetype]/[fuzzname]/[fuzzname][suffix]
		// where suffix is one of _debug.dump, _debug.err, _release.dump or _release.err
		name := item.Name
		if name == baseFolder || strings.Count(name, "/") <= 3 {
			return
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
			debugDump = emptyStringOnError(gs.FileContentsFromGS(storageClient, config.GS.Bucket, name))
		} else if strings.HasSuffix(name, "_debug.err") {
			debugErr = emptyStringOnError(gs.FileContentsFromGS(storageClient, config.GS.Bucket, name))
		} else if strings.HasSuffix(name, "_release.dump") {
			releaseDump = emptyStringOnError(gs.FileContentsFromGS(storageClient, config.GS.Bucket, name))
		} else if strings.HasSuffix(name, "_release.err") {
			releaseErr = emptyStringOnError(gs.FileContentsFromGS(storageClient, config.GS.Bucket, name))
		}
	})
	if err != nil {
		return reports, err
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
		glog.Warningf("Ignoring error when fetching file contents: %v", err)
		return ""
	}
	return string(b)
}
