// Utility to create and manage chromium builds.
package util

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/util/zip"
)

const (
	// Use 14 chars here instead of the traditional 7 to reduce the chances of
	// ambiguous hashes while still leaving directory lengths reasonable.
	TRUNCATED_HASH_LENGTH = 14
)

var (
	TELEMETRY_ISOLATES_TARGET  = "ct_telemetry_perf_tests_without_chrome"
	TELEMETRY_ISOLATES_OUT_DIR = filepath.Join("out", "telemetry_isolates")
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

// CreateTelemetryIsolates creates an isolate of telemetry binaries that can be
// distributed to CT workers to run run_benchmark or record_wpr.
//
// ctx is the Context to use.
// runID is the unique id of the current run (typically requester + timestamp).
// targetPlatform is the platform the benchmark will run on (Android / Linux / Windows).
// chromiumHash is the hash the checkout should be synced to.
// pathToPyFiles is the local path to CT's python scripts. Eg: sync_skia_in_chrome.py.
// gitExec is the local path to the git binary.
// applyPatches if true looks for Chromium/Skia/V8/Catapult patches in the temp dir.
func CreateTelemetryIsolates(ctx context.Context, runID, targetPlatform, chromiumHash, pathToPyFiles, gitExec string, applyPatches bool) error {
	chromiumBuildDir, _ := filepath.Split(ChromiumSrcDir)
	MkdirAll(chromiumBuildDir, 0700)

	// Make sure we are starting from a clean slate before the sync.
	if err := ResetChromiumCheckout(ctx, ChromiumSrcDir, gitExec); err != nil {
		return fmt.Errorf("Could not reset the chromium checkout in %s: %s", chromiumBuildDir, err)
	}

	// Run chromium sync command using the specified chromium hash.
	// Construct path to the sync_skia_in_chrome python script.
	syncArgs := []string{
		filepath.Join(pathToPyFiles, "sync_skia_in_chrome.py"),
		"--destination=" + chromiumBuildDir,
		"--fetch_target=chromium",
		"--chrome_revision=" + chromiumHash,
		"--skia_revision=SKIA_REV_DEPS",
	}
	syncCommand := &exec.Command{
		Name:      BINARY_PYTHON,
		Args:      syncArgs,
		Timeout:   SYNC_SKIA_IN_CHROME_TIMEOUT,
		LogStdout: true,
		LogStderr: true,
		Env:       os.Environ(),
	}
	if _, err := exec.RunCommand(ctx, syncCommand); err != nil {
		return fmt.Errorf("There was an error checking out chromium %s: %s", chromiumHash, err)
	}

	if applyPatches {
		if err := applyRepoPatches(ctx, ChromiumSrcDir, runID, gitExec); err != nil {
			return fmt.Errorf("Could not apply patches in the chromium checkout in %s: %s", chromiumBuildDir, err)
		}
	}

	if err := os.Chdir(ChromiumSrcDir); err != nil {
		return fmt.Errorf("Could not chdir to %s: %s", ChromiumSrcDir, err)
	}
	// Make sure depot_tools is first in PATH.
	if err := os.Setenv("PATH", DepotToolsDir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		return fmt.Errorf("Could not set PATH env var: %s", err)
	}
	// Run "gn gen out/${TELEMETRY_ISOLATES_OUT_DIR} --args=..."
	gn_args := []string{}
	if targetPlatform == PLATFORM_WINDOWS {
		gn_args = append(gn_args, "enable_precompiled_headers=false")
	}
	if err := ExecuteCmd(ctx, filepath.Join(DepotToolsDir, "gn"), []string{"gen", TELEMETRY_ISOLATES_OUT_DIR, fmt.Sprintf("--args=%s", strings.Join(gn_args, " "))}, os.Environ(), GN_CHROMIUM_TIMEOUT, nil, nil); err != nil {
		return fmt.Errorf("Error while running gn: %s", err)
	}
	// Run "tools/mb/mb.py isolate ${TELEMETRY_ISOLATES_OUT_DIR} ${TELEMETRY_ISOLATES_TARGET}"
	mbArgs := []string{filepath.Join("tools", "mb", "mb.py"), "isolate", TELEMETRY_ISOLATES_OUT_DIR, TELEMETRY_ISOLATES_TARGET}
	mbCommand := &exec.Command{
		Name:      BINARY_VPYTHON3,
		Args:      mbArgs,
		Timeout:   NINJA_TIMEOUT,
		LogStdout: true,
		LogStderr: true,
		Env:       os.Environ(),
	}
	if _, err := exec.RunCommand(ctx, mbCommand); err != nil {
		return fmt.Errorf("Error while running mb.py isolate: %s", err)
	}

	return nil
}

// CreateChromiumBuildOnSwarming creates a chromium build using the specified arguments.

// runID is the unique id of the current run (typically requester + timestamp).
// targetPlatform is the platform the benchmark will run on (Android / Linux / Windows ).
// chromiumHash is the hash the checkout should be synced to. If not specified then
// Chromium's Tot hash is used.
// skiaHash is the hash the checkout should be synced to. If not specified then
// Skia's LKGR hash is used (the hash in Chromium's DEPS file).
// pathToPyFiles is the local path to CT's python scripts. Eg: sync_skia_in_chrome.py.
// gitExec is the local path to the git binary.
// applyPatches if true looks for Chromium/Skia/V8/Catapult patches in the temp dir and
// runs once with the patch applied and once without the patch applied.
// uploadSingleBuild if true does not upload a 2nd build of Chromium.
func CreateChromiumBuildOnSwarming(ctx context.Context, runID, targetPlatform, chromiumHash, skiaHash, pathToPyFiles, gitExec string, applyPatches, uploadSingleBuild bool) (string, string, error) {
	chromiumBuildDir, _ := filepath.Split(ChromiumSrcDir)
	// Determine which fetch target to use.
	var fetchTarget string
	if targetPlatform == PLATFORM_ANDROID {
		fetchTarget = "android"
	} else if targetPlatform == PLATFORM_LINUX || targetPlatform == PLATFORM_WINDOWS {
		fetchTarget = "chromium"
	} else {
		return "", "", fmt.Errorf("Unrecognized target_platform %s", targetPlatform)
	}
	MkdirAll(chromiumBuildDir, 0700)

	// Find which Chromium commit hash should be used.
	var err error
	if chromiumHash == "" {
		chromiumHash, err = GetChromiumHash(ctx, gitExec)
		if err != nil {
			return "", "", fmt.Errorf("Error while finding Chromium's Hash: %s", err)
		}
	}

	// Make sure we are starting from a clean slate before the sync.
	if err := ResetChromiumCheckout(ctx, filepath.Join(chromiumBuildDir, "src"), gitExec); err != nil {
		return "", "", fmt.Errorf("Could not reset the chromium checkout in %s: %s", chromiumBuildDir, err)
	}

	// Run chromium sync command using the above commit hashes.
	// Construct path to the sync_skia_in_chrome python script.
	syncArgs := []string{
		filepath.Join(pathToPyFiles, "sync_skia_in_chrome.py"),
		"--destination=" + chromiumBuildDir,
		"--fetch_target=" + fetchTarget,
		"--chrome_revision=" + chromiumHash,
		"--skia_revision=SKIA_REV_DEPS",
	}
	syncCommand := &exec.Command{
		Name: BINARY_PYTHON,
		Args: syncArgs,
		// The below is to bypass the blocking Android license agreement that shows
		// up sometimes for Android CT builds.
		Stdin:     strings.NewReader("y"),
		Timeout:   SYNC_SKIA_IN_CHROME_TIMEOUT,
		LogStdout: true,
		LogStderr: true,
		Env:       os.Environ(),
	}
	if _, err = exec.RunCommand(ctx, syncCommand); err != nil {
		return "", "", fmt.Errorf("There was an error checking out chromium %s + skia %s: %s", chromiumHash, skiaHash, err)
	}

	googleStorageDirName := ChromiumBuildDir(chromiumHash, skiaHash, runID)
	if applyPatches {
		if err := applyRepoPatches(ctx, filepath.Join(chromiumBuildDir, "src"), runID, gitExec); err != nil {
			return "", "", fmt.Errorf("Could not apply patches in the chromium checkout in %s: %s", chromiumBuildDir, err)
		}
		// Add "try" prefix and "withpatch" suffix.
		googleStorageDirName = fmt.Sprintf("try-%s-withpatch", googleStorageDirName)
	}
	// Build chromium.
	if err := buildChromium(ctx, chromiumBuildDir, targetPlatform); err != nil {
		return "", "", fmt.Errorf("There was an error building chromium %s + skia %s: %s", chromiumHash, skiaHash, err)
	}

	// Upload to Google Storage.
	gs, err := NewGcsUtil(nil)
	if err != nil {
		return "", "", fmt.Errorf("Could not create GCS object: %s", err)
	}
	if err := uploadChromiumBuild(filepath.Join(chromiumBuildDir, "src", "out", "Release"), path.Join(CHROMIUM_BUILDS_DIR_NAME, googleStorageDirName), targetPlatform, gs); err != nil {
		return "", "", fmt.Errorf("There was an error uploading the chromium build dir %s: %s", filepath.Join(chromiumBuildDir, "src", "out", "Release"), err)
	}

	// Create and upload another chromium build if the uploadSingleBuild flag is false. This build
	// will be created without applying any patches except the chromium_base_build patch if specified.
	if !uploadSingleBuild {
		// Make sure we are starting from a clean slate.
		if err := ResetChromiumCheckout(ctx, filepath.Join(chromiumBuildDir, "src"), gitExec); err != nil {
			return "", "", fmt.Errorf("Could not reset the chromium checkout in %s: %s", chromiumBuildDir, err)
		}
		if applyPatches {
			if err := applyBaseBuildRepoPatches(ctx, filepath.Join(chromiumBuildDir, "src"), runID, gitExec); err != nil {
				return "", "", fmt.Errorf("Could not apply patches in the chromium checkout in %s: %s", chromiumBuildDir, err)
			}
		}
		// Build chromium.
		if err := buildChromium(ctx, chromiumBuildDir, targetPlatform); err != nil {
			return "", "", fmt.Errorf("There was an error building chromium %s + skia %s: %s", chromiumHash, skiaHash, err)
		}
		// Upload to Google Storage.
		googleStorageDirName = fmt.Sprintf("try-%s-nopatch", ChromiumBuildDir(chromiumHash, skiaHash, runID))
		if err := uploadChromiumBuild(filepath.Join(chromiumBuildDir, "src", "out", "Release"), path.Join(CHROMIUM_BUILDS_DIR_NAME, googleStorageDirName), targetPlatform, gs); err != nil {
			return "", "", fmt.Errorf("There was an error uploaded the chromium build dir %s: %s", filepath.Join(chromiumBuildDir, "src", "out", "Release"), err)
		}
	}
	return getTruncatedHash(chromiumHash), getTruncatedHash(skiaHash), nil
}

// GetChromiumHash uses ls-remote to find and return Chromium's Tot commit hash.
func GetChromiumHash(ctx context.Context, gitExec string) (string, error) {
	stdoutBuf := bytes.Buffer{}
	totArgs := []string{"ls-remote", "https://chromium.googlesource.com/chromium/src.git", "--verify", git.DefaultRef}
	if err := ExecuteCmd(ctx, gitExec, totArgs, []string{}, GIT_LS_REMOTE_TIMEOUT, &stdoutBuf, nil); err != nil {
		return "", fmt.Errorf("Error while finding Chromium's ToT: %s", err)
	}
	tokens := strings.Split(stdoutBuf.String(), "\t")
	return tokens[0], nil
}

func uploadChromiumBuild(localOutDir, gsDir, targetPlatform string, gs *GcsUtil) error {
	MkdirAll(ChromiumBuildsDir, 0755)
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
		defer Rename(genTmpDir, genDir)

		objDir := filepath.Join(localOutDir, "obj")
		objTmpDir := filepath.Join(ChromiumBuildsDir, "obj")
		// Make sure the tmp dir is empty.
		util.RemoveAll(objTmpDir)
		if err := os.Rename(objDir, objTmpDir); err != nil {
			return fmt.Errorf("Could not rename obj dir: %s", err)
		}
		defer Rename(objTmpDir, objDir)
	}

	zipFilePath := filepath.Join(ChromiumBuildsDir, CHROMIUM_BUILD_ZIP_NAME)
	defer util.Remove(zipFilePath)
	if err := zip.Directory(zipFilePath, localUploadDir); err != nil {
		return fmt.Errorf("Error when zipping %s to %s: %s", localUploadDir, zipFilePath, err)
	}
	return gs.UploadFile(CHROMIUM_BUILD_ZIP_NAME, ChromiumBuildsDir, gsDir)
}

