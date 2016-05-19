// Utility to create and manage chromium builds.
package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/util"

	"strings"
)

// Construct the name of a directory to store a chromium build. For generic clean builds, runID
// should be empty.
func ChromiumBuildDir(chromiumHash, skiaHash, runID string) string {
	if runID == "" {
		// Do not include the runID in the dir name if it is not specified.
		return fmt.Sprintf("%s-%s",
			getTruncatedHash(chromiumHash),
			getTruncatedHash(skiaHash))
	} else {
		return fmt.Sprintf("%s-%s-%s",
			getTruncatedHash(chromiumHash),
			getTruncatedHash(skiaHash),
			runID)
	}
}

// CreateChromiumBuild creates a chromium build using the specified arguments.

// runID is the unique id of the current run (typically requester + timestamp).
// targetPlatform is the platform the benchmark will run on (Android / Linux ).
// chromiumHash is the hash the checkout should be synced to. If not specified then
// Chromium's Tot hash is used.
// skiaHash is the hash the checkout should be synced to. If not specified then
// Skia's LKGR hash is used (the hash in Chromium's DEPS file).
// applyPatches if true looks for Chromium/Skia patches in the temp dir and
// runs once with the patch applied and once without the patch applied.
// uploadSingleBuild if true does not upload a 2nd build of Chromium.
func CreateChromiumBuild(runID, targetPlatform, chromiumHash, skiaHash string, applyPatches, uploadSingleBuild bool) (string, string, error) {
	// Determine which build dir and fetch target to use.
	var chromiumBuildDir, fetchTarget string
	if targetPlatform == "Android" {
		chromiumBuildDir = filepath.Join(ChromiumBuildsDir, "android_base")
		fetchTarget = "android"
	} else if targetPlatform == "Linux" {
		chromiumBuildDir = filepath.Join(ChromiumBuildsDir, "linux_base")
		fetchTarget = "chromium"
	} else {
		return "", "", fmt.Errorf("Unrecognized target_platform %s", targetPlatform)
	}
	util.MkdirAll(chromiumBuildDir, 0700)

	// Find which Chromium commit hash should be used.
	var err error
	if chromiumHash == "" {
		chromiumHash, err = getChromiumHash()
		if err != nil {
			return "", "", fmt.Errorf("Error while finding Chromium's Hash: %s", err)
		}
	}

	// Find which Skia commit hash should be used.
	if skiaHash == "" {
		skiaHash, err = buildskia.GetSkiaHash(nil)
		if err != nil {
			return "", "", fmt.Errorf("Error while finding Skia's Hash: %s", err)
		}
	}

	// Run chromium sync command using the above commit hashes.
	// Construct path to the sync_skia_in_chrome python script.
	_, currentFile, _, _ := runtime.Caller(0)
	pathToPyFiles := filepath.Join(
		filepath.Dir((filepath.Dir(filepath.Dir(currentFile)))),
		"py")
	syncArgs := []string{
		filepath.Join(pathToPyFiles, "sync_skia_in_chrome.py"),
		"--destination=" + chromiumBuildDir,
		"--fetch_target=" + fetchTarget,
		"--chrome_revision=" + chromiumHash,
		"--skia_revision=" + skiaHash,
	}
	err = ExecuteCmd("python", syncArgs, []string{}, SYNC_SKIA_IN_CHROME_TIMEOUT, nil, nil)
	if err != nil {
		glog.Warning("There was an error. Deleting base directory and trying again.")
		util.RemoveAll(chromiumBuildDir)
		util.MkdirAll(chromiumBuildDir, 0700)
		err := ExecuteCmd("python", syncArgs, []string{}, SYNC_SKIA_IN_CHROME_TIMEOUT, nil,
			nil)
		if err != nil {
			return "", "", fmt.Errorf("There was an error checking out chromium %s + skia %s: %s", chromiumHash, skiaHash, err)
		}
	}

	// Make sure we are starting from a clean slate.
	if err := resetChromiumCheckout(filepath.Join(chromiumBuildDir, "src")); err != nil {
		return "", "", fmt.Errorf("Could not reset the chromium checkout in %s: %s", chromiumBuildDir, err)
	}
	googleStorageDirName := ChromiumBuildDir(chromiumHash, skiaHash, runID)
	if applyPatches {
		if err := applyRepoPatches(filepath.Join(chromiumBuildDir, "src"), runID); err != nil {
			return "", "", fmt.Errorf("Could not apply patches in the chromium checkout in %s: %s", chromiumBuildDir, err)
		}
		// Add "try" prefix and "withpatch" suffix.
		googleStorageDirName = fmt.Sprintf("try-%s-withpatch", googleStorageDirName)
	}
	// Build chromium.
	if err := buildChromium(chromiumBuildDir, targetPlatform); err != nil {
		return "", "", fmt.Errorf("There was an error building chromium %s + skia %s: %s", chromiumHash, skiaHash, err)
	}

	// Upload to Google Storage.
	gs, err := NewGsUtil(nil)
	if err != nil {
		return "", "", fmt.Errorf("Could not create GS object: %s", err)
	}
	if err := uploadChromiumBuild(filepath.Join(chromiumBuildDir, "src", "out", "Release"), filepath.Join(CHROMIUM_BUILDS_DIR_NAME, googleStorageDirName), targetPlatform, gs); err != nil {
		return "", "", fmt.Errorf("There was an error uploaded the chromium build dir %s: %s", filepath.Join(chromiumBuildDir, "src", "out", "Release"), err)
	}

	// Create and upload another chromium build if the uploadSingleBuild flag is false. This build
	// will be created without applying patches.
	if !uploadSingleBuild {
		// Make sure we are starting from a clean slate.
		if err := resetChromiumCheckout(filepath.Join(chromiumBuildDir, "src")); err != nil {
			return "", "", fmt.Errorf("Could not reset the chromium checkout in %s: %s", chromiumBuildDir, err)
		}
		// Build chromium.
		if err := buildChromium(chromiumBuildDir, targetPlatform); err != nil {
			return "", "", fmt.Errorf("There was an error building chromium %s + skia %s: %s", chromiumHash, skiaHash, err)
		}
		// Upload to Google Storage.
		googleStorageDirName = fmt.Sprintf("try-%s-%s-%s-nopatch", getTruncatedHash(chromiumHash), getTruncatedHash(skiaHash), runID)
		if err := uploadChromiumBuild(filepath.Join(chromiumBuildDir, "src", "out", "Release"), filepath.Join(CHROMIUM_BUILDS_DIR_NAME, googleStorageDirName), targetPlatform, gs); err != nil {
			return "", "", fmt.Errorf("There was an error uploaded the chromium build dir %s: %s", filepath.Join(chromiumBuildDir, "src", "out", "Release"), err)
		}
	}
	return getTruncatedHash(chromiumHash), getTruncatedHash(skiaHash), nil
}

