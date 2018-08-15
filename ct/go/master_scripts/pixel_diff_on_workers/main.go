// pixel_diff_on_workers is an application that captures screenshots of the
// specified patchset type on all CT workers and uploads the results to Google
// Storage. The requester is emailed when the task is done.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"go.skia.org/infra/ct/go/ctfe/pixel_diff"
	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	MAX_PAGES_PER_SWARMING_BOT = 50

	PIXEL_DIFF_RESULTS_LINK_TEMPLATE = "https://ctpixeldiff.skia.org/load?runID=%s"
)

var (
	emails                    = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	description               = flag.String("description", "", "The description of the run as entered by the requester.")
	taskID                    = flag.Int64("task_id", -1, "The key of the CT task in CTFE. The task will be updated when it is started and also when it completes.")
	pagesetType               = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	benchmarkExtraArgs        = flag.String("benchmark_extra_args", "", "The extra arguments that are passed to the specified benchmark.")
	browserExtraArgsNoPatch   = flag.String("browser_extra_args_nopatch", "", "The extra arguments that are passed to the browser while running the benchmark for the nopatch case.")
	browserExtraArgsWithPatch = flag.String("browser_extra_args_withpatch", "", "The extra arguments that are passed to the browser while running the benchmark for the withpatch case.")
	runOnGCE                  = flag.Bool("run_on_gce", true, "Run on Linux GCE instances.")
	runID                     = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")

	taskCompletedSuccessfully = false

	pixelDiffResultsLink = ""
	skiaPatchLink        = ""
	chromiumPatchLink    = ""
	customWebpagesLink   = ""
)

