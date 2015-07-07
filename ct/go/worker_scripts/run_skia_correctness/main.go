// Application that runs Skia correctness over the specified SKP repository.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

var (
	workerNum          = flag.Int("worker_num", 1, "The number of this CT worker. It will be in the {1..100} range.")
	pagesetType        = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets to create from the Alexa CSV list. Eg: 10k, Mobile10k, All.")
	chromiumBuild      = flag.String("chromium_build", "", "The chromium build that was used to create the SKPs we would like to run skia correctness against.")
	runID              = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	renderPicturesArgs = flag.String("render_pictures_args", "", "The arguments to pass to render_pictures binary.")
	gpuNoPatchRun      = flag.Bool("gpu_nopatch_run", false, "Whether to build with GPU for the nopatch run.")
	gpuWithPatchRun    = flag.Bool("gpu_withpatch_run", false, "Whether to build with GPU for the withpatch run.")
)

func runRenderPictures(localSkpsDir, localOutputDir, remoteOutputDir string, runGpu bool) error {
	picturesArgs := *renderPicturesArgs
	if runGpu {
		glog.Info("Run with GPU has been specified. Using --config gpu.")
		reg, _ := regexp.Compile("--config [a-zA-Z0-9]+")
		picturesArgs = reg.ReplaceAllString(picturesArgs, "--config gpu")
	}
	skutil.MkdirAll(localOutputDir, 0700)
	args := []string{
		"-r", localSkpsDir,
		"-w", localOutputDir,
		"--writeJsonSummaryPath", filepath.Join(localOutputDir, "summary.json"),
		"--imageBaseGSUrl", remoteOutputDir,
	}
	for _, picturesArg := range strings.Split(picturesArgs, " ") {
		args = append(args, picturesArg)
	}
	if err := util.ExecuteCmd(filepath.Join(util.SkiaTreeDir, "out", "Release", util.BINARY_RENDER_PICTURES), args, []string{"DISPLAY=:0"}, 15*time.Minute, nil, nil); err != nil {
		return fmt.Errorf("Failure when running render_pictures: %s", err)
	}
	return nil
}

