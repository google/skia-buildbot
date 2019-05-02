package bt_vcs

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore"
	gs_testutils "go.skia.org/infra/go/gitstore/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	vcs_testutils "go.skia.org/infra/go/vcsinfo/testutils"
)

const (
	skiaRepoURL  = "https://skia.googlesource.com/skia.git"
	localRepoURL = "https://example.com/local.git"
)

func TestVCSSuite(t *testing.T) {
	testutils.LargeTest(t)
	vcs, _, cleanup := setupVCSLocalRepo(t, "master")
	defer cleanup()

	// Run the VCS test suite.
	vcs_testutils.TestDisplay(t, vcs)
	vcs_testutils.TestFrom(t, vcs)
	vcs_testutils.TestByIndex(t, vcs)
	vcs_testutils.TestLastNIndex(t, vcs)
	vcs_testutils.TestRange(t, vcs)
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

	vcs_testutils.TestBranchInfo(t, vcs, branches)
}

func TestGetFile(t *testing.T) {
	testutils.LargeTest(t)
	gtRepo := gitiles.NewRepo(skiaRepoURL, "", nil)
	hash := "9be246ed747fd1b900013dd0596aed0b1a63a1fa"
	vcs := &BigTableVCS{
		repo: gtRepo,
	}
	_, err := vcs.GetFile(context.TODO(), "DEPS", hash)
	assert.NoError(t, err)
}

// setupVCSLocalRepo loads the test repo into a new GitStore and returns an instance of vcsinfo.VCS.
func setupVCSLocalRepo(t *testing.T, branch string) (vcsinfo.VCS, gitstore.GitStore, func()) {
	repoDir, cleanup := vcs_testutils.InitTempRepo()
	_, _, btgs := gs_testutils.SetupAndLoadBTGitStore(t, localRepoURL, repoDir, true)
	vcs, err := New(btgs, branch, nil, nil, 0)
	assert.NoError(t, err)
	return vcs, btgs, cleanup
}
