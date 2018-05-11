// Application that builds and isolates telemetry.
package main

import (
	"context"
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
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	skutil "go.skia.org/infra/go/util"
)

var (
	runID   = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	hashes  = flag.String("hashes", "", "Comma separated list of hashes to checkout the specified repos to. Optional.")
	patches = flag.String("patches", "", "Comma separated names of patches to apply to the specified repo. Optional.")
	outDir  = flag.String("out", "", "The out directory where the isolate hash will be stored.")
)

func buildRepo() error {
	defer common.LogPanic()
	worker_common.Init()
	defer util.TimeTrack(time.Now(), "Isolating Telemetry")
	defer sklog.Flush()

	ctx := context.Background()

	//if *outDir == "" {
	//	return errors.New("Must specify --out")
	//}

	// Instantiate GcsUtil object.
	gs, err := util.NewGcsUtil(nil)
	if err != nil {
		return err
	}

	//var remoteDirs []string
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
	chromiumHash := *hashes // RENAME TO CHROMIUM_HASH!!
	// Things are read so lets skip for now!
	pathToPyFiles := util.GetPathToPyFiles(!*worker_common.Local)
	if err = util.CreateTelemetryIsolates(ctx, *runID, chromiumHash, pathToPyFiles, applyPatches); err != nil {
		return fmt.Errorf("Could not create chromium build: %s", err)
	}

	// Figure out how to use the isolate and isolated stuff!!!

	// hardcode path to the t
	// tools/swarming_client/isolate.py archive -I https://isolateserver.appspot.com -i out/Release/telemetry_perf_unittests.isolate -s out/Release/telemetry_perf_unittests.isolated --verbose

	buildOutDir := filepath.Join(util.ChromiumSrcDir, util.TELEMETRY_ISOLATES_OUT_DIR)
	isolateFile := filepath.Join(buildOutDir, fmt.Sprintf("%s.isolate", util.TELEMETRY_ISOLATES_TARGET))
	isolatedFile := filepath.Join(buildOutDir, fmt.Sprintf("%s.isolated", util.TELEMETRY_ISOLATES_TARGET))

	// Instantiate the swarming client.
	workDir, err := ioutil.TempDir(util.StorageDir, "swarming_work_")
	if err != nil {
		return fmt.Errorf("Could not get temp dir: %s", err)
	}

	i, err := isolate.NewClient(workDir, isolate.ISOLATE_SERVER_URL_PRIVATE)
	if err != nil {
		return fmt.Errorf("Failed to create isolate client: %s", err)
	}

	s, err := swarming.NewSwarmingClient(ctx, workDir, swarming.SWARMING_SERVER_PRIVATE, isolate.ISOLATE_SERVER_URL_PRIVATE)
	if err != nil {
		// Cleanup workdir.
		if err := os.RemoveAll(workDir); err != nil {
			sklog.Errorf("Could not cleanup swarming work dir: %s", err)
		}
		return fmt.Errorf("Could not instantiate swarming client: %s", err)
	}
	defer s.Cleanup()
	// YOU HAVE ISOLATED but not isoated.gen.json thingy...
	isolateTask := &isolate.Task{
		BaseDir:     buildOutDir, // what is this used for? what happens if I specify something custom.
		Blacklist:   []string{},
		IsolateFile: isolateFile,
	}
	//if isolateDep != "" {
	//	isolateTask.Deps = []string{isolateDep}
	//}
	isolateTasks := []*isolate.Task{isolateTask}
	hashes, err := i.IsolateTasks(ctx, isolateTasks)
	if err != nil {
		return fmt.Errorf("Could not isolate leasing task: %s", err)
	}
	if len(hashes) != 1 {
		return fmt.Errorf("IsolateTasks returned incorrect number of hashes %d (expected 1)", len(hashes))
	}
	fmt.Println("LOOK BELOW!!!")
	fmt.Println(hashes)

	fmt.Println(applyPatches)
	fmt.Println(chromiumHash)
	fmt.Println(isolateFile)
	fmt.Println(isolatedFile)

	// Record the remote dirs in the output file.
	hashOutputFile := filepath.Join(*outDir, util.ISOLATE_TELEMETRY_FILENAME)
	f, err := os.Create(hashOutputFile)
	if err != nil {
		return fmt.Errorf("Could not create %s: %s", hashOutputFile, err)
	}
	defer skutil.Close(f)
	if _, err := f.WriteString(hashes[0]); err != nil {
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
