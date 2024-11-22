package testutils

import (
	"context"
	"os"
	"path"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/sktest"
)

// GetDepotTools returns the path to depot_tools, syncing it if necessary.
func GetDepotTools(t sktest.TestingT, ctx context.Context) string {
	// Use a special location, for local testing.
	workdir := path.Join(os.TempDir(), "sktest_depot_tools")
	rv, err := depot_tools.GetDepotTools(ctx, workdir)
	require.NoError(t, err)
	return rv
}
