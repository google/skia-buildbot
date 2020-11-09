// Application that builds the specified repo and stores output to Google Storage.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

var (
	runID             = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	repoAndTarget     = flag.String("repo_and_target", "chromium", "The name of the repo and target this script should build and store in Google Storage. Eg: chromium")
	hashes            = flag.String("hashes", "", "Comma separated list of hashes to checkout the specified repos to. Optional.")
	patches           = flag.String("patches", "", "Comma separated names of patches to apply to the specified repo. Optional.")
	uploadSingleBuild = flag.Bool("single_build", true, "Whether only a single build should be created and uploaded to Google Storage.")
	targetPlatform    = flag.String("target_platform", util.PLATFORM_LINUX, "The platform we want to build the specified repo for.")
	outDir            = flag.String("out", "", "The out directory where hashes will be stored.")
)

func buildRepo() error {
	ctx := context.Background()
	httpClient, err := worker_common.Init(ctx, true /* useDepotTools */)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.TimeTrack(time.Now(), "Building Repo")
	defer sklog.Flush()

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

	var remoteDirs []string
	if *repoAndTarget == "chromium" {
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
		pathToPyFiles, err := util.GetPathToPyFiles(*worker_common.Local)
		if err != nil {
			return fmt.Errorf("Could not get path to py files: %s", err)
		}
		chromiumHash, skiaHash, err = util.CreateChromiumBuildOnSwarming(ctx, *runID, *targetPlatform, chromiumHash, skiaHash, pathToPyFiles, gitExec, applyPatches, *uploadSingleBuild)
		if err != nil {
			return fmt.Errorf("Could not create chromium build: %s", err)
		}

		if !*uploadSingleBuild {
			remoteDirs = append(remoteDirs, fmt.Sprintf("try-%s-nopatch", util.ChromiumBuildDir(chromiumHash, skiaHash, *runID)))
		}
		remoteDirs = append(remoteDirs, fmt.Sprintf("try-%s-withpatch", util.ChromiumBuildDir(chromiumHash, skiaHash, *runID)))
	} else {
		return fmt.Errorf("Unknown repo name specified to build_repo: %s", *repoAndTarget)
	}

	// Record the remote dirs in the output file.
	buildDirsOutputFile := filepath.Join(*outDir, util.BUILD_OUTPUT_FILENAME)
	f, err := os.Create(buildDirsOutputFile)
	if err != nil {
		return fmt.Errorf("Could not create %s: %s", buildDirsOutputFile, err)
	}
	defer skutil.Close(f)
	if _, err := f.WriteString(strings.Join(remoteDirs, ",")); err != nil {
		return fmt.Errorf("Could not write to %s: %s", buildDirsOutputFile, err)
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