func buildChromium(ctx context.Context, chromiumDir, targetPlatform string) error {
	if err := os.Chdir(filepath.Join(chromiumDir, "src")); err != nil {
		return fmt.Errorf("Could not chdir to %s/src: %s", chromiumDir, err)
	}

	// Find the build target to use while building chromium.
	buildTarget := "chrome"
	if targetPlatform == "Android" {
		buildTarget = "chrome_public_apk"
	}

	gn_args := []string{"is_debug=false", "treat_warnings_as_errors=false", "dcheck_always_on=false", "is_official_build=true"}
	// Disable NaCl to speed up the build.
	gn_args = append(gn_args, "enable_nacl=false")
	// Produce enough debug info for stack traces but not line-by-line debugging.
	gn_args = append(gn_args, "symbol_level=1")
	if targetPlatform == "Android" {
		gn_args = append(gn_args, "target_os=\"android\"")
	}

	// Run "gn gen out/Release --args=...".
	if err := ExecuteCmd(ctx, "gn", []string{"gen", "out/Release", fmt.Sprintf("--args=%s", strings.Join(gn_args, " "))}, os.Environ(), GN_CHROMIUM_TIMEOUT, nil, nil); err != nil {
		return fmt.Errorf("Error while running gn: %s", err)
	}
	// Run "ninja -C out/Release -j100 ${build_target}".
	// Use the full system env while building chromium.
	args := []string{"-C", "out/Release", "-j100", buildTarget}
	return ExecuteCmd(ctx, filepath.Join(DepotToolsDir, "ninja"), args, os.Environ(), NINJA_TIMEOUT, nil, nil)
}

