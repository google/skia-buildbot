package child

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vfs"
)

// TODO(borenet): Split up the tests in github_cipd_deps_repo_manager_test.go
// and move the relevant parts here.

func TestCIPDChild_VCS(t *testing.T) {
	unittest.LargeTest(t)
	// This is a manual test because it downloads a real CIPD package from
	// the production server. A mock isn't going to do us any good, since we
	// want to ensure that we actually get the correct package version
	// installed to the correct location.
	unittest.ManualTest(t)

	// Configuration.
	const pkgName = "skia/bots/svg"
	const pkgTag = "version:9"
	const pkgVer = "c2784ea640f0c9089ab3ea53775e2d24e1c89f63"

	// Setup.
	ctx := context.Background()
	cfg := config.CIPDChildConfig{
		Name: pkgName,
		Tag:  pkgTag,
	}
	ts, err := auth.NewDefaultTokenSource(true, auth.SCOPE_USERINFO_EMAIL)
	require.NoError(t, err)
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	c, err := NewCIPD(ctx, &cfg, client, wd)
	require.NoError(t, err)

	// Download.
	rev := &revision.Revision{Id: pkgVer}
	fs, err := c.VFS(ctx, rev)
	require.NoError(t, err)

	// Verify that we have the correct contents.
	topContents, err := vfs.ReadDir(ctx, fs, ".")
	require.NoError(t, err)
	for _, fi := range topContents {
		require.True(t, strings.HasSuffix(fi.Name(), ".svg"))
	}
	require.Len(t, topContents, 72)
}
