// run_lua_on_workers is an application that runs the specified lua script on all
// CT workers and uploads the results to Google Storage. The requester is emailed
// when the task is done.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"go.skia.org/infra/ct/go/master_scripts/master_common"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

const (
	MAX_PAGES_PER_SWARMING_BOT = 10000
)

var (
	pagesetType               = flag.String("pageset_type", "", "The type of pagesets to use. Eg: 10k, Mobile10k, All.")
	chromiumBuild             = flag.String("chromium_build", "", "The chromium build to use for this capture_archives run.")
	runOnGCE                  = flag.Bool("run_on_gce", true, "Run on Linux GCE instances.")
	runID                     = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	luaScriptGSPath           = flag.String("lua_script_gs_path", "", "The location of the lua script to run on workers in Google storage.")
	luaAggregatorScriptGSPath = flag.String("lua_aggregator_script_gs_path", "", "The location of the lua aggregator script in Google storage.")
)

func runLuaOnWorkers() error {
	master_common.Init("run_lua")

	ctx := context.Background()

	// Finish with glog flush and how long the task took.
	defer util.TimeTrack(time.Now(), "Running Lua script on workers")
	defer sklog.Flush()

	if *pagesetType == "" {
		return errors.New("Must specify --pageset_type")
	}
	if *chromiumBuild == "" {
		return errors.New("Must specify --chromium_build")
	}
	if *runID == "" {
		return errors.New("Must specify --run_id")
	}

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		return err
	}

	// Build lua_pictures.
	cipdPackage, err := util.GetCipdPackageFromAsset("clang_linux")
	if err != nil {
		return fmt.Errorf("Could not get cipd package for clang_linux: %s", err)
	}
	remoteDirNames, err := util.TriggerBuildRepoSwarmingTask(
		ctx, "build_lua_pictures", *runID, "skiaLuaPictures", util.PLATFORM_LINUX, "", []string{}, []string{}, []string{cipdPackage}, true, *master_common.Local, 3*time.Hour, 1*time.Hour)
	if err != nil {
		return fmt.Errorf("Error encountered when swarming build lua_pictures task: %s", err)
	}
	luaPicturesRemoteDirName := remoteDirNames[0]
	luaPicturesRemotePath := path.Join(util.BINARIES_DIR_NAME, luaPicturesRemoteDirName, util.BINARY_LUA_PICTURES)

	// Empty the remote dir before the workers upload to it.
	gsBaseDir := filepath.Join(util.SWARMING_DIR_NAME, filepath.Join(util.LuaRunsDir, *runID), *pagesetType)
	skutil.LogErr(gs.DeleteRemoteDir(gsBaseDir))

	// Archive, trigger and collect swarming tasks.
	isolateExtraArgs := map[string]string{
		"CHROMIUM_BUILD":           *chromiumBuild,
		"RUN_ID":                   *runID,
		"LUA_PICTURES_REMOTE_PATH": luaPicturesRemotePath,
		"LUA_SCRIPT_GS_PATH":       *luaScriptGSPath,
	}
	if _, err := util.TriggerSwarmingTask(ctx, *pagesetType, "run_lua", util.RUN_LUA_ISOLATE, *runID, "", util.PLATFORM_LINUX, 3*time.Hour, 1*time.Hour, util.TASKS_PRIORITY_MEDIUM, MAX_PAGES_PER_SWARMING_BOT, util.PagesetTypeToInfo[*pagesetType].NumPages, isolateExtraArgs, *runOnGCE, *master_common.Local, 1, []string{} /* isolateDeps */); err != nil {
		return fmt.Errorf("Error encountered when swarming tasks: %s", err)
	}

	// Copy outputs from all slaves locally and combine it into one file.
	consolidatedFileName := util.GetLuaConsolidatedFileName()
	// Aggregated scripts use dofile("/tmp/lua-output")
	consolidatedLuaOutput := filepath.Join("/", "tmp", consolidatedFileName)
	// If the file already exists it could be that there is another lua task running on this machine.
	// Wait for the file to be deleted within a deadline.
	if err := waitForOutputFile(consolidatedLuaOutput); err != nil {
		return err
	}
	defer skutil.Remove(consolidatedLuaOutput)
	if err := ioutil.WriteFile(consolidatedLuaOutput, []byte{}, 0660); err != nil {
		return fmt.Errorf("Could not create %s: %s", consolidatedLuaOutput, err)
	}
	numTasks := int(math.Ceil(float64(util.PagesetTypeToInfo[*pagesetType].NumPages) / float64(MAX_PAGES_PER_SWARMING_BOT)))
	for i := 1; i <= numTasks; i++ {
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
			return fmt.Errorf("Unable to open file %s: %s", consolidatedLuaOutput, err)
		}
		defer skutil.Close(out)
		if _, err = io.Copy(out, respBody); err != nil {
			return fmt.Errorf("Unable to write out %s to %s: %s", workerRemoteOutputPath, consolidatedLuaOutput, err)
		}
	}
	// Copy the consolidated file into Google Storage.
	consolidatedOutputRemoteDir := util.GetLuaConsolidatedOutputRemoteDir(*runID)
	if err := gs.UploadFile(consolidatedFileName, "/tmp", consolidatedOutputRemoteDir); err != nil {
		return fmt.Errorf("Unable to upload %s to %s: %s", consolidatedLuaOutput, consolidatedOutputRemoteDir, err)
	}

	// Download the lua aggregator (if specified) from Google storage and run it.
	if *luaAggregatorScriptGSPath != "" {
		luaAggregator, err := util.GetPatchFromStorage(*luaAggregatorScriptGSPath)
		if err != nil {
			return fmt.Errorf("Could not download lua aggregator script %s from Google storage: %s", *luaAggregatorScriptGSPath, err)
		}
		luaAggregatorPath := filepath.Join(os.TempDir(), *runID+".aggregator")
		if err := ioutil.WriteFile(luaAggregatorPath, []byte(luaAggregator), 0666); err != nil {
			return fmt.Errorf("Could not write lua aggregator %s to %s: %s", luaAggregator, luaAggregatorPath, err)
		}
		defer skutil.Remove(luaAggregatorPath)

		// Run the aggregator and save stdout.
		luaAggregatorOutputFileName := util.GetLuaAggregatorOutputFileName(*runID)
		luaAggregatorOutputFilePath := filepath.Join(os.TempDir(), luaAggregatorOutputFileName)
		luaAggregatorOutputFile, err := os.Create(luaAggregatorOutputFilePath)
		defer skutil.Close(luaAggregatorOutputFile)
		defer skutil.Remove(luaAggregatorOutputFilePath)
		if err != nil {
			return fmt.Errorf("Could not create %s: %s", luaAggregatorOutputFilePath, err)
		}
		err = util.ExecuteCmd(ctx, util.BINARY_LUA, []string{luaAggregatorPath}, []string{},
			util.LUA_AGGREGATOR_TIMEOUT, luaAggregatorOutputFile, nil)
		if err != nil {
			return fmt.Errorf("Could not execute the lua aggregator %s: %s", luaAggregatorPath, err)
		}
		// Copy the aggregator output into Google Storage.
		if err := gs.UploadFile(luaAggregatorOutputFileName, os.TempDir(), consolidatedOutputRemoteDir); err != nil {
			return fmt.Errorf("Unable to upload %s to %s: %s", luaAggregatorOutputFileName, consolidatedOutputRemoteDir, err)
		}
	} else {
		sklog.Info("A lua aggregator has not been specified.")
	}

	return nil
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

func main() {
	retCode := 0
	if err := runLuaOnWorkers(); err != nil {
		sklog.Errorf("Error while running lua on workers: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
