package repo_manager

/*
   This file contains canned pre-upload steps for RepoManagers to use.
*/

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"go.skia.org/infra/go/exec"
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
		"TrainInfra":            TrainInfra,
		"FlutterLicenseScripts": FlutterLicenseScripts,
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
	if _, err := exec.RunCwd(ctx, parentRepoDir, "git", "commit", "-a", "--amend", "--no-edit"); err != nil {
		return err
	}
	return nil
}

// Run the flutter license scripts as described in
// https://bugs.chromium.org/p/skia/issues/detail?id=7730#c6 and in
// https://github.com/flutter/engine/blob/master/tools/licenses/README.md
func FlutterLicenseScripts(ctx context.Context, parentRepoDir string) error {
	fmt.Println("WHAT IS parentRepoDir in pre upload steps!")
	fmt.Println(parentRepoDir)
	binariesPath := filepath.Join(parentRepoDir, "..", "third_party", "dart", "tools", "sdks", "linux", "dart-sdk", "bin")

	// Step1: Run pub get.
	licenseToolsDir := filepath.Join(parentRepoDir, "tools", "licenses")
	pubBinary := filepath.Join(binariesPath, "pub")
	if _, err := exec.RunCwd(ctx, licenseToolsDir, pubBinary, "get"); err != nil {
		return fmt.Errorf("Error when running pub get: %s", err)
	}

	// Step2: Clean out out/licenses
	licensesOutDir := filepath.Join(parentRepoDir, "out", "licenses")
	if err := os.RemoveAll(licensesOutDir); err != nil {
		return fmt.Errorf("Error when cleaning out/licenses: %s", err)
	}

	// Step3: Run dart license script.
	dartBinary := filepath.Join(binariesPath, "dart")
	licensesGoldenDir := filepath.Join(parentRepoDir, "travis", "licenses_golden")
	if _, err := exec.RunCwd(ctx, licenseToolsDir, dartBinary, "lib/main.dart", "--src", "../../..", "--out", licensesOutDir, "--golden", licensesGoldenDir); err != nil {
		return fmt.Errorf("Error when running dart license script: %s", err)
	}

	licensesThirdPartyFileName := "licenses_third_party"
	// Step4: Check to see if licenses_third_party was created in the out dir.
	if _, err := os.Stat(filepath.Join(licensesOutDir, licensesThirdPartyFileName)); err == nil {
		// Step5: Copy from out dir to goldens dir.
		if _, err := exec.RunCwd(ctx, licenseToolsDir, "cp", filepath.Join(licensesOutDir, licensesThirdPartyFileName), filepath.Join(licensesGoldenDir, licensesThirdPartyFileName)); err != nil {
			return fmt.Errorf("Error when copying licenses_third_party from out to golden dir: %s", err)
		}
		// Step6: Capture diff of licenses_golden/licenses_third_party.
		licensesDiffOutput, err := exec.RunCwd(ctx, licenseToolsDir, "git", "diff", filepath.Join(licensesGoldenDir, licensesThirdPartyFileName))
		if err != nil {
			return fmt.Errorf("Error when seeing diff of golden licenses_third_party: %s", err)
		}
		sklog.Infof("The licenses diff output is:\n%s", licensesDiffOutput)
		// TODO(rmistry): Run Step7 after detecting unexpected changes in licensesDiffOutput.
		//// Step7: Update sky_engine/LICENSE
		//if _, err := exec.RunCwd(ctx, licenseToolsDir, dartBinary, "lib/main.dart", "--release", "--src", "../../..", "--out", licensesOutDir, ">", "../../sky/packages/sky_engine/LICENSE"); err != nil {
		//	return fmt.Errorf("Error when running dart license script: %s", err)
		//}

	}

	// Step8: Revert any change to pubspec.lock. This should be a temporary step.
	if _, err := exec.RunCwd(ctx, licenseToolsDir, "git", "checkout", "--", "pubspec.lock"); err != nil {
		return err
	}

	// Step8: Commit all changed files.
	if _, err := exec.RunCwd(ctx, parentRepoDir, "git", "commit", "-a", "--amend", "--no-edit"); err != nil {
		return err
	}

	return nil
}
