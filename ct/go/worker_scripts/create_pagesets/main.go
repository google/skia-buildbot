// Application that creates pagesets on a CT worker and uploads it to Google
// Storage.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

var (
	startRange  = flag.Int("start_range", 1, "The number this worker will start creating page sets from.")
	num         = flag.Int("num", 100, "The total number of pagesets to process starting from the start_range.")
	pagesetType = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets to create from the CSV list in util.PagesetTypeToInfo.")
)

func createPagesets() error {
	ctx := context.Background()
	httpClient, err := worker_common.Init(ctx, false /* useDepotTools */)
	if err != nil {
		return skerr.Wrap(err)
	}
	if !*worker_common.Local {
		defer util.CleanTmpDir()
	}
	defer util.TimeTrack(time.Now(), "Creating Pagesets")
	defer sklog.Flush()

	// Delete and remake the local pagesets directory.
	pathToPagesets := filepath.Join(util.PagesetsDir, *pagesetType)
	skutil.RemoveAll(pathToPagesets)
	util.MkdirAll(pathToPagesets, 0700)
	defer skutil.RemoveAll(pathToPagesets)

	// Get info about the specified pageset type.
	pagesetTypeInfo := util.PagesetTypeToInfo[*pagesetType]
	csvSource := pagesetTypeInfo.CSVSource
	numPages := pagesetTypeInfo.NumPages
	userAgent := pagesetTypeInfo.UserAgent

	// Download the CSV file from Google Storage.
	gs, err := util.NewGcsUtil(httpClient)
	if err != nil {
		return err
	}
	csvFile := filepath.Join(util.StorageDir, filepath.Base(csvSource))
	if err := gs.DownloadRemoteFile(csvSource, csvFile); err != nil {
		return fmt.Errorf("Could not download %s: %s", csvSource, err)
	}
	defer skutil.Remove(csvFile)

	// Figure out the endRange of this worker.
	endRange := skutil.MinInt(*startRange+*num-1, numPages)

	// Construct path to the create_page_set.py python script.
	pathToPyFiles, err := util.GetPathToPyFiles(*worker_common.Local)
	if err != nil {
		return fmt.Errorf("Could not get path to py files: %s", err)
	}
	createPageSetScript := filepath.Join(pathToPyFiles, "create_page_set.py")

	// Execute the create_page_set.py python script.
	timeoutSecs := util.PagesetTypeToInfo[*pagesetType].CreatePagesetsTimeoutSecs
	args := []string{
		createPageSetScript,
		"-s", strconv.Itoa(*startRange),
		"-e", strconv.Itoa(endRange),
		"-c", csvFile,
		"-p", *pagesetType,
		"-u", userAgent,
		"-o", pathToPagesets,
	}
	if err := util.ExecuteCmd(ctx, "python", args, []string{}, time.Duration(timeoutSecs)*time.Second, nil, nil); err != nil {
		return err
	}

	// Check to see if there is anything in the pathToPagesets dir.
	pagesetsEmpty, err := skutil.IsDirEmpty(pathToPagesets)
	if err != nil {
		return err
	}
	if pagesetsEmpty {
		return fmt.Errorf("Could not create any page sets in %s", pathToPagesets)
	}

	// Upload all page sets to Google Storage.
	if err := gs.UploadSwarmingArtifacts(util.PAGESETS_DIR_NAME, *pagesetType); err != nil {
		return err
	}

	return nil
}

func main() {
	retCode := 0
	if err := createPagesets(); err != nil {
		sklog.Errorf("Error while creating pagesets: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
