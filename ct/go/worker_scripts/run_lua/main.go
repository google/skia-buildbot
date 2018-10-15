// Application that runs lua scripts over the specified SKP repository.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

var (
	startRange            = flag.Int("start_range", 1, "The number this worker will run lua scripts from.")
	num                   = flag.Int("num", 100, "The total number of SKPs to run on starting from the start_range.")
	pagesetType           = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets to create from the Alexa CSV list. Eg: 10k, Mobile10k, All.")
	chromiumBuild         = flag.String("chromium_build", "", "The chromium build that was used to create the SKPs we would like to run lua scripts against.")
	luaPicturesRemotePath = flag.String("lua_pictures_remote_path", "", "The location of the lua_pictures binary in Google Storage.")
	runID                 = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
)

func runLua() error {
	worker_common.Init()
	if !*worker_common.Local {
		defer util.CleanTmpDir()
	}
	defer util.TimeTrack(time.Now(), "Running Lua Scripts")
	defer sklog.Flush()

	if *chromiumBuild == "" {
		return errors.New("Must specify --chromium_build")
	}
	if *luaPicturesRemotePath == "" {
		return errors.New("Must specify --lua_pictures_remote_path")
	}
	if *runID == "" {
		return errors.New("Must specify --run_id")
	}

	ctx := context.Background()

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		return err
	}

	// Download SKPs if they do not exist locally.
	localSkpsDir := filepath.Join(util.SkpsDir, *pagesetType, *chromiumBuild)
	if _, err := gs.DownloadSwarmingArtifacts(localSkpsDir, util.SKPS_DIR_NAME, path.Join(*pagesetType, *chromiumBuild), *startRange, *num); err != nil {
		return err
	}
	defer skutil.RemoveAll(localSkpsDir)

	// Download the lua script for this run from Google storage.
	luaScriptName := *runID + ".lua"
	luaScriptLocalPath := filepath.Join(os.TempDir(), luaScriptName)
	remoteDir := filepath.Join(util.LuaRunsDir, *runID)
	luaScriptRemotePath := filepath.Join(remoteDir, "scripts", luaScriptName)
	respBody, err := gs.GetRemoteFileContents(luaScriptRemotePath)
	if err != nil {
		return fmt.Errorf("Could not fetch %s: %s", luaScriptRemotePath, err)
	}
	defer skutil.Close(respBody)
	out, err := os.Create(luaScriptLocalPath)
	if err != nil {
		return fmt.Errorf("Unable to create file %s: %s", luaScriptLocalPath, err)
	}
	defer skutil.Close(out)
	defer skutil.Remove(luaScriptLocalPath)
	if _, err = io.Copy(out, respBody); err != nil {
		return err
	}

	// Copy over the lua_pictures binary to this worker.
	luaPicturesLocalPath := filepath.Join(os.TempDir(), util.BINARY_LUA_PICTURES)
	luaPicturesRespBody, err := gs.GetRemoteFileContents(*luaPicturesRemotePath)
	if err != nil {
		return fmt.Errorf("Could not fetch %s: %s", *luaPicturesRemotePath, err)
	}
	defer skutil.Close(luaPicturesRespBody)
	writeErr := skutil.WithWriteFile(luaPicturesLocalPath, func(w io.Writer) error {
		_, err = io.Copy(w, luaPicturesRespBody)
		return err
	})
	if writeErr != nil {
		return fmt.Errorf("Failed to write to %s: %s", luaPicturesLocalPath, writeErr)
	}
	// Downloaded lua_pictures binary needs to be set as an executable.
	if err := os.Chmod(luaPicturesLocalPath, 0777); err != nil {
		return fmt.Errorf("Failed to set %s as executable: %s", luaPicturesLocalPath, err)
	}

	// Run lua_pictures and save stdout and stderr in files.
	stdoutFileName := *runID + ".output"
	stdoutFilePath := filepath.Join(os.TempDir(), stdoutFileName)
	stdoutFile, err := os.Create(stdoutFilePath)
	defer skutil.Close(stdoutFile)
	defer skutil.Remove(stdoutFilePath)
	if err != nil {
		return fmt.Errorf("Could not create %s: %s", stdoutFilePath, err)
	}
	stderrFileName := *runID + ".err"
	stderrFilePath := filepath.Join(os.TempDir(), stderrFileName)
	stderrFile, err := os.Create(stderrFilePath)
	defer skutil.Close(stderrFile)
	defer skutil.Remove(stderrFilePath)
	if err != nil {
		return fmt.Errorf("Could not create %s: %s", stderrFilePath, err)
	}
	args := []string{
		"--skpPath", localSkpsDir,
		"--luaFile", luaScriptLocalPath,
	}
	err = util.ExecuteCmd(
		ctx, luaPicturesLocalPath,
		args, []string{}, util.LUA_PICTURES_TIMEOUT, stdoutFile, stderrFile)
	if err != nil {
		return err
	}

	// Copy stdout and stderr files to Google Storage.
	skutil.LogErr(
		gs.UploadFile(stdoutFileName, os.TempDir(), filepath.Join(remoteDir, strconv.Itoa(*startRange), "outputs")))
	skutil.LogErr(
		gs.UploadFile(stderrFileName, os.TempDir(), filepath.Join(remoteDir, strconv.Itoa(*startRange), "errors")))

	return nil
}

func main() {
	retCode := 0
	if err := runLua(); err != nil {
		sklog.Errorf("Error while running lua scripts: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
