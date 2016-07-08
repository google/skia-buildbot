// Application that creates pagesets on a CT worker and uploads it to Google
// Storage.
package main

import (
	"flag"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/skia-dev/glog"

	"strconv"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

var (
	startRange  = flag.Int("start_range", 1, "The number this worker will start creating page sets from.")
	num         = flag.Int("num", 100, "The total number of pagesets to process starting from the start_range.")
	pagesetType = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets to create from the CSV list in util.PagesetTypeToInfo.")
)

func main() {
	defer common.LogPanic()
	worker_common.Init()
	defer util.TimeTrack(time.Now(), "Creating Pagesets")
	defer glog.Flush()

	// Delete and remake the local pagesets directory.
	pathToPagesets := filepath.Join(util.PagesetsDir, *pagesetType)
	skutil.RemoveAll(pathToPagesets)
	skutil.MkdirAll(pathToPagesets, 0700)
	defer skutil.RemoveAll(pathToPagesets)

	// Get info about the specified pageset type.
	pagesetTypeInfo := util.PagesetTypeToInfo[*pagesetType]
	csvSource := pagesetTypeInfo.CSVSource
	numPages := pagesetTypeInfo.NumPages
	userAgent := pagesetTypeInfo.UserAgent

	// Download the CSV file from Google Storage.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Fatal(err)
	}
	csvFile := filepath.Join(util.StorageDir, filepath.Base(csvSource))
	if err := gs.DownloadRemoteFile(csvSource, csvFile); err != nil {
		glog.Fatalf("Could not download %s: %s", csvSource, err)
	}
	defer skutil.Remove(csvFile)

	// Figure out the endRange of this worker.
	endRange := skutil.MinInt(*startRange+*num-1, numPages)

	// Construct path to the create_page_set.py python script.
	pathToPyFiles := util.GetPathToPyFiles(!*worker_common.Local)
	createPageSetScript := filepath.Join(pathToPyFiles, "create_page_set.py")

	// Execute the create_page_set.py python script.
	timeoutSecs := util.PagesetTypeToInfo[*pagesetType].CreatePagesetsTimeoutSecs
	for currNum := *startRange; currNum <= endRange; currNum++ {
		destDir := path.Join(pathToPagesets, strconv.Itoa(currNum))
		if err := os.MkdirAll(destDir, 0700); err != nil {
			glog.Fatal(err)
		}
		args := []string{
			createPageSetScript,
			"-s", strconv.Itoa(currNum),
			"-c", csvFile,
			"-p", *pagesetType,
			"-u", userAgent,
			"-o", destDir,
		}
		if err := util.ExecuteCmd("python", args, []string{}, time.Duration(timeoutSecs)*time.Second, nil, nil); err != nil {
			glog.Fatal(err)
		}
	}

	// Check to see if there is anything in the pathToPagesets dir.
	pagesetsEmpty, err := skutil.IsDirEmpty(pathToPagesets)
	if err != nil {
		glog.Fatal(err)
	}
	if pagesetsEmpty {
		glog.Fatalf("Could not create any page sets in %s", pathToPagesets)
	}

	// Upload all page sets to Google Storage.
	if err := gs.UploadSwarmingArtifacts(util.PAGESETS_DIR_NAME, *pagesetType); err != nil {
		glog.Fatal(err)
	}
}
