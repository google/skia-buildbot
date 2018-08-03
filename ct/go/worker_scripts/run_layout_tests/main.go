// Application that captures webpage archives on a CT worker and uploads it to
// Google Storage.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	skutil "go.skia.org/infra/go/util"
)

const (
	RUN_WEB_TESTS_BINARY         = "run_web_tests.py"
	RUN_WEB_TESTS_BINARY_TIMEOUT = time.Minute * 10
)

var (
	workerNum = flag.Int("worker_num", 1, "The assigned number of this worker. Should be unique across this CT run.")
	runID     = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
)

func captureArchives() error {
	worker_common.Init()
	if !*worker_common.Local {
		//defer util.CleanTmpDir()
	}
	defer util.TimeTrack(time.Now(), "Running Layout Tests")
	defer sklog.Flush()

	ctx := context.Background()

	// Create the local outputDir and define what the remote dir will be.
	outputDirName := *runID
	localOutputDir := filepath.Join(util.LayoutTestRunsDir, outputDirName)
	skutil.RemoveAll(localOutputDir)
	skutil.MkdirAll(localOutputDir, 0700)
	defer skutil.RemoveAll(localOutputDir)
	gsOutputDir := path.Join(util.SWARMING_DIR_NAME, util.LayoutTestRunsDir, outputDirName)

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		return err
	}

	outputFileName := fmt.Sprintf("worker%d.json", *workerNum)
	outputFilePath := path.Join(localOutputDir, outputFileName)
	runWebTestsBinary := filepath.Join(util.GetPathToLayoutTestBinaries(!*worker_common.Local), RUN_WEB_TESTS_BINARY)
	args := []string{
		"-t", "Release",
		"--json-test-results", outputFilePath,
	}
	env := []string{
		"DISPLAY=:0",
	}
	if err := util.ExecuteCmd(ctx, runWebTestsBinary, args, env, RUN_WEB_TESTS_BINARY_TIMEOUT, nil, nil); err != nil {
		return fmt.Errorf("Error when running %s: %s", runWebTestsBinary, err)
	}

	// Check to see if the output file was created.
	if _, err := os.Stat(outputFilePath); os.IsNotExist(err) {
		return fmt.Errorf("%s was not created after running %s: %s", outputFilePath, runWebTestsBinary, err)
	}

	if err := gs.UploadDir(localOutputDir, gsOutputDir, false); err != nil {
		return err
	}

	return nil
}

func main() {
	retCode := 0
	if err := captureArchives(); err != nil {
		sklog.Errorf("Error while capturing archives: %s", err)
		retCode = 255
	}
	os.Exit(retCode)
}
