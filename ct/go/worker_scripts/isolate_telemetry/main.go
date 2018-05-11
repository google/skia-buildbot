// Application that builds and isolates telemetry.
package main

import (
	"context"
	"io/ioutil"
	//"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/swarming"
	//skutil "go.skia.org/infra/go/util"
)

var (
	runID   = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	hashes  = flag.String("hashes", "", "Comma separated list of hashes to checkout the specified repos to. Optional.")
	patches = flag.String("patches", "", "Comma separated names of patches to apply to the specified repo. Optional.")
	//outDir  = flag.String("out", "", "The out directory where hashes will be stored.")
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
	chromiumHash := ""
	skiaHash := ""
	if *hashes != "" {
		tokens := strings.Split(*hashes, ",")
		if len(tokens) > 0 {
			chromiumHash = tokens[0]
			if len(tokens) == 2 {
				skiaHash = tokens[1]
			}
		}
	}
	// Things are read so lets skip for now!
	//pathToPyFiles := util.GetPathToPyFiles(!*worker_common.Local)
	//chromiumHash, skiaHash, err = util.CreateTelemetryIsolates(ctx, *runID, chromiumHash, skiaHash, pathToPyFiles, applyPatches)
	//if err != nil {
	//	return fmt.Errorf("Could not create chromium build: %s", err)
	//}

	// Figure out how to use the isolate and isolated stuff!!!

	// hardcode path to the t
	// tools/swarming_client/isolate.py archive -I https://isolateserver.appspot.com -i out/Release/telemetry_perf_unittests.isolate -s out/Release/telemetry_perf_unittests.isolated --verbose

	isolateFile := filepath.Join(util.ChromiumSrcDir, "out", "Release", "telemetry_perf_unittests.isolate")
	isolatedFile := filepath.Join(util.ChromiumSrcDir, "out", "Release", "telemetry_perf_unittests.isolated")

	// Instantiate the swarming client.
	workDir, err := ioutil.TempDir(util.StorageDir, "swarming_work_")
	if err != nil {
		return fmt.Errorf("Could not get temp dir: %s", err)
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

	// Create isolated.gen.json.
	// Get path to isolate files.
	//_, currentFile, _, _ := runtime.Caller(0)
	//pathToIsolates := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(currentFile))), "isolates")
	//isolateArgs := map[string]string{
	//	"RUN_ID":          runID,
	//	"REPO":            repo,
	//	"HASHES":          strings.Join(hashes, ","),
	//	"PATCHES":         strings.Join(patches, ","),
	//	"SINGLE_BUILD":    strconv.FormatBool(singleBuild),
	//	"TARGET_PLATFORM": targetPlatform,
	//}
	//genJSON, err := s.CreateIsolatedGenJSON(path.Join(pathToIsolates, util.BUILD_REPO_ISOLATE), s.WorkDir, "linux", taskName, isolateArgs, []string{})
	//if err != nil {
	//	return fmt.Errorf("Could not create isolated.gen.json for task %s: %s", taskName, err)
	//}
	//// Batcharchive the task.
	//tasksToHashes, err := s.BatchArchiveTargets(ctx, []string{genJSON}, util.BATCHARCHIVE_TIMEOUT)
	//if err != nil {
	//	return fmt.Errorf("Could not batch archive target: %s", err)
	//}

	fmt.Println(applyPatches)
	fmt.Println(chromiumHash)
	fmt.Println(skiaHash)
	fmt.Println(isolateFile)
	fmt.Println(isolatedFile)
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
