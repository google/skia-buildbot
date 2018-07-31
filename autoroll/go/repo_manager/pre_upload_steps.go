package repo_manager

/*
   This file contains canned pre-upload steps for RepoManagers to use.
*/

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/go_install"
	"go.skia.org/infra/go/sklog"
)

// PreUploadStep is a function to be run after the roll is performed but before
// a CL is uploaded. The string parameter is the absolute path to the directory
// of the parent repo.
type PreUploadStep func(context.Context, string) error

// Return the PreUploadStep with the given name.
func GetPreUploadStep(s string) (PreUploadStep, error) {
	rv, ok := map[string]PreUploadStep{
		"TrainInfra":              TrainInfra,
		"CheckForEngineArtifacts": CheckForEngineArtifacts,
		"FlutterLicenseScripts":   FlutterLicenseScripts,
	}[s]
	if !ok {
		return nil, fmt.Errorf("No such pre-upload step: %s", s)
	}
	return rv, nil
}

// Return the PreUploadSteps with the given names.
func GetPreUploadSteps(steps []string) ([]PreUploadStep, error) {
	rv := make([]PreUploadStep, 0, len(steps))
	for _, s := range steps {
		step, err := GetPreUploadStep(s)
		if err != nil {
			return nil, err
		}
		rv = append(rv, step)
	}
	return rv, nil
}

// Train the infra expectations.
func TrainInfra(ctx context.Context, parentRepoDir string) error {
	// TODO(borenet): Should we plumb through --local and --workdir?
	goExc, goEnv, err := go_install.EnsureGo(false, os.TempDir())
	if err != nil {
		return err
	}
	envSlice := make([]string, 0, len(goEnv))
	for k, v := range goEnv {
		if k == "PATH" {
			v += ":" + os.Getenv("PATH")
		}
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Name: goExc,
		Args: []string{"get", "-u", "go.skia.org/infra/..."},
		Env:  envSlice,
	}); err != nil {
		return err
	}
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Name: "make",
		Args: []string{"train"},
		Dir:  path.Join(parentRepoDir, "infra", "bots"),
		Env:  envSlice,
	}); err != nil {
		return err
	}
	return nil
}

// Before flutter/engine.git can roll into flutter/flutter.git the engine's
// artifacts needs to be created via
// https://build.chromium.org/p/client.flutter/builders/Linux%20Engine
// This pre-upload step checks to see when the engine's artifacts show up
// in google storage and then proceeds. If the artifacts do not show up within
// a time limit then an error is returned.
func CheckForEngineArtifacts(ctx context.Context, parentRepoDir string) error {
	sklog.Info("[Flutter pre-upload step] Starting check for flutter engine artifacts.")
	engineHashBytes, err := ioutil.ReadFile(path.Join(parentRepoDir, "bin/internal/engine.version"))
	if err != nil {
		return err
	}
	engineHash := strings.TrimRight(string(engineHashBytes), "\n")
	gsPath := fmt.Sprintf("flutter/%s/sky_engine.zip", engineHash)
	flutterInfraBucket := "flutter_infra"
	gsPathWithBucket := fmt.Sprintf("gs://%s/%s", flutterInfraBucket, gsPath)

	s, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	gcsClient := gcs.NewGCSClient(s, flutterInfraBucket)

	sleepBetweenAttempts := 5 * time.Minute
	maxAttempts := 6

	foundFile := false
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		_, err := gcsClient.GetFileContents(ctx, gsPath)
		if err == nil {
			sklog.Infof("[Flutter pre-upload step] Found %s", gsPathWithBucket)
			foundFile = true
			break
		} else if err != storage.ErrObjectNotExist {
			sklog.Errorf("[Flutter pre-upload step] %s", err)
			return err
		}
		if attempt < maxAttempts {
			sklog.Infof("[Flutter pre-upload step] Attempt #%d: Could not find %s. Trying again after %s.", attempt, gsPathWithBucket, sleepBetweenAttempts)
			time.Sleep(sleepBetweenAttempts)
		}
	}

	if !foundFile {
		return fmt.Errorf("[Flutter pre-upload step] Could not find %s within %d minutes", gsPathWithBucket, int(sleepBetweenAttempts.Minutes())*maxAttempts)
	}
	return nil
}

