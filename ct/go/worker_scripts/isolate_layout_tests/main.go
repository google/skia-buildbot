// Application that builds and isolates layout test isolates.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

var (
	runID        = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	chromiumHash = flag.String("chromium_hash", "", "Chromium repo will be synced to this hash if specified. Optional.")
	outDir       = flag.String("out", "", "The out directory where the isolate hash will be stored.")
)

func buildRepo() error {
	worker_common.Init()
	defer util.TimeTrack(time.Now(), "Isolating Layout Test Artifacts")
	defer sklog.Flush()

	ctx := context.Background()

	// Validate required arguments.
	if *runID == "" {
		return errors.New("Must specify --run_id")
	}
	if *outDir == "" {
		return errors.New("Must specify --out")
	}

	// Create the layout test isolates.
	pathToPyFiles := util.GetPathToPyFiles(!*worker_common.Local)
	if err := util.CreateLayoutTestIsolates(ctx, *runID, *chromiumHash, pathToPyFiles); err != nil {
		return fmt.Errorf("Could not create layout test isolates: %s", err)
	}

	buildOutDir := filepath.Join(util.ChromiumSrcDir, util.LAYOUT_TEST_ISOLATES_OUT_DIR)
	isolateFile := filepath.Join(buildOutDir, fmt.Sprintf("%s.isolate", util.LAYOUT_TEST_ISOLATES_TARGET))

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
	// Isolate layout test artifacts.
	isolateTask := &isolate.Task{
		BaseDir:     buildOutDir,
		Blacklist:   []string{},
		IsolateFile: isolateFile,
	}
	isolateTasks := []*isolate.Task{isolateTask}
	hashes, err := i.IsolateTasks(ctx, isolateTasks)
	if err != nil {
		return fmt.Errorf("Could not isolate layout test task: %s", err)
	}
	if len(hashes) != 1 {
		return fmt.Errorf("IsolateTasks returned incorrect number of hashes %d (expected 1)", len(hashes))
	}

	// Record the isolate hash in the output file.
	hashOutputFile := filepath.Join(*outDir, util.ISOLATE_LAYOUT_TEST_FILENAME)
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