func getChromiumHash() (string, error) {
	// Find Chromium's Tot commit hash.
	stdoutFilePath := filepath.Join(os.TempDir(), "chromium-tot")
	stdoutFile, err := os.Create(stdoutFilePath)
	defer util.Close(stdoutFile)
	defer util.Remove(stdoutFilePath)
	if err != nil {
		return "", fmt.Errorf("Could not create %s: %s", stdoutFilePath, err)
	}
	totArgs := []string{"ls-remote", "https://chromium.googlesource.com/chromium/src.git", "--verify", "refs/heads/master"}
	err = ExecuteCmd(BINARY_GIT, totArgs, []string{}, GIT_LS_REMOTE_TIMEOUT, stdoutFile, nil)
	if err != nil {
		return "", fmt.Errorf("Error while finding Chromium's ToT: %s", err)
	}
	output, err := ioutil.ReadFile(stdoutFilePath)
	if err != nil {
		return "", fmt.Errorf("Cannot read %s: %s", stdoutFilePath, err)
	}
	tokens := strings.Split(string(output), "\t")
	return tokens[0], nil
}

func uploadChromiumBuild(localOutDir, gsDir, targetPlatform string, gs *GsUtil) error {
	localUploadDir := localOutDir
	if targetPlatform == "Android" {
		localUploadDir = filepath.Join(localUploadDir, "apks")
	} else {
		// Temporarily move the not needed large "gen" and "obj" directories so
		// that they do not get uploaded to Google Storage. Move them back after
		// the method completes.

		genDir := filepath.Join(localOutDir, "gen")
		genTmpDir := filepath.Join(ChromiumBuildsDir, "gen")
		// Make sure the tmp dir is empty.
		util.RemoveAll(genTmpDir)
		if err := os.Rename(genDir, genTmpDir); err != nil {
			return fmt.Errorf("Could not rename gen dir: %s", err)
		}
		defer util.Rename(genTmpDir, genDir)

		objDir := filepath.Join(localOutDir, "obj")
		objTmpDir := filepath.Join(ChromiumBuildsDir, "obj")
		// Make sure the tmp dir is empty.
		util.RemoveAll(objTmpDir)
		if err := os.Rename(objDir, objTmpDir); err != nil {
			return fmt.Errorf("Could not rename obj dir: %s", err)
		}
		defer util.Rename(objTmpDir, objDir)
	}
	return gs.UploadDir(localUploadDir, gsDir, true)
}