func getTruncatedHash(commitHash string) string {
	if len(commitHash) < TRUNCATED_HASH_LENGTH {
		return commitHash
	}
	return commitHash[0:TRUNCATED_HASH_LENGTH]
}

func ResetChromiumCheckout(ctx context.Context, chromiumSrcDir, gitExec string) error {
	// Clean up any left over lock files from sync errors of previous runs.
	err := os.Remove(filepath.Join(chromiumSrcDir, ".git", "index.lock"))
	if err != nil {
		sklog.Info("No index.lock file found.")
	}
	sklog.Info("Resetting Skia")
	skiaDir := filepath.Join(chromiumSrcDir, "third_party", "skia")
	if err := ResetCheckout(ctx, skiaDir, "HEAD", git.MainBranch, gitExec); err != nil {
		return fmt.Errorf("Could not reset Skia's checkout in %s: %s", skiaDir, err)
	}
	sklog.Info("Resetting V8")
	v8Dir := filepath.Join(chromiumSrcDir, "v8")
	// Detach the v8 checkout because of the problem described in
	// https://bugs.chromium.org/p/chromium/issues/detail?id=584742#c8
	if err := ResetCheckout(ctx, v8Dir, "HEAD", "--detach", gitExec); err != nil {
		return fmt.Errorf("Could not reset V8's checkout in %s: %s", v8Dir, err)
	}
	sklog.Info("Resetting Catapult")
	catapultDir := filepath.Join(chromiumSrcDir, RelativeCatapultSrcDir)
	if err := ResetCheckout(ctx, catapultDir, "HEAD", git.MainBranch, gitExec); err != nil {
		return fmt.Errorf("Could not reset Catapult's checkout in %s: %s", catapultDir, err)
	}
	sklog.Info("Resetting Chromium")
	if err := ResetCheckout(ctx, chromiumSrcDir, "HEAD", git.MainBranch, gitExec); err != nil {
		return fmt.Errorf("Could not reset Chromium's checkout in %s: %s", chromiumSrcDir, err)
	}
	return nil
}

