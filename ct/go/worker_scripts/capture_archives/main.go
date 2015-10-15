// Application that captures webpage archives on a CT worker and uploads it to
// Google Storage.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

var (
	workerNum     = flag.Int("worker_num", 1, "The number of this CT worker. It will be in the {1..100} range.")
	pagesetType   = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets to create from the Alexa CSV list. Eg: 10k, Mobile10k, All.")
	chromiumBuild = flag.String("chromium_build", "", "The chromium build to use for this capture_archives run.")
)

func main() {
	defer common.LogPanic()
	worker_common.Init()
	defer util.TimeTrack(time.Now(), "Capturing Archives")
	defer glog.Flush()

	// Create the task file so that the master knows this worker is still busy.
	skutil.LogErr(util.CreateTaskFile(util.ACTIVITY_CAPTURING_ARCHIVES))
	defer util.DeleteTaskFile(util.ACTIVITY_CAPTURING_ARCHIVES)

	if *chromiumBuild == "" {
		glog.Error("Must specify --chromium_build")
		return
	}

	// Reset the local chromium checkout.
	if err := util.ResetCheckout(util.ChromiumSrcDir); err != nil {
		glog.Errorf("Could not reset %s: %s", util.ChromiumSrcDir, err)
		return
	}
	// Sync the local chromium checkout.
	if err := util.SyncDir(util.ChromiumSrcDir); err != nil {
		glog.Errorf("Could not gclient sync %s: %s", util.ChromiumSrcDir, err)
		return
	}

	// Delete and remake the local webpage archives directory.
	pathToArchives := filepath.Join(util.WebArchivesDir, *pagesetType)
	skutil.RemoveAll(pathToArchives)
	skutil.MkdirAll(pathToArchives, 0700)

	// Instantiate GsUtil object.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Error(err)
		return
	}

	// Download the specified chromium build if it does not exist locally.
	if err := gs.DownloadChromiumBuild(*chromiumBuild); err != nil {
		glog.Error(err)
		return
	}

	// Download pagesets if they do not exist locally.
	if err := gs.DownloadWorkerArtifacts(util.PAGESETS_DIR_NAME, *pagesetType, *workerNum); err != nil {
		glog.Error(err)
		return
	}

	pathToPagesets := filepath.Join(util.PagesetsDir, *pagesetType)
	chromiumBinary := filepath.Join(util.ChromiumBuildsDir, *chromiumBuild, util.BINARY_CHROME)
	recordWprBinary := filepath.Join(util.TelemetryBinariesDir, util.BINARY_RECORD_WPR)
	timeoutSecs := util.PagesetTypeToInfo[*pagesetType].CaptureArchivesTimeoutSecs
	// Loop through all pagesets.
	fileInfos, err := ioutil.ReadDir(pathToPagesets)
	if err != nil {
		glog.Errorf("Unable to read the pagesets dir %s: %s", pathToPagesets, err)
		return
	}
	glog.Infof("The %s fileInfos are: %s", len(fileInfos), fileInfos)
	for _, fileInfo := range fileInfos {
		pagesetBaseName := filepath.Base(fileInfo.Name())
		if pagesetBaseName == util.TIMESTAMP_FILE_NAME || filepath.Ext(pagesetBaseName) == ".pyc" {
			// Ignore timestamp files and .pyc files.
			continue
		}

		// Read the pageset.
		pagesetPath := filepath.Join(pathToPagesets, fileInfo.Name())
		decodedPageset, err := util.ReadPageset(pagesetPath)
		if err != nil {
			glog.Errorf("Could not read %s: %s", pagesetPath, err)
			return
		}

		glog.Infof("===== Processing %s =====", pagesetPath)
		args := []string{
			util.CAPTURE_ARCHIVES_DEFAULT_CT_BENCHMARK,
			"--extra-browser-args=--disable-setuid-sandbox",
			"--browser=exact",
			"--browser-executable=" + chromiumBinary,
			"--user-agent=" + decodedPageset.UserAgent,
			"--urls-list=" + decodedPageset.UrlsList,
			"--archive-data-file=" + decodedPageset.ArchiveDataFile,
		}
		env := []string{
			fmt.Sprintf("PYTHONPATH=%s:$PYTHONPATH", pathToPagesets),
			"DISPLAY=:0",
		}
		skutil.LogErr(util.ExecuteCmd(recordWprBinary, args, env, time.Duration(timeoutSecs)*time.Second, nil, nil))
	}

	// Write timestamp to the webpage archives dir.
	skutil.LogErr(util.CreateTimestampFile(pathToArchives))

	// Upload webpage archives dir to Google Storage.
	if err := gs.UploadWorkerArtifacts(util.WEB_ARCHIVES_DIR_NAME, *pagesetType, *workerNum); err != nil {
		glog.Error(err)
		return
	}
}
