// run_layout_tests_on_workers is an application that run the layout tests
// on many CT workers and uploads the results to Google Storage. The requester
// is emailed when the task is done.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

var (
	emails  = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	runID   = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	numBots = flag.Int("num", 0, "The number of swarming bots to run on.")

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
	layoutTestsHash, err := util.TriggerIsolateLayoutTestsSwarmingTask(ctx, "isolate_layout_tests", *runID, chromiumHash, 1*time.Hour, 1*time.Hour)
	if err != nil {
		sklog.Errorf("Error encountered when swarming isolate layout tests task: %s", err)
		return
	}
	if layoutTestsHash == "" {
		sklog.Errorf("Found empty telemetry hash!")
		return
	}
	//layoutTestsHash := "e41893db3547a0fdd9b071705723068a4eb2fd01"
	fmt.Println(chromiumHash)
	fmt.Println("HASH IS THIS!!!!!!")
	fmt.Println(layoutTestsHash)
	isolateDeps := []string{layoutTestsHash}

	// Archive, trigger and collect swarming tasks.
	isolateExtraArgs := map[string]string{
		"RUN_ID": *runID,
	}
	if _, err := util.TriggerSwarmingTask(ctx, "" /* pagesetType */, "layout_test", util.LAYOUT_TESTS_ISOLATE, *runID, 24*time.Hour, 1*time.Hour, util.ADMIN_TASKS_PRIORITY, 1, *numBots, isolateExtraArgs, true /* runOnGCE */, 1, isolateDeps); err != nil {
		sklog.Errorf("Error encountered when swarming tasks: %s", err)
		return
	}

	// DO THE MERGING HERE AND THEN UPLOAD TO GOOGLE STORAGE!
	localOutputDir := filepath.Join(util.StorageDir, util.LayoutTestRunsDir, *runID)
	skutil.MkdirAll(localOutputDir, 0700)
	defer skutil.RemoveAll(localOutputDir)

	foundOneFile := false
	// This will be needed if globs do not work!!
	outputJSONFiles := []string{}
	gsDir := path.Join(util.SWARMING_DIR_NAME, util.LayoutTestRunsDir, *runID)
	for i := 1; i <= *numBots; i++ {
		outputFileName := fmt.Sprintf("worker%d.json", i)
		workerLocalOutputPath := filepath.Join(localOutputDir, outputFileName)
		gsOutputFilePath := path.Join(gsDir, outputFileName)
		respBody, err := gs.GetRemoteFileContents(gsOutputFilePath)
		if err != nil {
			sklog.Errorf("Could not fetch %s", gsOutputFilePath)
			continue
		}
		sklog.Infof("Found %s", gsOutputFilePath)
		foundOneFile = true

		defer skutil.Close(respBody)
		out, err := os.Create(workerLocalOutputPath)
		if err != nil {
			sklog.Errorf("Unable to create file %s: %s", workerLocalOutputPath, err)
			return
		}
		defer skutil.Close(out)
		if _, err = io.Copy(out, respBody); err != nil {
			sklog.Errorf("Unable to copy to file %s: %s", workerLocalOutputPath, err)
			return
		}
		// If an output is less than 20 bytes that means something went wrong on the slave.
		outputInfo, err := out.Stat()
		if err != nil {
			sklog.Errorf("Unable to stat file %s: %s", workerLocalOutputPath, err)
			return
		}
		if outputInfo.Size() <= 20 {
			sklog.Errorf("Output file was less than 20 bytes %s: %s", workerLocalOutputPath, err)
			continue
		}
		outputJSONFiles = append(outputJSONFiles, workerLocalOutputPath)
	}
	if !foundOneFile {
		sklog.Error("Workers returned no results!")
		return
	}

	// Call the python script to merge all results.
	pathToPyFiles := util.GetPathToPyFiles(false)
	pathToMergerScript := path.Join(pathToPyFiles, "layout_merge_scripts", "standard_isolated_script_merge.py")
	consolidatedOutputFile := path.Join(util.StorageDir, util.LayoutTestRunsDir, fmt.Sprintf("%s.json", *runID))
	args := []string{
		pathToMergerScript,
		"-o", consolidatedOutputFile,
		//path.Join(localOutputDir, "*.json"),
	}
	args = append(args, outputJSONFiles...)
	if err := util.ExecuteCmd(ctx, "python", args, nil, 1*time.Hour, nil, nil); err != nil {
		sklog.Errorf("Error when running %s: %s", pathToMergerScript, err)
		return
	}

	fmt.Printf("%d outputs were found", len(outputJSONFiles))
	fmt.Printf("The consolidated output is available in %s", consolidatedOutputFile)

	taskCompletedSuccessfully = true
}