func applyBaseBuildRepoPatches(ctx context.Context, chromiumSrcDir, runID, gitExec string) error {
	// Apply Chromium patch for the base build if it exists.
	chromiumPatch := filepath.Join(os.TempDir(), runID+".chromium_base_build.patch")
	if _, err := os.Stat(chromiumPatch); err == nil {
		chromiumPatchFile, _ := os.Open(chromiumPatch)
		chromiumPatchFileInfo, _ := chromiumPatchFile.Stat()
		if chromiumPatchFileInfo.Size() > 10 {
			if err := ApplyPatch(ctx, chromiumPatch, chromiumSrcDir, gitExec); err != nil {
				return fmt.Errorf("Could not apply Chromium's patch for the base build in %s: %s", chromiumSrcDir, err)
			}
		}
	}
	return nil
}

func applyRepoPatches(ctx context.Context, chromiumSrcDir, runID, gitExec string) error {
	// Apply Skia patch if it exists.
	skiaDir := filepath.Join(chromiumSrcDir, "third_party", "skia")
	skiaPatch := filepath.Join(os.TempDir(), runID+".skia.patch")
	if _, err := os.Stat(skiaPatch); err == nil {
		skiaPatchFile, _ := os.Open(skiaPatch)
		skiaPatchFileInfo, _ := skiaPatchFile.Stat()
		if skiaPatchFileInfo.Size() > 10 {
			if err := ApplyPatch(ctx, skiaPatch, skiaDir, gitExec); err != nil {
				return fmt.Errorf("Could not apply Skia's patch in %s: %s", skiaDir, err)
			}
		}
	}
	// Apply V8 patch if it exists.
	v8Dir := filepath.Join(chromiumSrcDir, "v8")
	v8Patch := filepath.Join(os.TempDir(), runID+".v8.patch")
	if _, err := os.Stat(v8Patch); err == nil {
		v8PatchFile, _ := os.Open(v8Patch)
		v8PatchFileInfo, _ := v8PatchFile.Stat()
		if v8PatchFileInfo.Size() > 10 {
			if err := ApplyPatch(ctx, v8Patch, v8Dir, gitExec); err != nil {
				return fmt.Errorf("Could not apply V8's patch in %s: %s", v8Dir, err)
			}
		}
	}
	// Apply Catapult patch if it exists.
	catapultDir := filepath.Join(chromiumSrcDir, "third_party", "catapult")
	catapultPatch := filepath.Join(os.TempDir(), runID+".catapult.patch")
	if _, err := os.Stat(catapultPatch); err == nil {
		catapultPatchFile, _ := os.Open(catapultPatch)
		catapultPatchFileInfo, _ := catapultPatchFile.Stat()
		if catapultPatchFileInfo.Size() > 10 {
			if err := ApplyPatch(ctx, catapultPatch, catapultDir, gitExec); err != nil {
				return fmt.Errorf("Could not apply Catapult's patch in %s: %s", catapultDir, err)
			}
		}
	}
	// Apply Chromium patch if it exists.
	chromiumPatch := filepath.Join(os.TempDir(), runID+".chromium.patch")
	if _, err := os.Stat(chromiumPatch); err == nil {
		chromiumPatchFile, _ := os.Open(chromiumPatch)
		chromiumPatchFileInfo, _ := chromiumPatchFile.Stat()
		if chromiumPatchFileInfo.Size() > 10 {
			if err := ApplyPatch(ctx, chromiumPatch, chromiumSrcDir, gitExec); err != nil {
				return fmt.Errorf("Could not apply Chromium's patch in %s: %s", chromiumSrcDir, err)
			}
		}
	}
	return nil
}

