// Application that builds the specified repo and stores output to Google Storage.
package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct/go/worker_scripts/worker_common"
	"go.skia.org/infra/go/common"
	skutil "go.skia.org/infra/go/util"
)

var (
	runID             = flag.String("run_id", "", "The unique run id (typically requester + timestamp).")
	repoName          = flag.String("repo", "chromium", "The name of the repo this script should build and store in Google Storage.")
	hashes            = flag.String("hashes", "", "Comma separated list of hashes to checkout the specified repos to. Optional.")
	patches           = flag.String("patches", "", "Comma separated names of patches to apply to the specified repo. Optional.")
	uploadSingleBuild = flag.Bool("single_build", true, "Whether only a single build should be created and uploaded to Google Storage.")
	targetPlatform    = flag.String("target_platform", util.PLATFORM_LINUX, "The platform we want to build the specified repo for.")
	outDir            = flag.String("out", "", "The out directory where hashes will be stored.")
)

func main() {
	defer common.LogPanic()
	worker_common.Init()
	defer util.TimeTrack(time.Now(), "Building Repo")
	defer glog.Flush()

	if *outDir == "" {
		glog.Fatal("Must specify --out")
	}

	// Instantiate GsUtil object.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Fatal(err)
	}

	var remoteDirs []string
	if *repoName == "chromium" {
		applyPatches := false
		if *patches != "" {
			applyPatches = true
			for _, patch := range strings.Split(*patches, ",") {
				patchName := path.Base(patch)
				patchLocalPath := filepath.Join(os.TempDir(), patchName)
				if _, err := util.DownloadPatch(patchLocalPath, patch, gs); err != nil {
					glog.Fatal(err)
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
		pathToPyFiles := util.GetPathToPyFiles(!*worker_common.Local)
		chromiumHash, skiaHash, err := util.CreateChromiumBuildOnSwarming(*runID, *targetPlatform, chromiumHash, skiaHash, pathToPyFiles, applyPatches, *uploadSingleBuild)
		if err != nil {
			glog.Fatalf("Could not create chromium build: %s", err)
		}

		if !*uploadSingleBuild {
			remoteDirs = append(remoteDirs, fmt.Sprintf("try-%s-nopatch", util.ChromiumBuildDir(chromiumHash, skiaHash, *runID)))
		}
		remoteDirs = append(remoteDirs, fmt.Sprintf("try-%s-withpatch", util.ChromiumBuildDir(chromiumHash, skiaHash, *runID)))
	} else if *repoName == "pdfium" {
		// Sync PDFium and build pdfium_test binary.
		if err := util.SyncDir(util.PDFiumTreeDir, map[string]string{}); err != nil {
			glog.Fatalf("Could not sync PDFium: %s", err)
		}
		if err := util.BuildPDFium(); err != nil {
			glog.Fatalf("Could not build PDFium: %s", err)
		}
		// Copy pdfium_test to Google Storage.
		pdfiumLocalDir := path.Join(util.PDFiumTreeDir, "out", "Debug")
		pdfiumRemoteDir := path.Join(util.BINARIES_DIR_NAME, *runID)
		// Instantiate GsUtil object.
		gs, err := util.NewGsUtil(nil)
		if err != nil {
			glog.Fatal(err)
		}
		if err := gs.UploadFile(util.BINARY_PDFIUM_TEST, pdfiumLocalDir, pdfiumRemoteDir); err != nil {
			glog.Fatalf("Could not upload %s to %s: %s", util.BINARY_PDFIUM_TEST, pdfiumRemoteDir, err)
		}
		remoteDirs = append(remoteDirs, *runID)
	}

	// Record the remote dirs in the output file.
	buildDirsOutputFile := filepath.Join(*outDir, util.BUILD_OUTPUT_FILENAME)
	f, err := os.Create(buildDirsOutputFile)
	if err != nil {
		glog.Fatalf("Could not create %s: %s", buildDirsOutputFile, err)
	}
	defer skutil.Close(f)
	if _, err := f.WriteString(strings.Join(remoteDirs, ",")); err != nil {
		glog.Fatalf("Could not write to %s: %s", buildDirsOutputFile, err)
	}
}
