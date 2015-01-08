// run_lua_on_workers is an application that runs the specified lua script on all
// CT workers and uploads the results to Google Storage. The requester is emailed
// when the task is done.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/skia-dev/glog"
	"skia.googlesource.com/buildbot.git/ct/go/util"
	"skia.googlesource.com/buildbot.git/go/common"
)

var (
	emails        = flag.String("emails", "", "The comma separated email addresses to notify when the task is picked up and completes.")
	gaeTaskID     = flag.Int("gae_task_id", -1, "The key of the App Engine task. This task will be updated when the task is completed.")
	pagesetType   = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	chromiumBuild = flag.String("chromium_build", "", "The chromium build to use for this capture_archives run.")
	runID         = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")

	taskCompletedSuccessfully     = false
	luaScriptRemoteLink           = ""
	luaAggregatorRemoteLink       = ""
	luaOutputRemoteLink           = ""
	luaAggregatorOutputRemoteLink = ""
)

func sendEmail() {
	// Send completion email.
	emailsArr := util.ParseEmails(*emails)
	if len(emailsArr) == 0 {
		glog.Error("At least one email address must be specified")
		return
	}
	emailSubject := fmt.Sprintf("Run lua script Cluster telemetry task has completed (%s)", *runID)
	if !taskCompletedSuccessfully {
		emailSubject += " with failures"
	}
	// TODO(rmistry): Add a link to the master logs here and maybe a table with
	// links to logs of the 100 slaves.
	bodyTemplate := `
	The Cluster telemetry queued task to run lua script on %s pageset has completed.<br/>
	The output of your script is available <a href='%s'>here</a>.<br/>`
	if luaAggregatorOutputRemoteLink != "" {
		bodyTemplate += fmt.Sprintf("The aggregated output of your script (if specified) is available <a href='%s'>here</a>.<br/>", luaAggregatorOutputRemoteLink)
	}
	bodyTemplate += `
	You can schedule more runs <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, *pagesetType, luaOutputRemoteLink, util.LuaTasksWebapp)
	if err := util.SendEmail(emailsArr, emailSubject, emailBody); err != nil {
		glog.Errorf("Error while sending email: %s", err)
		return
	}
}

func updateWebappTask() {
	outputLink := luaOutputRemoteLink
	if luaAggregatorOutputRemoteLink != "" {
		// Use the aggregated output if it exists.
		outputLink = luaAggregatorOutputRemoteLink
	}
	extraData := map[string]string{
		"lua_script_link":     luaScriptRemoteLink,
		"lua_aggregator_link": luaAggregatorRemoteLink,
		"lua_output_link":     outputLink,
	}
	if err := util.UpdateWebappTask(*gaeTaskID, util.UpdateLuaTasksWebapp, extraData); err != nil {
		glog.Errorf("Error while updating webapp task: %s", err)
		return
	}
}

func main() {
	common.Init()
	// Ensure webapp is updated and email is sent even if task fails.
	defer updateWebappTask()
	defer sendEmail()
	defer util.TimeTrack(time.Now(), "Running Lua script on workers")
	defer glog.Flush()

	if *pagesetType == "" {
		glog.Error("Must specify --pageset_type")
		return
	}
	if *chromiumBuild == "" {
		glog.Error("Must specify --chromium_build")
		return
	}
	if *runID == "" {
		glog.Error("Must specify --run_id")
		return
	}

	// Instantiate GsUtil object.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Error(err)
		return
	}

	// Upload the lua script for this run to Google storage.
	luaScriptName := *runID + ".lua"
	defer os.Remove(filepath.Join(os.TempDir(), luaScriptName))
	luaScriptRemoteDir := filepath.Join(util.LuaRunsDir, *runID, "scripts")
	luaScriptRemoteLink = util.GS_HTTP_LINK + filepath.Join(util.GS_BUCKET_NAME, luaScriptRemoteDir, luaScriptName)
	if err := gs.UploadFile(luaScriptName, os.TempDir(), luaScriptRemoteDir); err != nil {
		glog.Errorf("Could not upload %s to %s: %s", luaScriptName, luaScriptRemoteDir, err)
		return
	}

	// Run the run_lua script on all workers.
	runLuaCmdTemplate := "DISPLAY=:0 run_lua --worker_num={{.WorkerNum}} --log_dir={{.LogDir}} --pageset_type={{.PagesetType}} --chromium_build={{.ChromiumBuild}} --run_id={{.RunID}};"
	runLuaTemplateParsed := template.Must(template.New("run_lua_cmd").Parse(runLuaCmdTemplate))
	luaCmdBytes := new(bytes.Buffer)
	runLuaTemplateParsed.Execute(luaCmdBytes, struct {
		WorkerNum     string
		LogDir        string
		PagesetType   string
		ChromiumBuild string
		RunID         string
	}{
		WorkerNum:     util.WORKER_NUM_KEYWORD,
		LogDir:        util.GLogDir,
		PagesetType:   *pagesetType,
		ChromiumBuild: *chromiumBuild,
		RunID:         *runID,
	})
	cmd := []string{
		fmt.Sprintf("cd %s;", util.CtTreeDir),
		"git pull;",
		"make worker_scripts;",
		// The main command that runs run_lua on all workers.
		luaCmdBytes.String(),
	}
	if _, err := util.SSH(strings.Join(cmd, " "), util.Slaves, 2*time.Hour); err != nil {
		glog.Errorf("Error while running cmd %s: %s", cmd, err)
		return
	}

	// Copy outputs from all slaves locally and combine it into one file.
	consolidatedFileName := "lua-output"
	consolidatedLuaOutput := filepath.Join(os.TempDir(), consolidatedFileName)
	if err := ioutil.WriteFile(consolidatedLuaOutput, []byte{}, 0660); err != nil {
		glog.Errorf("Could not create %s: %s", consolidatedLuaOutput, err)
		return
	}
	for i := 0; i < util.NUM_WORKERS; i++ {
		workerNum := i + 1
		workerRemoteOutputPath := filepath.Join(util.LuaRunsDir, *runID, fmt.Sprintf("slave%d", workerNum), "outputs", *runID+".output")
		respBody, err := gs.GetRemoteFileContents(workerRemoteOutputPath)
		if err != nil {
			glog.Errorf("Could not fetch %s: %s", workerRemoteOutputPath, err)
			// TODO(rmistry): Should we instead return here? We can only return
			// here if all 100 slaves reliably run without any failures which they
			// really should.
			continue
		}
		defer respBody.Close()
		out, err := os.OpenFile(consolidatedLuaOutput, os.O_RDWR|os.O_APPEND, 0660)
		if err != nil {
			glog.Errorf("Unable to open file %s: %s", consolidatedLuaOutput, err)
			return
		}
		defer out.Close()
		defer os.Remove(consolidatedLuaOutput)
		if _, err = io.Copy(out, respBody); err != nil {
			glog.Errorf("Unable to write out %s to %s: %s", workerRemoteOutputPath, consolidatedLuaOutput, err)
			return
		}
	}
	// Copy the consolidated file into Google Storage.
	consolidatedOutputRemoteDir := filepath.Join(util.LuaRunsDir, *runID, "consolidated_outputs")
	luaOutputRemoteLink = util.GS_HTTP_LINK + filepath.Join(util.GS_BUCKET_NAME, consolidatedOutputRemoteDir, consolidatedFileName)
	if err := gs.UploadFile(consolidatedFileName, os.TempDir(), consolidatedOutputRemoteDir); err != nil {
		glog.Errorf("Unable to upload %s to %s: %s", consolidatedLuaOutput, consolidatedOutputRemoteDir, err)
		return
	}

	// Upload the lua aggregator (if specified) for this run to Google storage.
	luaAggregatorName := *runID + ".aggregator"
	luaAggregatorPath := filepath.Join(os.TempDir(), luaAggregatorName)
	defer os.Remove(luaAggregatorPath)
	luaAggregatorRemoteLink = util.GS_HTTP_LINK + filepath.Join(util.GS_BUCKET_NAME, luaScriptRemoteDir, luaAggregatorName)
	luaAggregatorFileInfo, err := os.Stat(luaAggregatorPath)
	if !os.IsNotExist(err) && luaAggregatorFileInfo.Size() > 10 {
		if err := gs.UploadFile(luaAggregatorName, os.TempDir(), luaScriptRemoteDir); err != nil {
			glog.Errorf("Could not upload %s to %s: %s", luaAggregatorName, luaScriptRemoteDir, err)
			return
		}
		// Run the aggregator and save stdout.
		luaAggregatorOutputFileName := *runID + ".agg.output"
		luaAggregatorOutputFilePath := filepath.Join(os.TempDir(), luaAggregatorOutputFileName)
		luaAggregatorOutputFile, err := os.Create(luaAggregatorOutputFilePath)
		defer luaAggregatorOutputFile.Close()
		defer os.Remove(luaAggregatorOutputFilePath)
		if err != nil {
			glog.Errorf("Could not create %s: %s", luaAggregatorOutputFilePath, err)
			return
		}
		if err := util.ExecuteCmd(util.BINARY_LUA, []string{luaAggregatorPath}, []string{}, time.Hour, luaAggregatorOutputFile, nil); err != nil {
			glog.Errorf("Could not execute the lua aggregator %s: %s", luaAggregatorPath, err)
			return
		}
		// Copy the aggregator output into Google Storage.
		luaAggregatorOutputRemoteLink = util.GS_HTTP_LINK + filepath.Join(util.GS_BUCKET_NAME, consolidatedOutputRemoteDir, luaAggregatorOutputFileName)
		if err := gs.UploadFile(luaAggregatorOutputFileName, os.TempDir(), consolidatedOutputRemoteDir); err != nil {
			glog.Errorf("Unable to upload %s to %s: %s", luaAggregatorOutputFileName, consolidatedOutputRemoteDir, err)
			return
		}
	} else {
		glog.Info("A lua aggregator has not been specified.")
	}

	taskCompletedSuccessfully = true
}
