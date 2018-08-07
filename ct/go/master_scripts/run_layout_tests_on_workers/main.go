// run_layout_tests_on_workers is an application that run the layout tests
// on many CT workers and uploads the results to Google Storage. The requester
// is emailed when the task is done.
package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"time"

	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/sklog"
	//skutil "go.skia.org/infra/go/util"
)

const (
	NUM_SWARMING_BOTS_TO_RUN_ON = 1 // TODO(rmistry): Make this 500? or 1000? all based on how long it takes to run.
)

var (
	emails = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	runID  = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")

	taskCompletedSuccessfully = false
	outputLink                = ""
)

func sendEmail(recipients []string) {
	// Send completion email.
	emailSubject := fmt.Sprintf("Run layout tests cluster telemetry task has completed (#%d)", *runID)
	failureHtml := ""
	viewActionMarkup := ""
	var err error

	if taskCompletedSuccessfully {
		if viewActionMarkup, err = email.GetViewActionMarkup(outputLink, "View Results", "Direct link to the results"); err != nil {
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
	The layout tests task has completed. %s.<br/>
	%s
	<br/>
	The results of the run are available <a href='%s'>here</a>.<br/>
	<br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, util.GetSwarmingLogsLink(*runID), failureHtml, outputLink)
	if err := util.SendEmailWithMarkup(recipients, emailSubject, emailBody, viewActionMarkup); err != nil {
		sklog.Errorf("Error while sending email: %s", err)
		return
	}
}

func main() {
	master_common.Init("run_layout_tests_on_workers")

	ctx := context.Background()

	// Send start email.
	emailsArr := util.ParseEmails(*emails)
	// emailsArr = append(emailsArr, util.CtAdmins...) // REMOVED util.CtAdmins...
	if len(emailsArr) == 0 {
		sklog.Error("At least one email address must be specified")
		return
	}
	//skutil.LogErr(util.SendTaskStartEmail(1 /* dummy number */, emailsArr, "Layout test", *runID, ""))

	// Ensure completion email is sent even if task fails.
	//defer sendEmail(emailsArr)

	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running layout tests on workers")
	defer sklog.Flush()

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
	remoteOutputDir := path.Join(util.SWARMING_DIR_NAME, util.LayoutTestRunsDir, *runID)
	fmt.Println(gs)
	fmt.Println(remoteOutputDir)

	// Find which chromium hash the workers should use.
	chromiumHash, err := util.GetChromiumHash(ctx)
	if err != nil {
		sklog.Error("Could not find the latest chromium hash")
		return
	}

	// Trigger the isolate layout tests task.
	//layoutTestsHash, err := util.TriggerIsolateLayoutTestsSwarmingTask(ctx, "isolate_layout_tests", *runID, chromiumHash, 1*time.Hour, 1*time.Hour)
	//if err != nil {
	//	sklog.Errorf("Error encountered when swarming isolate layout tests task: %s", err)
	//	return
	//}
	//if layoutTestsHash == "" {
	//	sklog.Errorf("Found empty telemetry hash!")
	//	return
	//}
	layoutTestsHash := "e41893db3547a0fdd9b071705723068a4eb2fd01"
	fmt.Println(chromiumHash)
	fmt.Println("HASH IS THIS!!!!!!")
	fmt.Println(layoutTestsHash)
	isolateDeps := []string{layoutTestsHash}

	// Archive, trigger and collect swarming tasks.
	isolateExtraArgs := map[string]string{
		"RUN_ID": *runID,
	}
	// NEED TO FIX THIS! NO WAY THIS IS GOING TO WORK
	if _, err := util.TriggerSwarmingTask(ctx, "" /* pagesetType */, "layout_test", util.LAYOUT_TESTS_ISOLATE, *runID, 3*time.Hour, 1*time.Hour, util.ADMIN_TASKS_PRIORITY, 1, NUM_SWARMING_BOTS_TO_RUN_ON, isolateExtraArgs, true /* runOnGCE */, 1, isolateDeps); err != nil {
		sklog.Errorf("Error encountered when swarming tasks: %s", err)
		return
	}

	// DO THE MERGING HERE AND THEN UPLOAD TO GOOGLE STORAGE!

	// pixelDiffResultsLink = fmt.Sprintf(PIXEL_DIFF_RESULTS_LINK_TEMPLATE, *runID)
	taskCompletedSuccessfully = true
}

//func createAndUploadMetadataFile(gs *util.GcsUtil) error {
//	baseRemoteDir, err := util.GetBasePixelDiffRemoteDir(*runID)
//	if err != nil {
//		return fmt.Errorf("Error encountered when calculating remote base dir: %s", err)
//	}
//	noPatchRemoteDir := filepath.Join(baseRemoteDir, "nopatch")
//	totalNoPatchWebpages, err := gs.GetRemoteDirCount(noPatchRemoteDir)
//	if err != nil {
//		return fmt.Errorf("Could not find any content in %s: %s", noPatchRemoteDir, err)
//	}
//	withPatchRemoteDir := filepath.Join(baseRemoteDir, "withpatch")
//	totalWithPatchWebpages, err := gs.GetRemoteDirCount(withPatchRemoteDir)
//	if err != nil {
//		return fmt.Errorf("Could not find any content in %s: %s", withPatchRemoteDir, err)
//	}
//	metadata := Metadata{
//		RunID:                *runID,
//		NoPatchImagesCount:   totalNoPatchWebpages,
//		WithPatchImagesCount: totalWithPatchWebpages,
//		Description:          *description,
//	}
//	m, err := json.Marshal(&metadata)
//	if err != nil {
//		return fmt.Errorf("Could not marshall %s to json: %s", m, err)
//	}
//	localMetadataFileName := *runID + ".metadata.json"
//	localMetadataFilePath := filepath.Join(os.TempDir(), localMetadataFileName)
//	if err := ioutil.WriteFile(localMetadataFilePath, m, os.ModePerm); err != nil {
//		return fmt.Errorf("Could not write to %s: %s", localMetadataFilePath, err)
//	}
//	defer skutil.Remove(localMetadataFilePath)
//	if err := gs.UploadFile(localMetadataFileName, os.TempDir(), baseRemoteDir); err != nil {
//		return fmt.Errorf("Could not upload %s to %s: %s", localMetadataFileName, baseRemoteDir, err)
//	}
//	return nil
//}
