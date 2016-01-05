package gsloader

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/functionnamefinder"
	"go.skia.org/infra/fuzzer/go/fuzz"
	"go.skia.org/infra/fuzzer/go/fuzzcache"
	"go.skia.org/infra/go/gs"
	"google.golang.org/cloud/storage"
)

// LoadFromBoltDB loads the fuzz.FuzzReport from FuzzReportCache associated with the given hash.
// The FuzzReport is first put into the staging fuzz cache, and then into the current.
// If a cache for the commit does not exist, or there are other problems with the retrieval,
// an error is returned.
func LoadFromBoltDB(cache *fuzzcache.FuzzReportCache) error {
	glog.Infof("Looking into cache for revision %s", config.FrontEnd.SkiaVersion.Hash)
	if staging, fuzzes, err := cache.Load(config.FrontEnd.SkiaVersion.Hash); err != nil {
		return fmt.Errorf("Problem decoding existing from bolt db: %s", err)
	} else {
		fuzz.SetStaging(*staging)
		fuzz.StagingToCurrent()
		glog.Infof("Successfully loaded %d binary fuzzes from bolt db cache", len(fuzzes))
	}
	return nil
}

// GSLoader is a struct that handles downloading fuzzes from Google Storage.
type GSLoader struct {
	storageClient *storage.Client
	finder        functionnamefinder.Finder
	Cache         *fuzzcache.FuzzReportCache

	// completedCounter is the number of fuzzes that have been downloaded from GCS, used for logging.
	completedCounter int32
}

// New creates a GSLoader and returns it.
func New(s *storage.Client, f functionnamefinder.Finder, c *fuzzcache.FuzzReportCache) *GSLoader {
	return &GSLoader{
		storageClient: s,
		finder:        f,
		Cache:         c,
	}
}

// SetFinder updates the finder used to look up function names in the fuzz stacktraces
// loaded from GCS by this object.
func (g *GSLoader) SetFinder(f functionnamefinder.Finder) {
	g.finder = f
}

// LoadFreshFromGoogleStorage pulls all fuzzes out of GCS and loads them into memory.
// The "fresh" in the name refers to the fact that all other loaded fuzzes (if any)
// are written over, including in the cache.
// Upon completion, the full results are cached to a boltDB instance and moved from staging
// to the current copy.
func (g *GSLoader) LoadFreshFromGoogleStorage() error {
	revision := config.FrontEnd.SkiaVersion.Hash
	reports, err := g.getBinaryReportsFromGS(fmt.Sprintf("binary_fuzzes/%s/bad/", revision), nil)
	if err != nil {
		return err
	}
	fuzz.ClearStaging()
	binaryFuzzNames := make([]string, 0, len(reports))
	for report := range reports {
		report.DebugStackTrace.LookUpFunctions(g.finder)
		report.ReleaseStackTrace.LookUpFunctions(g.finder)
		fuzz.NewBinaryFuzzFound(report)
		binaryFuzzNames = append(binaryFuzzNames, report.BadBinaryName)
	}
	glog.Infof("%d fuzzes freshly loaded from Google Storage", len(binaryFuzzNames))
	fuzz.StagingToCurrent()

	return g.Cache.Store(fuzz.StagingCopy(), binaryFuzzNames, revision)
}

// LoadBinaryFuzzesFromGoogleStorage pulls all fuzzes out of GCS that are on the given whitelist
// and loads them into memory (as staging).  After loading them, it updates the cache
// and moves them from staging to the current copy.
func (g *GSLoader) LoadBinaryFuzzesFromGoogleStorage(whitelist []string) error {
	revision := config.FrontEnd.SkiaVersion.Hash
	sort.Strings(whitelist)
	reports, err := g.getBinaryReportsFromGS(fmt.Sprintf("binary_fuzzes/%s/bad/", revision), whitelist)
	if err != nil {
		return err
	}
	fuzz.StagingFromCurrent()
	n := 0
	for report := range reports {
		if g.finder != nil {
			report.DebugStackTrace.LookUpFunctions(g.finder)
			report.ReleaseStackTrace.LookUpFunctions(g.finder)
		}
		fuzz.NewBinaryFuzzFound(report)
		n++
	}
	glog.Infof("%d new fuzzes loaded from Google Storage", n)
	fuzz.StagingToCurrent()

	_, oldBinaryFuzzNames, err := g.Cache.Load(revision)
	if err != nil {
		glog.Warningf("Could not read old binary fuzz names from cache.  Continuing...", err)
		oldBinaryFuzzNames = []string{}
	}

	return g.Cache.Store(fuzz.StagingCopy(), append(oldBinaryFuzzNames, whitelist...), revision)
}