// UnInstallChromeAPK uninstalls the chrome APK from the Android device.
func UnInstallChromeAPK(ctx context.Context) {
	sklog.Info("UnInstalling the com.google.android.apps.chrome APK to start from a clean slate.")
	err := ExecuteCmd(ctx, BINARY_ADB, []string{"uninstall", CHROME_ANDROID_PACKAGE_NAME}, []string{},
		ADB_UNINSTALL_TIMEOUT, nil, nil)
	if err != nil {
		// It is ok if the APK was not already installed.
		// TODO(rmistry): Add a check to see if the APK exists after
		// failing to remove it. It is still exists then throw an error.
		sklog.Warningf("Could not uninstall the Chrome APK: %s", err)
	}
}

func InstallChromeAPK(ctx context.Context, chromiumApkPath string) error {
	// Install the APK on the Android device.
	sklog.Infof("Installing the APK at %s", chromiumApkPath)
	err := ExecuteCmd(ctx, BINARY_ADB, []string{"install", "-r", chromiumApkPath}, []string{},
		ADB_INSTALL_TIMEOUT, nil, nil)
	if err != nil {
		return fmt.Errorf("Could not install the chromium APK at %s: %s", chromiumApkPath, err)
	}
	return nil
}

func PatchesAreEmpty(patches []string) bool {
	for _, p := range patches {
		fInfo, err := os.Stat(p)
		if err == nil && fInfo.Size() > 10 {
			return false
		}
	}
	return true
}
