package repo_manager

/*
   This file contains canned pre-upload steps for RepoManagers to use.
*/

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/go_install"
	"go.skia.org/infra/go/sklog"
)

const (
	recipesCfgPath = "infra/config/recipes.cfg"
	recipesPyPath  = "infra/bots/recipes.py"
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

	licensesThirdPartyFileName := "licenses_third_party"
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

	// Step8: Revert any change to pubspec.lock. This should be a temporary step
	// as described in https://bugs.chromium.org/p/skia/issues/detail?id=7730#c9
	if _, err := git.GitDir(licenseToolsDir).Git(ctx, "checkout", "--", "pubspec.lock"); err != nil {
		return err
	}

	sklog.Info("Done running flutter license scripts.")
	return nil
}

// NoCheckoutPreUploadStep is a function to be run to modify the roll,
// specifically designed for rollers which do not use local checkouts.
//
// Arguments are:
//   - Context
//   - gitiles.Repo instance for the child repo.
//   - Child repo commit we're rolling to.
//   - gitiles.Repo instance for the parent repo.
//   - Current parent repo commit.
// Return value is a map of file path to new file content.
type NoCheckoutPreUploadStep func(context.Context, *gitiles.Repo, string, *gitiles.Repo, string) (map[string]string, error)

// Return the NoCheckoutPreUploadStep with the given name.
func GetNoCheckoutPreUploadStep(s string) (NoCheckoutPreUploadStep, error) {
	rv, ok := map[string]NoCheckoutPreUploadStep{
		"RollRecipes": RollRecipes,
	}[s]
	if !ok {
		return nil, fmt.Errorf("No such pre-upload step: %s", s)
	}
	return rv, nil
}

// Return the NoCheckoutPreUploadSteps with the given names.
func GetNoCheckoutPreUploadSteps(steps []string) ([]NoCheckoutPreUploadStep, error) {
	rv := make([]NoCheckoutPreUploadStep, 0, len(steps))
	for _, s := range steps {
		step, err := GetNoCheckoutPreUploadStep(s)
		if err != nil {
			return nil, err
		}
		rv = append(rv, step)
	}
	return rv, nil
}

// TODO(borenet): Is there an upstream from which we could obtain this format?
type recipesCfg struct {
	ApiVersion            int                   `json:"api_version"`
	AutorollRecipeOptions autorollRecipeOptions `json:"autoroll_recipe_options"`
	Deps                  map[string]*recipeDep `json:"deps"`
	ProjectId             string                `json:"project_id"`
	RecipesPath           string                `json:"recipes_path"`
}

type autorollRecipeOptions struct {
	Nontrivial autorollRecipeOption `json:"nontrivial"`
	Trivial    autorollRecipeOption `json:"trivial"`
}

type autorollRecipeOption struct {
	AutomaticCommit       *bool    `json:"automatic_commit,omitempty"`
	AutomaticCommitDryRun *bool    `json:"automatic_commit_dry_run,omitempty"`
	TbrEmails             []string `json:"tbr_emails,omitempty"`
}

type recipeDep struct {
	Branch   string `json:"branch"`
	Revision string `json:"revision"`
	Url      string `json:"url"`
}

// Parse the recipes.cfg file content and return a map of dependency URL to
// commit hash.
func getRecipesCfg(repo *gitiles.Repo, ref string) (*recipesCfg, error) {
	var buf bytes.Buffer
	if err := repo.ReadFileAtRef(recipesCfgPath, ref, &buf); err != nil {
		return nil, err
	}
	cfg := new(recipesCfg)
	if err := json.NewDecoder(&buf).Decode(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// RollRecipes rolls recipe dependencies to make the parent repo deps match the
// child's.
func RollRecipes(_ context.Context, childRepo *gitiles.Repo, childCommit string, parentRepo *gitiles.Repo, parentCommit string) (map[string]string, error) {
	parentRecipesCfg, err := getRecipesCfg(parentRepo, parentCommit)
	if err != nil {
		return nil, fmt.Errorf("Failed to load recipes.cfg from parent: %s", err)
	}
	childRecipesCfg, err := getRecipesCfg(childRepo, childCommit)
	if err != nil {
		return nil, fmt.Errorf("Failed to load recipes.cfg from child: %s", err)
	}
	// Overwrite parent DEPS with those from the child.
	for name, dep := range childRecipesCfg.Deps {
		parentRecipesCfg.Deps[name] = dep
	}
	// Special case: if the child repo is in the dependency list, update its
	// revision to match the next roll rev.
	for name, dep := range parentRecipesCfg.Deps {
		if dep.Url == childRepo.URL {
			dep.Revision = childCommit
			break
		}
	}
	newRecipesCfgContents, err := json.MarshalIndent(parentRecipesCfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("Failed to write recipes.cfg JSON: %s", err)
	}

	// Also copy recipes.py.
	var buf bytes.Buffer
	if err := childRepo.ReadFileAtRef(recipesPyPath, childCommit, &buf); err != nil {
		return nil, fmt.Errorf("Failed to read recipes.py: %s", err)
	}
	newRecipesPyContents := buf.String()

	return map[string]string{
		recipesCfgPath: string(newRecipesCfgContents),
		recipesPyPath:  string(newRecipesPyContents),
	}, nil
}
