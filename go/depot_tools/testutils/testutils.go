package testutils

import (
	"os"
	"os/exec"
	"path"
	"sync"
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/sklog"
)

var (
	depotToolsMtx sync.Mutex
)

// Sync syncs depot_tools into the given dir.
func Sync(t *testing.T, dir string) string {
	d, err := depot_tools.Sync(dir)
	assert.NoError(t, err)
	return d
}

// GetDepotTools returns the path to depot_tools, syncing it if necessary.
func GetDepotTools(t *testing.T) string {
	depotToolsMtx.Lock()
	defer depotToolsMtx.Unlock()

	// Sync to a special location.
	workdir := path.Join(os.TempDir(), "sktest_depot_tools")
	if _, err := os.Stat(workdir); err == nil {
		return Sync(t, workdir)
	}

	// Check the environment. Bots may not have a full Git checkout, so
	// just return the dir.
	depotTools := os.Getenv("DEPOT_TOOLS")
	if depotTools != "" {
		if _, err := os.Stat(depotTools); err == nil {
			return depotTools
		}
		sklog.Errorf("DEPOT_TOOLS=%s but dir does not exist!", depotTools)
	}

	// If "gclient" is in PATH, then we know where to get depot_tools.
	gclient, err := exec.LookPath("gclient")
	if err == nil && gclient != "" {
		return Sync(t, path.Dir(path.Dir(gclient)))
	}

	// Fall back to the special location.
	return Sync(t, workdir)
}
