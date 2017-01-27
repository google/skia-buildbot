// run_lua_on_workers is an application that runs the specified lua script on all
// CT workers and uploads the results to Google Storage. The requester is emailed
// when the task is done.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go.skia.org/infra/ct/go/ctfe/lua_scripts"
	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	MAX_PAGES_PER_SWARMING_BOT = 10000
)

var (
	emails        = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	description   = flag.String("description", "", "The description of the run as entered by the requester.")
	gaeTaskID     = flag.Int64("gae_task_id", -1, "The key of the App Engine task. This task will be updated when the task is completed.")
	pagesetType   = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	chromiumBuild = flag.String("chromium_build", "", "The chromium build to use for this capture_archives run.")
	runID         = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")

	taskCompletedSuccessfully     = false
	luaOutputRemoteLink           = ""
	luaAggregatorOutputRemoteLink = ""
)

func sendEmail(recipients []string) {
	// Send completion email.
	emailSubject := fmt.Sprintf("Run lua script Cluster telemetry task has completed (%s)", *runID)
	failureHtml := ""
	if !taskCompletedSuccessfully {
		emailSubject += " with failures"
		failureHtml = util.GetFailureEmailHtml(*runID)
	}
	scriptOutputHtml := ""
	if luaOutputRemoteLink != "" {
		scriptOutputHtml = fmt.Sprintf("The output of your script is available <a href='%s'>here</a>.<br/>\n", luaOutputRemoteLink)
	}
	aggregatorOutputHtml := ""
	if luaAggregatorOutputRemoteLink != "" {
		aggregatorOutputHtml = fmt.Sprintf("The aggregated output of your script is available <a href='%s'>here</a>.<br/>\n", luaAggregatorOutputRemoteLink)
	}
	bodyTemplate := `
	The Cluster telemetry queued task to run lua script on %s pageset has completed. %s.<br/>
	Run description: %s<br/>
	%s
	%s
	%s
	You can schedule more runs <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *pagesetType, util.GetSwarmingLogsLink(*runID), *description, failureHtml, scriptOutputHtml, aggregatorOutputHtml, frontend.LuaTasksWebapp)
	if err := util.SendEmail(recipients, emailSubject, emailBody); err != nil {
		sklog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateWebappTask() {
	vars := lua_scripts.UpdateVars{}
	vars.Id = *gaeTaskID
	vars.SetCompleted(taskCompletedSuccessfully)
	if luaOutputRemoteLink != "" {
		vars.ScriptOutput = sql.NullString{String: luaOutputRemoteLink, Valid: true}
	}
	if luaAggregatorOutputRemoteLink != "" {
		vars.AggregatedOutput = sql.NullString{String: luaAggregatorOutputRemoteLink, Valid: true}
	}
	skutil.LogErr(frontend.UpdateWebappTaskV2(&vars))
}

func main() {
	defer common.LogPanic()
	master_common.Init("run_lua")

	// Send start email.
	emailsArr := util.ParseEmails(*emails)
	emailsArr = append(emailsArr, util.CtAdmins...)
	if len(emailsArr) == 0 {
		sklog.Error("At least one email address must be specified")
		return
	}
	skutil.LogErr(frontend.UpdateWebappTaskSetStarted(&lua_scripts.UpdateVars{}, *gaeTaskID))
	skutil.LogErr(util.SendTaskStartEmail(emailsArr, "Lua script", *runID, *description))
	// Ensure webapp is updated and email is sent even if task fails.
	defer updateWebappTask()
	defer sendEmail(emailsArr)
	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running Lua script on workers")
	defer sklog.Flush()

	if *pagesetType == "" {
		sklog.Error("Must specify --pageset_type")
		return
	}
	if *chromiumBuild == "" {
		sklog.Error("Must specify --chromium_build")
		return
	}
	if *runID == "" {
		sklog.Error("Must specify --run_id")
		return
	}

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		sklog.Error(err)
		return
	}

	// Upload the lua script for this run to Google storage.
	luaScriptName := *runID + ".lua"
	defer skutil.Remove(filepath.Join(os.TempDir(), luaScriptName))
	luaScriptRemoteDir := filepath.Join(util.LuaRunsDir, *runID, "scripts")
	if err := gs.UploadFile(luaScriptName, os.TempDir(), luaScriptRemoteDir); err != nil {
		sklog.Errorf("Could not upload %s to %s: %s", luaScriptName, luaScriptRemoteDir, err)
		return
	}

	// Empty the remote dir before the workers upload to it.
	gsBaseDir := filepath.Join(util.SWARMING_DIR_NAME, filepath.Join(util.LuaRunsDir, *runID), *pagesetType)
	skutil.LogErr(gs.DeleteRemoteDir(gsBaseDir))

	// Archive, trigger and collect swarming tasks.
	isolateExtraArgs := map[string]string{
		"CHROMIUM_BUILD": *chromiumBuild,
		"RUN_ID":         *runID,
	}
	if err := util.TriggerSwarmingTask(*pagesetType, "run_lua", util.RUN_LUA_ISOLATE, *runID, 3*time.Hour, 1*time.Hour, util.USER_TASKS_PRIORITY, MAX_PAGES_PER_SWARMING_BOT, util.PagesetTypeToInfo[*pagesetType].NumPages, isolateExtraArgs, util.GCE_WORKER_DIMENSIONS); err != nil {
		sklog.Errorf("Error encountered when swarming tasks: %s", err)
		return
	}

	// Copy outputs from all slaves locally and combine it into one file.
	consolidatedFileName := "lua-output"
	consolidatedLuaOutput := filepath.Join(os.TempDir(), consolidatedFileName)
	// If the file already exists it could be that there is another lua task running on this machine.
	// Wait for the file to be deleted within a deadline.
	if err := waitForOutputFile(consolidatedLuaOutput); err != nil {
		sklog.Error(err)
		return
	}
	defer skutil.Remove(consolidatedLuaOutput)
	if err := ioutil.WriteFile(consolidatedLuaOutput, []byte{}, 0660); err != nil {
		sklog.Errorf("Could not create %s: %s", consolidatedLuaOutput, err)
		return
	}
	for i := 1; i <= util.PagesetTypeToInfo[*pagesetType].NumPages/MAX_PAGES_PER_SWARMING_BOT; i++ {
		startRange := strconv.Itoa(util.GetStartRange(i, MAX_PAGES_PER_SWARMING_BOT))
		workerRemoteOutputPath := filepath.Join(util.LuaRunsDir, *runID, startRange, "outputs", *runID+".output")
		respBody, err := gs.GetRemoteFileContents(workerRemoteOutputPath)
		if err != nil {
			sklog.Errorf("Could not fetch %s: %s", workerRemoteOutputPath, err)
			continue
		}
		defer skutil.Close(respBody)
		out, err := os.OpenFile(consolidatedLuaOutput, os.O_RDWR|os.O_APPEND, 0660)
		if err != nil {
			sklog.Errorf("Unable to open file %s: %s", consolidatedLuaOutput, err)
			return
		}
		defer skutil.Close(out)
		if _, err = io.Copy(out, respBody); err != nil {
			sklog.Errorf("Unable to write out %s to %s: %s", workerRemoteOutputPath, consolidatedLuaOutput, err)
			return
		}
	}
	// Copy the consolidated file into Google Storage.
	consolidatedOutputRemoteDir := filepath.Join(util.LuaRunsDir, *runID, "consolidated_outputs")
	luaOutputRemoteLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, consolidatedOutputRemoteDir, consolidatedFileName)
	if err := gs.UploadFile(consolidatedFileName, os.TempDir(), consolidatedOutputRemoteDir); err != nil {
		sklog.Errorf("Unable to upload %s to %s: %s", consolidatedLuaOutput, consolidatedOutputRemoteDir, err)
		return
	}

	// Upload the lua aggregator (if specified) for this run to Google storage.
	luaAggregatorName := *runID + ".aggregator"
	luaAggregatorPath := filepath.Join(os.TempDir(), luaAggregatorName)
	defer skutil.Remove(luaAggregatorPath)
	luaAggregatorFileInfo, err := os.Stat(luaAggregatorPath)
	if !os.IsNotExist(err) && luaAggregatorFileInfo.Size() > 10 {
		if err := gs.UploadFile(luaAggregatorName, os.TempDir(), luaScriptRemoteDir); err != nil {
			sklog.Errorf("Could not upload %s to %s: %s", luaAggregatorName, luaScriptRemoteDir, err)
			return
		}
		// Run the aggregator and save stdout.
		luaAggregatorOutputFileName := *runID + ".agg.output"
		luaAggregatorOutputFilePath := filepath.Join(os.TempDir(), luaAggregatorOutputFileName)
		luaAggregatorOutputFile, err := os.Create(luaAggregatorOutputFilePath)
		defer skutil.Close(luaAggregatorOutputFile)
		defer skutil.Remove(luaAggregatorOutputFilePath)
		if err != nil {
			sklog.Errorf("Could not create %s: %s", luaAggregatorOutputFilePath, err)
			return
		}
		err = util.ExecuteCmd(util.BINARY_LUA, []string{luaAggregatorPath}, []string{},
			util.LUA_AGGREGATOR_TIMEOUT, luaAggregatorOutputFile, nil)
		if err != nil {
			sklog.Errorf("Could not execute the lua aggregator %s: %s", luaAggregatorPath, err)
			return
		}
		// Copy the aggregator output into Google Storage.
		luaAggregatorOutputRemoteLink = util.GCS_HTTP_LINK + filepath.Join(util.GCSBucketName, consolidatedOutputRemoteDir, luaAggregatorOutputFileName)
		if err := gs.UploadFile(luaAggregatorOutputFileName, os.TempDir(), consolidatedOutputRemoteDir); err != nil {
			sklog.Errorf("Unable to upload %s to %s: %s", luaAggregatorOutputFileName, consolidatedOutputRemoteDir, err)
			return
		}
	} else {
		sklog.Info("A lua aggregator has not been specified.")
	}

	taskCompletedSuccessfully = true
}

func waitForOutputFile(luaOutput string) error {
	// Check every 10 secs and timeout after 10 mins.
	ticker := time.NewTicker(10 * time.Second)
	deadline := 10 * time.Minute
	deadlineTicker := time.NewTicker(deadline)
	defer ticker.Stop()
	defer deadlineTicker.Stop()
	for {
		select {
		case <-ticker.C:
			if _, err := os.Stat(luaOutput); os.IsNotExist(err) {
				return nil
			}
			sklog.Infof("%s still exists. Waiting for the other lua task to complete.", luaOutput)
		case <-deadlineTicker.C:
			return fmt.Errorf("%s still existed after %v secs", luaOutput, deadline.Seconds())
		}
	}
}
