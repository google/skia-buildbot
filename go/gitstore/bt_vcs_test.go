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
	vcs, _, cleanup := setupVCSLocalRepo(t, "master")
	defer cleanup()

	// Run the VCS test suite.
	vcstu.TestDisplay(t, vcs)
	vcstu.TestFrom(t, vcs)
	vcstu.TestByIndex(t, vcs)
	vcstu.TestLastNIndex(t, vcs)
	vcstu.TestRange(t, vcs)
}

func TestBranchInfo(t *testing.T) {
	testutils.LargeTest(t)
	vcs, gitStore, cleanup := setupVCSLocalRepo(t, "")
	defer cleanup()

	ctx := context.TODO()
	branchPointers, err := gitStore.GetBranches(ctx)
	assert.NoError(t, err)
	branches := []string{}
	for branchName := range branchPointers {
		if branchName != "" {
			branches = append(branches, branchName)
		}
	}

	vcstu.TestBranchInfo(t, vcs, branches)
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