func sendEmail(recipients []string) {
	// Send completion email.
	emailSubject := fmt.Sprintf("Pixel diff cluster telemetry task has completed (#%d)", *taskID)
	failureHtml := ""
	viewActionMarkup := ""
	var err error

	if taskCompletedSuccessfully {
		if viewActionMarkup, err = email.GetViewActionMarkup(pixelDiffResultsLink, "View Results", "Direct link to the HTML results"); err != nil {
			sklog.Errorf("Failed to get view action markup: %s", err)
			return
		}
	} else {
		emailSubject += " with failures"
		failureHtml = util.GetFailureEmailHtml(*runID)
		if viewActionMarkup, err = email.GetViewActionMarkup(fmt.Sprintf(util.SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, *runID), "View Failure", "Direct link to the swarming logs"); err != nil {
			sklog.Errorf("Failed to get view action markup: %s", err)
			return
		}
	}
	bodyTemplate := `
	The pixel diff task on %s pageset has completed. %s.<br/>
	Run description: %s<br/>
	%s
	<br/>
	The results of the run are available <a href='%s'>here</a>.<br/>
	Note: Results will take some time to be processed and thus might not be immediately available.<br/>
	<br/>
	The patch(es) you specified are here:
	<a href='%s'>chromium</a>/<a href='%s'>skia</a>
	<br/>
	Custom webpages (if specified) are <a href='%s'>here</a>.
	<br/><br/>
	You can schedule more runs <a href='%s'>here</a>.
	<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *pagesetType, util.GetSwarmingLogsLink(*runID), *description, failureHtml, pixelDiffResultsLink, chromiumPatchLink, skiaPatchLink, customWebpagesLink, frontend.PixelDiffTasksWebapp)
	if err := util.SendEmailWithMarkup(recipients, emailSubject, emailBody, viewActionMarkup); err != nil {
		sklog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateWebappTask() {
	vars := pixel_diff.UpdateVars{}
	vars.Id = *taskID
	vars.SetCompleted(taskCompletedSuccessfully)
	vars.Results = pixelDiffResultsLink
	skutil.LogErr(frontend.UpdateWebappTaskV2(&vars))
}

func main() {
	master_common.Init("pixel_diff")

	ctx := context.Background()

	// Send start email.
	emailsArr := util.ParseEmails(*emails)
	emailsArr = append(emailsArr, util.CtAdmins...)
	if len(emailsArr) == 0 {
		sklog.Error("At least one email address must be specified")
		return
	}
	skutil.LogErr(frontend.UpdateWebappTaskSetStarted(&pixel_diff.UpdateVars{}, *taskID, *runID))
	skutil.LogErr(util.SendTaskStartEmail(*taskID, emailsArr, "Pixel diff", *runID, *description))

	// Ensure webapp is updated and completion email is sent even if task
	// fails.
	defer updateWebappTask()
	defer sendEmail(emailsArr)

	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running pixel diff task on workers")
	defer sklog.Flush()

	if *pagesetType == "" {
		sklog.Error("Must specify --pageset_type")
		return
	}
	if *description == "" {
		sklog.Error("Must specify --description")
		return
	}
	if *runID == "" {
		sklog.Error("Must specify --run_id")
		return
	}

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		sklog.Errorf("GcsUtil instantiation failed: %s", err)
		return
	}
	remoteOutputDir := filepath.Join(util.BenchmarkRunsDir, *runID)

	// Copy the patches and custom webpages to Google Storage.
	skiaPatchName := *runID + ".skia.patch"
	chromiumPatchName := *runID + ".chromium.patch"
	customWebpagesName := *runID + ".custom_webpages.csv"
	for _, patchName := range []string{skiaPatchName, chromiumPatchName, customWebpagesName} {
		if err := gs.UploadFile(patchName, os.TempDir(), remoteOutputDir); err != nil {
			sklog.Errorf("Could not upload %s to %s: %s", patchName, remoteOutputDir, err)
			return
		}
	}
	skiaPatchLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, skiaPatchName)
	chromiumPatchLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, chromiumPatchName)
	customWebpagesLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, remoteOutputDir, customWebpagesName)

	// Find which chromium hash the workers should use.
	chromiumHash, err := util.GetChromiumHash(ctx)
	if err != nil {
		sklog.Error("Could not find the latest chromium hash")
		return
	}

	// Trigger both the build repo and isolate telemetry tasks in parallel.
	group := skutil.NewNamedErrGroup()
	var chromiumBuildNoPatch, chromiumBuildWithPatch string
	group.Go("build chromium", func() error {
		// Check if the patches have any content to decide if we need one or two chromium builds.
		localPatches := []string{filepath.Join(os.TempDir(), chromiumPatchName), filepath.Join(os.TempDir(), skiaPatchName)}
		remotePatches := []string{filepath.Join(remoteOutputDir, chromiumPatchName), filepath.Join(remoteOutputDir, skiaPatchName)}
		if util.PatchesAreEmpty(localPatches) {
			// Create only one chromium build.
			chromiumBuilds, err := util.TriggerBuildRepoSwarmingTask(
				ctx, "build_chromium", *runID, "chromium", util.PLATFORM_LINUX, []string{chromiumHash}, remotePatches, []string{},
				/*singlebuild*/ true, *master_common.Local, 3*time.Hour, 1*time.Hour)
			if err != nil {
				return sklog.FmtErrorf("Error encountered when swarming build repo task: %s", err)
			}
			if len(chromiumBuilds) != 1 {
				return sklog.FmtErrorf("Expected 1 build but instead got %d: %v.", len(chromiumBuilds), chromiumBuilds)
			}
			chromiumBuildNoPatch = chromiumBuilds[0]
			chromiumBuildWithPatch = chromiumBuilds[0]

		} else {
			// Create the two required chromium builds (with patch and without the patch).
			chromiumBuilds, err := util.TriggerBuildRepoSwarmingTask(
				ctx, "build_chromium", *runID, "chromium", util.PLATFORM_LINUX, []string{chromiumHash}, remotePatches, []string{},
				/*singlebuild*/ false, *master_common.Local, 3*time.Hour, 1*time.Hour)
			if err != nil {
				return sklog.FmtErrorf("Error encountered when swarming build repo task: %s", err)
			}
			if len(chromiumBuilds) != 2 {
				return sklog.FmtErrorf("Expected 2 builds but instead got %d: %v.", len(chromiumBuilds), chromiumBuilds)
			}
			chromiumBuildNoPatch = chromiumBuilds[0]
			chromiumBuildWithPatch = chromiumBuilds[1]
		}
		return nil
	})

	// Isolate telemetry.
	isolateDeps := []string{}
	group.Go("isolate telemetry", func() error {
		telemetryIsolatePatches := []string{filepath.Join(remoteOutputDir, chromiumPatchName)}
		telemetryHash, err := util.TriggerIsolateTelemetrySwarmingTask(ctx, "isolate_telemetry", *runID, chromiumHash, telemetryIsolatePatches, 1*time.Hour, 1*time.Hour, *master_common.Local)
		if err != nil {
			return sklog.FmtErrorf("Error encountered when swarming isolate telemetry task: %s", err)
		}
		if telemetryHash == "" {
			return sklog.FmtErrorf("Found empty telemetry hash!")
		}
		isolateDeps = append(isolateDeps, telemetryHash)
		return nil
	})

	// Wait for chromium build task and isolate telemetry task to complete.
	if err := group.Wait(); err != nil {
		sklog.Error(err)
		return
	}

	// Clean up the chromium builds from Google storage after the run completes.
	defer gs.DeleteRemoteDirLogErr(filepath.Join(util.CHROMIUM_BUILDS_DIR_NAME, chromiumBuildNoPatch))
	defer gs.DeleteRemoteDirLogErr(filepath.Join(util.CHROMIUM_BUILDS_DIR_NAME, chromiumBuildWithPatch))

	// Archive, trigger and collect swarming tasks.
	isolateExtraArgs := map[string]string{
		"CHROMIUM_BUILD_NOPATCH":       chromiumBuildNoPatch,
		"CHROMIUM_BUILD_WITHPATCH":     chromiumBuildWithPatch,
		"RUN_ID":                       *runID,
		"BENCHMARK_ARGS":               *benchmarkExtraArgs,
		"BROWSER_EXTRA_ARGS_NOPATCH":   *browserExtraArgsNoPatch,
		"BROWSER_EXTRA_ARGS_WITHPATCH": *browserExtraArgsWithPatch,
	}
	customWebPagesFilePath := filepath.Join(os.TempDir(), customWebpagesName)
	numPages, err := util.GetNumPages(*pagesetType, customWebPagesFilePath)
	if err != nil {
		sklog.Errorf("Error encountered when calculating number of pages: %s", err)
		return
	}
	if _, err := util.TriggerSwarmingTask(ctx, *pagesetType, "pixel_diff", util.PIXEL_DIFF_ISOLATE, *runID, 3*time.Hour, 1*time.Hour, util.USER_TASKS_PRIORITY, MAX_PAGES_PER_SWARMING_BOT, numPages, isolateExtraArgs, *runOnGCE, *master_common.Local, 1, isolateDeps); err != nil {
		sklog.Errorf("Error encountered when swarming tasks: %s", err)
		return
	}

	// Create metadata file at the top level that lists the total number of webpages processed in both nopatch and withpatch directories.
	if err := createAndUploadMetadataFile(gs); err != nil {
		sklog.Errorf("Could not create and upload metadata file: %s", err)
		return
	}

	pixelDiffResultsLink = fmt.Sprintf(PIXEL_DIFF_RESULTS_LINK_TEMPLATE, *runID)
	taskCompletedSuccessfully = true
}

type Metadata struct {
	RunID                string `json:"run_id"`
	NoPatchImagesCount   int    `json:"nopatch_images_count"`
	WithPatchImagesCount int    `json:"withpatch_images_count"`
	Description          string `json:"description"`
}

func createAndUploadMetadataFile(gs *util.GcsUtil) error {
	baseRemoteDir, err := util.GetBasePixelDiffRemoteDir(*runID)
	if err != nil {
		return fmt.Errorf("Error encountered when calculating remote base dir: %s", err)
	}
	noPatchRemoteDir := filepath.Join(baseRemoteDir, "nopatch")
	totalNoPatchWebpages, err := gs.GetRemoteDirCount(noPatchRemoteDir)
	if err != nil {
		return fmt.Errorf("Could not find any content in %s: %s", noPatchRemoteDir, err)
	}
	withPatchRemoteDir := filepath.Join(baseRemoteDir, "withpatch")
	totalWithPatchWebpages, err := gs.GetRemoteDirCount(withPatchRemoteDir)
	if err != nil {
		return fmt.Errorf("Could not find any content in %s: %s", withPatchRemoteDir, err)
	}
	metadata := Metadata{
		RunID:                *runID,
		NoPatchImagesCount:   totalNoPatchWebpages,
		WithPatchImagesCount: totalWithPatchWebpages,
		Description:          *description,
	}
	m, err := json.Marshal(&metadata)
	if err != nil {
		return fmt.Errorf("Could not marshall %s to json: %s", m, err)
	}
	localMetadataFileName := *runID + ".metadata.json"
	localMetadataFilePath := filepath.Join(os.TempDir(), localMetadataFileName)
	if err := ioutil.WriteFile(localMetadataFilePath, m, os.ModePerm); err != nil {
		return fmt.Errorf("Could not write to %s: %s", localMetadataFilePath, err)
	}
	defer skutil.Remove(localMetadataFilePath)
	if err := gs.UploadFile(localMetadataFileName, os.TempDir(), baseRemoteDir); err != nil {
		return fmt.Errorf("Could not upload %s to %s: %s", localMetadataFileName, baseRemoteDir, err)
	}
	return nil
}
