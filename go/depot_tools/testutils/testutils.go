package testutils

import (
	"context"
	"os"
	"path"
	"path/filepath"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
)

// GetDepotTools returns the path to depot_tools, syncing it if necessary.
func GetDepotTools(t sktest.TestingT, ctx context.Context) string {
	// Find the recipes cfg file, assuming we're in a checkout.
	root := testutils.GetRepoRoot(t)
	recipesCfgFile := filepath.Join(root, "infra/config/recipes.cfg")

	// Use a special location, for local testing.
	workdir := path.Join(os.TempDir(), "sktest_depot_tools")
	rv, err := depot_tools.GetDepotTools(ctx, workdir, recipesCfgFile)
	assert.NoError(t, err)
	return rv
}