func main() {
	common.Init()
	defer util.TimeTrack(time.Now(), "Running Skia Correctness")
	defer glog.Flush()

	if *chromiumBuild == "" {
		glog.Error("Must specify --chromium_build")
		return
	}
	if *runID == "" {
		glog.Error("Must specify --run_id")
		return
	}

	// Create the task file so that the master knows this worker is still busy.
	skutil.LogErr(util.CreateTaskFile(util.ACTIVITY_RUNNING_SKIA_CORRECTNESS))
	defer util.DeleteTaskFile(util.ACTIVITY_RUNNING_SKIA_CORRECTNESS)

	// Establish output paths.
	localOutputDir := filepath.Join(util.StorageDir, util.SkiaCorrectnessRunsDir, *runID)
	skutil.RemoveAll(filepath.Join(util.StorageDir, util.SkiaCorrectnessRunsDir))
	skutil.MkdirAll(localOutputDir, 0700)
	defer skutil.RemoveAll(localOutputDir)
	remoteOutputDir := filepath.Join(util.SkiaCorrectnessRunsDir, *runID, fmt.Sprintf("slave%d", *workerNum))

	// Instantiate GsUtil object.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Error(err)
		return
	}

	// Download SKPs if they do not exist locally.
	if err := gs.DownloadWorkerArtifacts(util.SKPS_DIR_NAME, filepath.Join(*pagesetType, *chromiumBuild), *workerNum); err != nil {
		glog.Error(err)
		return
	}
	localSkpsDir := filepath.Join(util.SkpsDir, *pagesetType, *chromiumBuild)

	// Download the Skia patch for this run from Google storage.
	patchName := *runID + ".patch"
	patchLocalPath := filepath.Join(os.TempDir(), patchName)
	remoteDir := filepath.Join(util.SkiaCorrectnessRunsDir, *runID)
	patchRemotePath := filepath.Join(remoteDir, "patches", patchName)
	respBody, err := gs.GetRemoteFileContents(patchRemotePath)
	if err != nil {
		glog.Errorf("Could not fetch %s: %s", patchRemotePath, err)
		return
	}
	defer skutil.Close(respBody)
	out, err := os.Create(patchLocalPath)
	if err != nil {
		glog.Errorf("Unable to create file %s: %s", patchLocalPath, err)
		return
	}
	defer skutil.Close(out)
	defer skutil.Remove(patchLocalPath)
	if _, err = io.Copy(out, respBody); err != nil {
		glog.Error(err)
		return
	}

	// Apply the patch to a clean checkout and run render_pictures.
	// Reset Skia tree.
	skutil.LogErr(util.ResetCheckout(util.SkiaTreeDir))
	// Sync Skia tree.
	skutil.LogErr(util.SyncDir(util.SkiaTreeDir))
	// Apply Skia patch.
	file, _ := os.Open(patchLocalPath)
	fileInfo, _ := file.Stat()
	// It is a valid patch only if it is more than 10 bytes.
	if fileInfo.Size() > 10 {
		glog.Info("Attempting to apply %s to %s", patchLocalPath, util.SkiaTreeDir)
		if err := util.ApplyPatch(patchLocalPath, util.SkiaTreeDir); err != nil {
			glog.Errorf("Could not apply patch %s to %s: %s", patchLocalPath, util.SkiaTreeDir, err)
			return
		}
		glog.Info("Patch successfully applied")
	} else {
		glog.Info("Patch is empty or invalid. Skipping the patch.")
	}
	// Build tools.
	skutil.LogErr(util.BuildSkiaTools())
	// Run render_pictures.
	if err := runRenderPictures(localSkpsDir, filepath.Join(localOutputDir, "withpatch"), filepath.Join(remoteOutputDir, "withpatch"), *gpuWithPatchRun); err != nil {
		glog.Errorf("Error while running withpatch render_pictures: %s", err)
		return
	}

	// Remove the patch and run render_pictures.
	// Reset Skia tree.
	skutil.LogErr(util.ResetCheckout(util.SkiaTreeDir))
	// Build tools.
	skutil.LogErr(util.BuildSkiaTools())
	// Run render_pictures.
	if err := runRenderPictures(localSkpsDir, filepath.Join(localOutputDir, "nopatch"), filepath.Join(remoteOutputDir, "nopatch"), *gpuNoPatchRun); err != nil {
		glog.Errorf("Error while running nopatch render_pictures: %s", err)
		return
	}

	// Comparing pictures and saving differences in JSON output file.
	jsonSummaryDir := filepath.Join(localOutputDir, "json_summary")
	skutil.MkdirAll(jsonSummaryDir, 0700)
	jsonSummaryPath := filepath.Join(jsonSummaryDir, fmt.Sprintf("slave%d", *workerNum)+".json")
	// Construct path to the write_json_summary python script.
	_, currentFile, _, _ := runtime.Caller(0)
	pathToPyFiles := filepath.Join(
		filepath.Dir((filepath.Dir(filepath.Dir(filepath.Dir(currentFile))))),
		"py")
	summaryArgs := []string{
		filepath.Join(pathToPyFiles, "write_json_summary.py"),
		"--img_root=" + localOutputDir,
		"--withpatch_json=" + filepath.Join(localOutputDir, "withpatch", "summary.json"),
		"--withpatch_images_base_url=file://" + filepath.Join(localOutputDir, "withpatch"),
		"--nopatch_json=" + filepath.Join(localOutputDir, "nopatch", "summary.json"),
		"--nopatch_images_base_url=file://" + filepath.Join(localOutputDir, "nopatch"),
		"--output_file_path=" + jsonSummaryPath,
		"--gs_output_dir=gs://" + filepath.Join(util.GS_BUCKET_NAME, remoteDir),
		"--gs_skp_dir=gs://" + filepath.Join(util.GS_BUCKET_NAME, util.SKPS_DIR_NAME, *pagesetType, *chromiumBuild, fmt.Sprintf("slave%d", *workerNum)),
		"--slave_num=" + strconv.Itoa(*workerNum),
	}
	if err := util.ExecuteCmd("python", summaryArgs, []string{}, 15*time.Minute, nil, nil); err != nil {
		glog.Error(err)
		return
	}

	// Upload artifacts to Google Storage.
	// Get list of failed file names and upload only those to Google Storage.
	// Read the JSON file.
	summaryFile, err := ioutil.ReadFile(jsonSummaryPath)
	if err != nil {
		glog.Errorf("Unable to read %s: %s", jsonSummaryPath, err)
	}
	var jsontype map[string]interface{}
	skutil.LogErr(json.Unmarshal(summaryFile, &jsontype))
	if jsontype[fmt.Sprintf("slave%d", *workerNum)] != nil {
		failedFiles := jsontype[fmt.Sprintf("slave%d", *workerNum)].(map[string]interface{})["failedFiles"].([]interface{})
		for i := range failedFiles {
			failedFile := failedFiles[i].(map[string]interface{})["fileName"].(string)
			// TODO(rmistry): Use goroutines to do the below in parallel.
			skutil.LogErr(
				gs.UploadFile(failedFile, filepath.Join(localOutputDir, "withpatch"), filepath.Join(remoteDir, fmt.Sprintf("slave%d", *workerNum), "withpatch-images")))
			skutil.LogErr(
				gs.UploadFile(failedFile, filepath.Join(localOutputDir, "nopatch"), filepath.Join(remoteDir, fmt.Sprintf("slave%d", *workerNum), "nopatch-images")))
		}
		// Copy the diffs and whitediffs to Google Storage.
		skutil.LogErr(
			gs.UploadDir(filepath.Join(localOutputDir, "diffs"), filepath.Join(remoteDir, fmt.Sprintf("slave%d", *workerNum), "diffs"), true))
		skutil.LogErr(
			gs.UploadDir(filepath.Join(localOutputDir, "whitediffs"), filepath.Join(remoteDir, fmt.Sprintf("slave%d", *workerNum), "whitediffs"), true))
	}
	// Upload the summary file.
	skutil.LogErr(
		gs.UploadFile(fmt.Sprintf("slave%d", *workerNum)+".json", jsonSummaryDir, filepath.Join(remoteDir, fmt.Sprintf("slave%d", *workerNum))))
}
