// Application that builds and isolates telemetry.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

var (
	runID          = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	chromiumHash   = flag.String("chromium_hash", "", "Chromium repo will be synced to this hash if specified. Optional.")
	patches        = flag.String("patches", "", "Comma separated names of patches to apply to the specified repo. Optional.")
	targetPlatform = flag.String("target_platform", util.PLATFORM_LINUX, "The platform we want to build for.")
	outDir         = flag.String("out", "", "The out directory where the isolate hash will be stored.")
)

func buildRepo() error {
	ctx := context.Background()
	httpClient, err := worker_common.Init(ctx, true /* useDepotTools */)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.TimeTrack(time.Now(), "Isolating Telemetry")
	defer sklog.Flush()

	// Validate required arguments.
	if *runID == "" {
		return errors.New("Must specify --run_id")
	}
	if *outDir == "" {
		return errors.New("Must specify --out")
	}

	// Find git exec.
	gitExec, err := git.Executable(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(httpClient)
	if err != nil {
		return err
	}

	// Apply patches if specified and create the telemetry isolates.
	applyPatches := false
	if *patches != "" {
		applyPatches = true
		for _, patch := range strings.Split(*patches, ",") {
			patchName := path.Base(patch)
			patchLocalPath := filepath.Join(os.TempDir(), patchName)
			if _, err := util.DownloadPatch(patchLocalPath, patch, gs); err != nil {
				return err
			}
		}
	}
	pathToPyFiles, err := util.GetPathToPyFiles(*worker_common.Local)
	if err != nil {
		return fmt.Errorf("Could not get path to py files: %s", err)
	}
	if err = util.CreateTelemetryIsolates(ctx, *runID, *targetPlatform, *chromiumHash, pathToPyFiles, gitExec, applyPatches); err != nil {
		return fmt.Errorf("Could not create telemetry isolates: %s", err)
	}

	buildOutDir := filepath.Join(util.ChromiumSrcDir, util.TELEMETRY_ISOLATES_OUT_DIR)
	isolateFile := filepath.Join(buildOutDir, fmt.Sprintf("%s.isolate", util.TELEMETRY_ISOLATES_TARGET))

	// Instantiate the isolate client.
	workDir, err := ioutil.TempDir(util.StorageDir, "isolate_")
	if err != nil {
		return fmt.Errorf("Could not create work dir: %s", err)
	}
	defer skutil.RemoveAll(workDir)
	// Create isolate client.
	i, err := isolate.NewClient(workDir, isolate.ISOLATE_SERVER_URL_PRIVATE)
	if err != nil {
		return fmt.Errorf("Failed to create isolate client: %s", err)
	}
	// Isolate telemetry artifacts.
	isolateTask := &isolate.Task{
		BaseDir:     buildOutDir,
		IsolateFile: isolateFile,
	}
	isolateTasks := []*isolate.Task{isolateTask}
	hashes, _, err := i.IsolateTasks(ctx, isolateTasks)
	if err != nil {
		return fmt.Errorf("Could not isolate telemetry task: %s", err)
	}
	if len(hashes) != 1 {
		return fmt.Errorf("IsolateTasks returned incorrect number of hashes %d (expected 1)", len(hashes))
	}

	// Record the isolate hash in the output file.
	hashOutputFile := filepath.Join(*outDir, util.ISOLATE_TELEMETRY_FILENAME)
	if err := ioutil.WriteFile(hashOutputFile, []byte(hashes[0]), os.ModePerm); err != nil {
		return fmt.Errorf("Could not write to %s: %s", hashOutputFile, err)
	}

	return nil
}

func main() {
	retCode := 0
	if err := buildRepo(); err != nil {
		sklog.Errorf("Error while building repo: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