// Run the flutter license scripts as described in
// https://bugs.chromium.org/p/skia/issues/detail?id=7730#c6 and in
// https://github.com/flutter/engine/blob/master/tools/licenses/README.md
func FlutterLicenseScripts(ctx context.Context, parentRepoDir string) error {
	sklog.Info("Running flutter license scripts.")
	binariesPath := filepath.Join(parentRepoDir, "..", "third_party", "dart", "tools", "sdks", "linux", "dart-sdk", "bin")

	// Step1: Run pub get.
	licenseToolsDir := filepath.Join(parentRepoDir, "tools", "licenses")
	pubBinary := filepath.Join(binariesPath, "pub")
	sklog.Info("Running put get.")
	if _, err := exec.RunCwd(ctx, licenseToolsDir, pubBinary, "get"); err != nil {
		return fmt.Errorf("Error when running pub get: %s", err)
	}

	// Step2: Clean out out/licenses. Things fail without this step.
	licensesOutDir := filepath.Join(parentRepoDir, "..", "out", "licenses")
	if err := os.RemoveAll(licensesOutDir); err != nil {
		return fmt.Errorf("Error when cleaning out/licenses: %s", err)
	}

	// Step3: Run dart license script to create new licenses.
	dartBinary := filepath.Join(binariesPath, "dart")
	licensesGoldenDir := filepath.Join(parentRepoDir, "travis", "licenses_golden")
	licenseCmd := []string{dartBinary, "lib/main.dart", "--src", "../../..", "--out", licensesOutDir, "--golden", licensesGoldenDir}
	sklog.Infof("Running %s", licenseCmd)
	if _, err := exec.RunCwd(ctx, licenseToolsDir, licenseCmd...); err != nil {
		return fmt.Errorf("Error when running dart license script: %s", err)
	}

	licensesThirdPartyFileName := "licenses_skia"
	// Step4: Check to see if licenses_third_party was created in the out dir.
	//        It will be created if the third_party hash changes.
	if _, err := os.Stat(filepath.Join(licensesOutDir, licensesThirdPartyFileName)); err == nil {
		sklog.Infof("Found %s", licensesThirdPartyFileName)

		// Step5: Copy from out dir to goldens dir. This is required for updating
		//        the release file in sky_engine/LICENSE.
		if _, err := exec.RunCwd(ctx, licenseToolsDir, "cp", filepath.Join(licensesOutDir, licensesThirdPartyFileName), filepath.Join(licensesGoldenDir, licensesThirdPartyFileName)); err != nil {
			return fmt.Errorf("Error when copying licenses_third_party from out to golden dir: %s", err)
		}
		// Step6: Capture diff of licenses_golden/licenses_third_party.
		licensesDiffOutput, err := git.GitDir(licenseToolsDir).Git(ctx, "diff", "--no-ext-diff", filepath.Join(licensesGoldenDir, licensesThirdPartyFileName))
		if err != nil {
			return fmt.Errorf("Error when seeing diff of golden licenses_third_party: %s", err)
		}
		sklog.Infof("The licenses diff output is:\n%s", licensesDiffOutput)

		// Step7: Update sky_engine/LICENSE. It should always be run
		// according to https://github.com/flutter/engine/pull/4959#issuecomment-380222322
		updateLicenseCmd := []string{dartBinary, "lib/main.dart", "--release", "--src", "../../..", "--out", licensesOutDir}
		sklog.Infof("Running %s", updateLicenseCmd)
		releasesLicensePath := filepath.Join(parentRepoDir, "sky", "packages", "sky_engine", "LICENSE")
		outFile, err := os.Create(releasesLicensePath)
		if err != nil {
			return fmt.Errorf("Could not open %s: %s", releasesLicensePath, err)
		}
		if err := exec.Run(ctx, &exec.Command{
			Dir:    licenseToolsDir,
			Name:   dartBinary,
			Args:   []string{"lib/main.dart", "--release", "--src", "../../..", "--out", licensesOutDir},
			Stdout: outFile,
		}); err != nil {
			return fmt.Errorf("Error when running dart license script: %s", err)
		}
	}

	sklog.Info("Done running flutter license scripts.")
	return nil
}
