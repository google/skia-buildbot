package depot_tools

/*
   Utility for finding a depot_tools checkout.
*/

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sync"

	"go.skia.org/infra/go/depot_tools/deps"
	"go.skia.org/infra/go/skerr"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
)

const (
	DepotToolsTestEnvVar = "SKIABOT_TEST_DEPOT_TOOLS"
	DepotToolsURL        = "https://chromium.googlesource.com/chromium/tools/depot_tools.git"
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
func FindVersion() (string, error) {
	versionMtx.Lock()
	defer versionMtx.Unlock()

	if version != "" {
		return version, nil
	}

	dep, err := deps.Get(DepotToolsURL)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	version = dep.Version
	return version, nil
}

// Sync syncs the depot_tools checkout to DEPOT_TOOLS_VERSION. Returns the
// location of the checkout or an error.
func Sync(ctx context.Context, workdir string) (string, error) {
	version, err := FindVersion()
	if err != nil {
		return "", skerr.Wrap(err)
	}
	sklog.Infof("Want depot_tools at %s", version)

	// Clone the repo if necessary.
	co, err := git.NewCheckout(ctx, common.REPO_DEPOT_TOOLS, workdir)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	// Avoid doing any syncing if we already have the desired revision.
	hash, err := co.RevParse(ctx, "HEAD")
	if err != nil {
		return "", skerr.Wrap(err)
	}
	sklog.Infof("Have depot_tools at %s", hash)
	if hash == version {
		return co.Dir(), skerr.Wrap(err)
	}

	// Sync the checkout into the desired state.
	if err := co.Fetch(ctx); err != nil {
		return "", skerr.Wrapf(err, "Failed to fetch repo in %s", co.Dir())
	}
	if err := co.CleanupBranch(ctx, git.MainBranch); err != nil {
		return "", skerr.Wrapf(err, "Failed to cleanup repo in %s", co.Dir())
	}
	if _, err := co.Git(ctx, "reset", "--hard", version); err != nil {
		return "", skerr.Wrapf(err, "Failed to reset repo in %s", co.Dir())
	}
	hash, err = co.RevParse(ctx, "HEAD")
	if err != nil {
		return "", skerr.Wrap(err)
	}
	if hash != version {
		return "", skerr.Fmt("Got incorrect depot_tools revision: %s, wanted %s", hash, version)
	}
	sklog.Infof("Successfully synced depot_tools to %s", version)
	return co.Dir(), nil
}

// GetDepotTools returns the path to depot_tools, syncing it into the given
// workdir if necessary.
func GetDepotTools(ctx context.Context, workdir string) (string, error) {
	depotToolsMtx.Lock()
	defer depotToolsMtx.Unlock()

	sklog.Infof("Finding depot_tools...")

	// Check the environment. Bots may not have a full Git checkout, so
	// just return the dir.
	depotTools := os.Getenv(DepotToolsTestEnvVar)
	if depotTools != "" {
		sklog.Infof("Found %s environment variable.", DepotToolsTestEnvVar)
		if _, err := os.Stat(depotTools); err == nil {
			sklog.Infof("Found depot_tools in dir specified in env.")
			return depotTools, nil
		}
		sklog.Infof("depot_tools is not present in dir specified in env.")
		return "", skerr.Fmt("%s=%s but dir does not exist!", DepotToolsTestEnvVar, depotTools)
	}

	// If "gclient" is in PATH, then we know where to get depot_tools.
	gclient, err := exec.LookPath("gclient")
	if err == nil && gclient != "" {
		sklog.Infof("Found depot_tools in PATH: %s", gclient)
		return path.Dir(gclient), nil
	}

	// Sync to the given workdir.
	sklog.Infof("Syncing depot_tools.")
	dir, err := Sync(ctx, workdir)
	return dir, skerr.Wrap(err)
}
