// Application that runs lua scripts over the specified SKP repository.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

var (
	workerNum     = flag.Int("worker_num", 1, "The number of this CT worker. It will be in the {1..100} range.")
	pagesetType   = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets to create from the Alexa CSV list. Eg: 10k, Mobile10k, All.")
	chromiumBuild = flag.String("chromium_build", "", "The chromium build that was used to create the SKPs we would like to run lua scripts against.")
	runID         = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
)

func main() {
	common.Init()
	defer util.TimeTrack(time.Now(), "Running Lua Scripts")
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
	skutil.LogErr(util.CreateTaskFile(util.ACTIVITY_RUNNING_LUA_SCRIPTS))
	defer util.DeleteTaskFile(util.ACTIVITY_RUNNING_LUA_SCRIPTS)

	// Sync Skia tree.
	skutil.LogErr(util.SyncDir(util.SkiaTreeDir))

	// Build tools.
	skutil.LogErr(util.BuildSkiaTools())

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

	// Download the lua script for this run from Google storage.
	luaScriptName := *runID + ".lua"
	luaScriptLocalPath := filepath.Join(os.TempDir(), luaScriptName)
	remoteDir := filepath.Join(util.LuaRunsDir, *runID)
	luaScriptRemotePath := filepath.Join(remoteDir, "scripts", luaScriptName)
	respBody, err := gs.GetRemoteFileContents(luaScriptRemotePath)
	if err != nil {
		glog.Errorf("Could not fetch %s: %s", luaScriptRemotePath, err)
		return
	}
	defer skutil.Close(respBody)
	out, err := os.Create(luaScriptLocalPath)
	if err != nil {
		glog.Errorf("Unable to create file %s: %s", luaScriptLocalPath, err)
		return
	}
	defer skutil.Close(out)
	defer skutil.Remove(luaScriptLocalPath)
	if _, err = io.Copy(out, respBody); err != nil {
		glog.Error(err)
		return
	}

	// Run lua_pictures and save stdout and stderr in files.
	stdoutFileName := *runID + ".output"
	stdoutFilePath := filepath.Join(os.TempDir(), stdoutFileName)
	stdoutFile, err := os.Create(stdoutFilePath)
	defer skutil.Close(stdoutFile)
	defer skutil.Remove(stdoutFilePath)
	if err != nil {
		glog.Errorf("Could not create %s: %s", stdoutFilePath, err)
		return
	}
	stderrFileName := *runID + ".err"
	stderrFilePath := filepath.Join(os.TempDir(), stderrFileName)
	stderrFile, err := os.Create(stderrFilePath)
	defer skutil.Close(stderrFile)
	defer skutil.Remove(stderrFilePath)
	if err != nil {
		glog.Errorf("Could not create %s: %s", stderrFilePath, err)
		return
	}
	args := []string{
		"--skpPath", localSkpsDir,
		"--luaFile", luaScriptLocalPath,
	}
	err = util.ExecuteCmd(
		filepath.Join(util.SkiaTreeDir, "out", "Release", util.BINARY_LUA_PICTURES), args,
		[]string{}, util.LUA_PICTURES_TIMEOUT, stdoutFile, stderrFile)
	if err != nil {
		glog.Error(err)
		return
	}

	// Copy stdout and stderr files to Google Storage.
	skutil.LogErr(
		gs.UploadFile(stdoutFileName, os.TempDir(), filepath.Join(remoteDir, fmt.Sprintf("slave%d", *workerNum), "outputs")))
	skutil.LogErr(
		gs.UploadFile(stderrFileName, os.TempDir(), filepath.Join(remoteDir, fmt.Sprintf("slave%d", *workerNum), "errors")))
}
