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
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

var (
	runID   = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	numBots = flag.Int("num", 0, "The number of swarming bots to run on.")

	taskCompletedSuccessfully = false
)

func main() {
	master_common.Init("run_layout_tests_on_workers")

	ctx := context.Background()

	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running layout tests on workers")
	defer sklog.Flush()

	if *runID == "" {
		sklog.Error("Must specify --run_id")
		return
	}

	if *numBots == 0 {
		sklog.Error("Must specify --num > 0. Cannot run on <=0 swarming bots.")
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
	isolateDeps := []string{layoutTestsHash}

	// Archive, trigger and collect swarming tasks.
	isolateExtraArgs := map[string]string{
		"RUN_ID": *runID,
	}
	if _, err := util.TriggerSwarmingTask(ctx, "" /* pagesetType */, "layout_test", util.LAYOUT_TESTS_ISOLATE, *runID, 24*time.Hour, 1*time.Hour, util.ADMIN_TASKS_PRIORITY, 1, *numBots, isolateExtraArgs, true /* runOnGCE */, 1, isolateDeps); err != nil {
		sklog.Errorf("Error encountered when swarming tasks: %s", err)
		return
	}

	// Merge output files and upload consolidated file to google storage.
	localOutputDir := filepath.Join(util.StorageDir, util.LayoutTestRunsDir, *runID)
	skutil.MkdirAll(localOutputDir, 0700)
	defer skutil.RemoveAll(localOutputDir)

	outputJSONFiles := []string{}
	foundOneFile := false
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
	}
	args = append(args, outputJSONFiles...)
	if err := util.ExecuteCmd(ctx, "python", args, nil, 1*time.Hour, nil, nil); err != nil {
		sklog.Errorf("Error when running %s: %s", pathToMergerScript, err)
		return
	}

	// Upload it.
	if err := gs.UploadFile(fmt.Sprintf("%s.json", *runID), path.Join(util.StorageDir, util.LayoutTestRunsDir), path.Join(util.SWARMING_DIR_NAME, util.LayoutTestRunsDir)); err != nil {
		sklog.Errorf("Could not uploaded the consolidated file: %s", err)
		return
	}

	fmt.Printf("%d outputs were found", len(outputJSONFiles))
	fmt.Printf("The consolidated output is available in %s", consolidatedOutputFile)
	fmt.Println("It has been uploaded to Google Storage here: %s", path.Join(util.SWARMING_DIR_NAME, util.LayoutTestRunsDir, fmt.Sprintf("%s.json", *runID)))

	taskCompletedSuccessfully = true
}
