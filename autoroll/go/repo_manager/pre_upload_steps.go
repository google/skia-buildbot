package repo_manager

/*
   This file contains canned pre-upload steps for RepoManagers to use.
*/

import (
	"context"
	"fmt"
	"os"
	"path"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/go_install"
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
// https://bugs.chromium.org/p/skia/issues/detail?id=7730#c6
/*
# make changes in license files... following instructions in the README here: https://github.com/flutter/engine/tree/master/tools/licenses
cd tools/licenses
../../../third_party/dart/tools/sdks/linux/dart-sdk/bin/pub get
rm -rf ../../../out/licenses
../../../third_party/dart/tools/sdks/linux/dart-sdk/bin/dart lib/main.dart --src ../../.. --out ../../../out/licenses --golden ../../travis/licenses_golden
cp ../../../out/licenses/licenses_third_party ../../travis/licenses_golden/licenses_third_party
(might fail if licenses_third_party does not exist!).

If any changes in licenses_golden/licenses_third_party:
../../../third_party/dart/tools/sdks/linux/dart-sdk/bin/dart lib/main.dart --release --src ../../.. --out ../../../out/licenses > ../../sky/packages/sky_engine/LICENSE

*/
func FlutterLicenseScripts(ctx context.Context, parentRepoDir string) error {
	fmt.Println("WHAT IS parentRepoDir in pre upload steps!")
	fmt.Println(parentRepoDir)
	//if _, err := exec.RunCwd(ctx, parentRepoDir, "git", "commit", "-a", "--amend", "--no-edit"); err != nil {
	//	return err
	//}
	return nil
	// TODO(borenet): Should we plumb through --local and --workdir?
	//goExc, goEnv, err := go_install.EnsureGo(false, os.TempDir())
	//if err != nil {
	//	return err
	//}
	//envSlice := make([]string, 0, len(goEnv))
	//for k, v := range goEnv {
	//	if k == "PATH" {
	//		v += ":" + os.Getenv("PATH")
	//	}
	//	envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	//}
	//if _, err := exec.RunCommand(ctx, &exec.Command{
	//	Name: goExc,
	//	Args: []string{"get", "-u", "go.skia.org/infra/..."},
	//	Env:  envSlice,
	//}); err != nil {
	//	return err
	//}
	//if _, err := exec.RunCommand(ctx, &exec.Command{
	//	Name: "make",
	//	Args: []string{"train"},
	//	Dir:  path.Join(parentRepoDir, "infra", "bots"),
	//	Env:  envSlice,
	//}); err != nil {
	//	return err
	//}
	//if _, err := exec.RunCwd(ctx, parentRepoDir, "git", "commit", "-a", "--amend", "--no-edit"); err != nil {
	//	return err
	//}
	//return nil
}