func buildChromium(chromiumDir, targetPlatform string) error {
	if err := os.Chdir(filepath.Join(chromiumDir, "src")); err != nil {
		return fmt.Errorf("Could not chdir to %s/src: %s", chromiumDir, err)
	}

	// Find the build target to use while building chromium.
	buildTarget := "chrome"
	if targetPlatform == "Android" {
		buildTarget = "chrome_public_apk"
	}

	// Start Goma's compiler proxy right before building the checkout.
	err := ExecuteCmd("python", []string{filepath.Join(GomaDir, "goma_ctl.py"), "start"},
		os.Environ(), GOMA_CTL_RESTART_TIMEOUT, nil, nil)
	if err != nil {
		return fmt.Errorf("Error while starting goma compiler proxy: %s", err)
	}

	// Run "gn gen out/Release --args=...".
	gn_args := []string{"is_debug=false", "use_goma=true", fmt.Sprintf("goma_dir=\"%s\"", GomaDir), "treat_warnings_as_errors=false"}
	if targetPlatform == "Android" {
		gn_args = append(gn_args, "target_os=\"android\"")
	}
	err = ExecuteCmd("gn", []string{"gen", "out/Release", fmt.Sprintf("--args=%s", strings.Join(gn_args, " "))}, os.Environ(), GN_CHROMIUM_TIMEOUT, nil, nil)
	if err != nil {
		return fmt.Errorf("Error while running gn: %s", err)
	}
	// Run "ninja -C out/Release -j100 ${build_target}".
	// Use the full system env while building chromium.
	args := []string{"-C", "out/Release", "-j100", buildTarget}
	return ExecuteCmd("ninja", args, os.Environ(), NINJA_TIMEOUT, nil, nil)
}

func getTruncatedHash(commitHash string) string {
	return commitHash[0:7]
}

func resetChromiumCheckout(chromiumSrcDir string) error {
	// Reset Skia.
	skiaDir := filepath.Join(chromiumSrcDir, "third_party", "skia")
	if err := ResetCheckout(skiaDir); err != nil {
		return fmt.Errorf("Could not reset Skia's checkout in %s: %s", skiaDir, err)
	}
	// Reset Chromium.
	if err := ResetCheckout(chromiumSrcDir); err != nil {
		return fmt.Errorf("Could not reset Chromium's checkout in %s: %s", chromiumSrcDir, err)
	}
	return nil
}

func applyRepoPatches(chromiumSrcDir, runID string) error {
	// Apply Skia patch if it exists.
	skiaDir := filepath.Join(chromiumSrcDir, "third_party", "skia")
	skiaPatch := filepath.Join(os.TempDir(), runID+".skia.patch")
	if _, err := os.Stat(skiaPatch); err == nil {
		skiaPatchFile, _ := os.Open(skiaPatch)
		skiaPatchFileInfo, _ := skiaPatchFile.Stat()
		if skiaPatchFileInfo.Size() > 10 {
			if err := ApplyPatch(skiaPatch, skiaDir); err != nil {
				return fmt.Errorf("Could not apply Skia's patch in %s: %s", skiaDir, err)
			}
		}
	}
	// Apply Chromium patch if it exists.
	chromiumPatch := filepath.Join(os.TempDir(), runID+".chromium.patch")
	if _, err := os.Stat(chromiumPatch); err == nil {
		chromiumPatchFile, _ := os.Open(chromiumPatch)
		chromiumPatchFileInfo, _ := chromiumPatchFile.Stat()
		if chromiumPatchFileInfo.Size() > 10 {
			if err := ApplyPatch(chromiumPatch, chromiumSrcDir); err != nil {
				return fmt.Errorf("Could not apply Chromium's patch in %s: %s", chromiumSrcDir, err)
			}
		}
	}
	return nil
}

func InstallChromeAPK(chromiumBuildName string) error {
	// Install the APK on the Android device.
	chromiumApk := filepath.Join(ChromiumBuildsDir, chromiumBuildName, ApkName)
	glog.Infof("Installing the APK at %s", chromiumApk)
	err := ExecuteCmd(BINARY_ADB, []string{"install", "-r", chromiumApk}, []string{},
		ADB_INSTALL_TIMEOUT, nil, nil)
	if err != nil {
		return fmt.Errorf("Could not install the chromium APK at %s: %s", chromiumBuildName, err)
	}
	return nil
}
