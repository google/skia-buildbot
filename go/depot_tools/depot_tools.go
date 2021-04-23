package depot_tools

/*
   Utility for finding a depot_tools checkout.
*/

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sync"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/sklog"
)

const (
	DEPOT_TOOLS_TEST_ENV_VAR = "SKIABOT_TEST_DEPOT_TOOLS"
)

var (
	depotToolsMtx sync.Mutex

	version    string
	versionMtx sync.Mutex
)

// Env returns the environment used for depot_tools commands.
func Env(depotToolsPath string) []string {
	return []string{
		"DEPOT_TOOLS_UPDATE=0",
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
		fmt.Sprintf("LUCI_CONTEXT=%s", os.Getenv("LUCI_CONTEXT")),
		fmt.Sprintf("PATH=%s:%s", depotToolsPath, os.Getenv("PATH")),
	}
}

// Determine the desired depot_tools revision.
func FindVersion(recipesCfgFile string) (string, error) {
	versionMtx.Lock()
	defer versionMtx.Unlock()

	if version != "" {
		return version, nil
	}

	recipesCfg, err := recipe_cfg.ParseCfg(recipesCfgFile)
	if err != nil {
		return "", err
	}
	dep, ok := recipesCfg.Deps["depot_tools"]
	if !ok {
		return "", errors.New("No dependency found for depot_tools.")
	}
	version = dep.Revision
	return version, nil
}

// Sync syncs the depot_tools checkout to DEPOT_TOOLS_VERSION. Returns the
// location of the checkout or an error.
func Sync(ctx context.Context, workdir, recipesCfgFile string) (string, error) {
	version, err := FindVersion(recipesCfgFile)
	if err != nil {
		return "", err
	}
	sklog.Infof("Want depot_tools at %s", version)

	// Clone the repo if necessary.
	co, err := git.NewCheckout(ctx, common.REPO_DEPOT_TOOLS, workdir)
	if err != nil {
		return "", err
	}

	// Avoid doing any syncing if we already have the desired revision.
	hash, err := co.RevParse(ctx, "HEAD")
	if err != nil {
		return "", err
	}
	sklog.Infof("Have depot_tools at %s", hash)
	if hash == version {
		return co.Dir(), nil
	}

	// Sync the checkout into the desired state.
	if err := co.Fetch(ctx); err != nil {
		return "", fmt.Errorf("Failed to fetch repo in %s: %s", co.Dir(), err)
	}
	if err := co.Cleanup(ctx); err != nil {
		return "", fmt.Errorf("Failed to cleanup repo in %s: %s", co.Dir(), err)
	}
	if _, err := co.Git(ctx, "reset", "--hard", version); err != nil {
		return "", fmt.Errorf("Failed to reset repo in %s: %s", co.Dir(), err)
	}
	hash, err = co.RevParse(ctx, "HEAD")
	if err != nil {
		return "", err
	}
	if hash != version {
		return "", fmt.Errorf("Got incorrect depot_tools revision: %s, wanted %s", hash, version)
	}
	sklog.Infof("Successfully synced depot_tools to %s", version)
	return co.Dir(), nil
}

// GetDepotTools returns the path to depot_tools, syncing it into the given
// workdir if necessary.
func GetDepotTools(ctx context.Context, workdir, recipesCfgFile string) (string, error) {
	depotToolsMtx.Lock()
	defer depotToolsMtx.Unlock()

	sklog.Infof("Finding depot_tools...")

	// Check the environment. Bots may not have a full Git checkout, so
	// just return the dir.
	depotTools := os.Getenv(DEPOT_TOOLS_TEST_ENV_VAR)
	if depotTools != "" {
		sklog.Infof("Found %s environment variable.", DEPOT_TOOLS_TEST_ENV_VAR)
		if _, err := os.Stat(depotTools); err == nil {
			sklog.Infof("Found depot_tools in dir specified in env.")
			return depotTools, nil
		}
		sklog.Infof("depot_tools is not present in dir specified in env.")
		return "", fmt.Errorf("%s=%s but dir does not exist!", DEPOT_TOOLS_TEST_ENV_VAR, depotTools)
	}

	// If "gclient" is in PATH, then we know where to get depot_tools.
	gclient, err := exec.LookPath("gclient")
	if err == nil && gclient != "" {
		sklog.Infof("Found depot_tools in PATH")
		return Sync(ctx, path.Dir(path.Dir(gclient)), recipesCfgFile)
	}

	// Sync to the given workdir.
	sklog.Infof("Syncing depot_tools.")
	return Sync(ctx, workdir, recipesCfgFile)
}
