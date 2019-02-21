package gitstore

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/testutils"
	vcstu "go.skia.org/infra/go/vcsinfo/testutils"
)

func TestVCSSuite(t *testing.T) {
	testutils.LargeTest(t)
	vcs, cleanup := setupVCSLocalRepo(t)
	defer cleanup()

	// Run the VCS test suite.
	vcstu.TestDisplay(t, vcs)
	vcstu.TestFrom(t, vcs)
	vcstu.TestByIndex(t, vcs)
	vcstu.TestLastNIndex(t, vcs)
	vcstu.TestRange(t, vcs)
}

func TestGetFile(t *testing.T) {
	testutils.LargeTest(t)
	gtRepo := gitiles.NewRepo(skiaRepoURL, "", nil)
	hash := "9be246ed747fd1b900013dd0596aed0b1a63a1fa"
	vcs := &btVCS{
		repo: gtRepo,
	}
	_, err := vcs.GetFile(context.TODO(), "DEPS", hash)
	assert.NoError(t, err)
}