// A fuzzPackage contains all the information about a fuzz, mostly the paths to the files that
// need to be downloaded.
type fuzzPackage struct {
	FuzzType        string
	FuzzName        string
	DebugDumpName   string
	DebugErrName    string
	ReleaseDumpName string
	ReleaseErrName  string
}

// getBinaryReportsFromGS pulls all files in baseFolder from the skia-fuzzer bucket and
// groups them by fuzz.  It parses these groups of files into a BinaryFuzzReport and returns
// a channel through whcih all reports generated in this way will be streamed.
// The channel will be closed when all reports are done being sent.
func (g *GSLoader) getBinaryReportsFromGS(baseFolder string, whitelist []string) (<-chan fuzz.BinaryFuzzReport, error) {
	reports := make(chan fuzz.BinaryFuzzReport, 10000)

	fuzzPackages, err := g.fetchFuzzPackages(baseFolder)
	if err != nil {
		close(reports)
		return reports, err
	}

	toDownload := make(chan fuzzPackage, len(fuzzPackages))
	g.completedCounter = 0

	var wg sync.WaitGroup
	for i := 0; i < config.FrontEnd.NumDownloadProcesses; i++ {
		wg.Add(1)
		go g.download(toDownload, reports, &wg)
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

	go func() {
		wg.Wait()
		close(reports)
	}()

	return reports, nil
}

// fetchFuzzPackages scans for all fuzzes in the given folder and returns a
// slice of all of the metadata for each fuzz, as a fuzz package.  It returns
// error if it cannot access Google Storage.
func (g *GSLoader) fetchFuzzPackages(baseFolder string) (fuzzPackages []fuzzPackage, err error) {
	var debugDump, debugErr, releaseDump, releaseErr string
	isInitialized := false
	currFuzzFolder := "" // will be something like binary_fuzzes/bad/skp/badbeef
	currFuzzName := ""
	currFuzzType := ""

	// We cannot simply use common.GetAllFuzzNamesInFolder because that loses FuzzType.
	err = gs.AllFilesInDir(g.storageClient, config.GS.Bucket, baseFolder, func(item *storage.ObjectAttrs) {
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
// the four pieces of the package.  It then parses them into a BinaryFuzzReport and sends
// the binary to the passed in channel.  When there is no more work to be done, this function.
// returns and writes out true to the done channel.
func (g *GSLoader) download(toDownload <-chan fuzzPackage, reports chan<- fuzz.BinaryFuzzReport, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range toDownload {
		debugDump := emptyStringOnError(gs.FileContentsFromGS(g.storageClient, config.GS.Bucket, job.DebugDumpName))
		debugErr := emptyStringOnError(gs.FileContentsFromGS(g.storageClient, config.GS.Bucket, job.DebugErrName))
		releaseDump := emptyStringOnError(gs.FileContentsFromGS(g.storageClient, config.GS.Bucket, job.ReleaseDumpName))
		releaseErr := emptyStringOnError(gs.FileContentsFromGS(g.storageClient, config.GS.Bucket, job.ReleaseErrName))
		reports <- fuzz.ParseBinaryReport(job.FuzzType, job.FuzzName, debugDump, debugErr, releaseDump, releaseErr)
		atomic.AddInt32(&g.completedCounter, 1)
		if g.completedCounter%100 == 0 {
			glog.Infof("%d fuzzes downloaded", g.completedCounter)
		}
	}
}
