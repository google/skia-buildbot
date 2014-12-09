// Application that runs lua scripts over the specified SKP repository.
package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"

	"skia.googlesource.com/buildbot.git/ct/go/util"
	"skia.googlesource.com/buildbot.git/go/common"
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

	// Create the task file so that the master knows this worker is still busy.
	util.CreateTaskFile(util.ACTIVITY_RUNNING_LUA_SCRIPTS)
	defer util.DeleteTaskFile(util.ACTIVITY_RUNNING_LUA_SCRIPTS)

	// Sync Skia tree.
	util.SyncDir(util.SkiaTreeDir)

	// Build tools.
	util.BuildSkiaTools()

	// Instantiate GsUtil object.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Fatal(err)
	}

	// Download SKPs if they do not exist locally.
	if err := gs.DownloadWorkerArtifacts(util.SKPS_DIR_NAME, filepath.Join(*pagesetType, *chromiumBuild), *workerNum); err != nil {
		glog.Fatal(err)
	}
	localSkpsDir := filepath.Join(util.SkpsDir, *pagesetType, *chromiumBuild)

	// Download the lua script for this run from Google storage.
	luaScriptName := *runID + ".lua"
	luaScriptLocalPath := filepath.Join(os.TempDir(), luaScriptName)
	remoteDir := filepath.Join(util.LuaRunsDir, *runID)
	luaScriptRemotePath := filepath.Join(remoteDir, "scripts", luaScriptName)
	respBody, err := gs.GetRemoteFileContents(luaScriptRemotePath)
	if err != nil {
		glog.Fatalf("Could not fetch %s: %s", luaScriptRemotePath, err)
	}
	defer respBody.Close()
	out, err := os.Create(luaScriptLocalPath)
	if err != nil {
		glog.Fatalf("Unable to create file %s: %s", luaScriptLocalPath, err)
	}
	defer out.Close()
	defer os.Remove(luaScriptLocalPath)
	if _, err = io.Copy(out, respBody); err != nil {
		glog.Fatal(err)
	}

	// Run lua_pictures and save stdout and stderr in files.
	stdoutFileName := *runID + ".output"
	stdoutFilePath := filepath.Join(os.TempDir(), stdoutFileName)
	stdoutFile, err := os.Create(stdoutFilePath)
	defer stdoutFile.Close()
	defer os.Remove(stdoutFilePath)
	if err != nil {
		glog.Fatalf("Could not create %s: %s", stdoutFilePath, err)
	}
	stderrFileName := *runID + ".err"
	stderrFilePath := filepath.Join(os.TempDir(), stderrFileName)
	stderrFile, err := os.Create(stderrFilePath)
	defer stderrFile.Close()
	defer os.Remove(stderrFilePath)
	if err != nil {
		glog.Fatalf("Could not create %s: %s", stderrFilePath, err)
	}
	args := []string{
		"--skpPath", localSkpsDir,
		"--luaFile", luaScriptLocalPath,
	}
	util.ExecuteCmd(filepath.Join(util.SkiaTreeDir, "out", "Release", util.BINARY_LUA_PICTURES), args, []string{}, true, 15*time.Minute, stdoutFile, stderrFile)

	// Copy stdout and stderr files to Google Storage.
	gs.UploadFile(stdoutFileName, os.TempDir(), filepath.Join(remoteDir, "outputs"))
	gs.UploadFile(stderrFileName, os.TempDir(), filepath.Join(remoteDir, "errors"))
}
