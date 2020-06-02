package repo_manager

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gerrit_common/gerrit_common_testutils"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	gerrit_testutils "go.skia.org/infra/go/gerrit/testutils"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func setupNoCheckout(t *testing.T, cfg *NoCheckoutDEPSRepoManagerConfig) (context.Context, string, *parentChildRepoManager, *git_testutils.GitBuilder, *git_testutils.GitBuilder, *gitiles_testutils.MockRepo, *gitiles_testutils.MockRepo, []string, *mockhttpclient.URLMock, *gerrit_testutils.MockGerrit, func()) {
	unittest.LargeTest(t)

	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	// Create child and parent repos.
	child := git_testutils.GitInit(t, context.Background())
	child.Add(context.Background(), "DEPS", `deps = {
  "child/dep": "https://grandchild-in-child@def4560000def4560000def4560000def4560000",
}`)
	child.Commit(context.Background())
	f := "somefile.txt"
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		childCommits = append(childCommits, child.CommitGen(context.Background(), f))
	}

	urlmock := mockhttpclient.NewURLMock()

	mockChild := gitiles_testutils.NewMockRepo(t, child.RepoUrl(), git.GitDir(child.Dir()), urlmock)

	parent := git_testutils.GitInit(t, context.Background())
	parent.Add(context.Background(), "DEPS", fmt.Sprintf(`deps = {
  "%s": "%s@%s",
  "parent/dep": "https://grandchild-in-parent@abc1230000abc1230000abc1230000abc1230000",
}`, childPath, child.RepoUrl(), childCommits[0]))
	parent.Commit(context.Background())

	mockParent := gitiles_testutils.NewMockRepo(t, parent.RepoUrl(), git.GitDir(parent.Dir()), urlmock)

	ctx := context.Background()

	mockGerrit := gerrit_common_testutils.SetupMockGerrit(t, urlmock)
	g := mockGerrit.Gerrit

	cfg.ChildRepo = child.RepoUrl()
	cfg.ParentRepo = parent.RepoUrl()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	// Create the RepoManager.
	rm, err := NewNoCheckoutDEPSRepoManager(ctx, cfg, setupRegistry(t), wd, g, recipesCfg, "fake.server.com", urlmock.Client(), gerritCR(t, g), false)
	require.NoError(t, err)

	// Mock requests for Update().
	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parent.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockGetCommit(ctx, "master")
	if len(cfg.TransitiveDeps) > 0 {
		mockChild.MockReadFile(ctx, "DEPS", childCommits[len(childCommits)-1])
	}
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
		if len(cfg.TransitiveDeps) > 0 {
			mockChild.MockReadFile(ctx, "DEPS", hash)
		}
	}
	// Update.
	_, _, _, err = rm.Update(ctx)
	require.NoError(t, err)
	require.True(t, urlmock.Empty(), strings.Join(urlmock.List(), "\n"))
	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
		require.True(t, urlmock.Empty(), strings.Join(urlmock.List(), "\n"))
	}
	return ctx, wd, rm, child, parent, mockChild, mockParent, childCommits, urlmock, mockGerrit, cleanup
}

func noCheckoutDEPSCfg(t *testing.T) *NoCheckoutDEPSRepoManagerConfig {
	return &NoCheckoutDEPSRepoManagerConfig{
		NoCheckoutRepoManagerConfig: NoCheckoutRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  masterBranchTmpl(t),
				ChildPath:    childPath,
				ParentBranch: masterBranchTmpl(t),
			},
		},
		Gerrit: &codereview.GerritConfig{
			URL:     gerrit_testutils.FakeGerritURL,
			Project: "fake-gerrit-project",
			Config:  codereview.GERRIT_CONFIG_CHROMIUM,
		},
	}
}

func TestNoCheckoutDEPSRepoManagerUpdate(t *testing.T) {
	cfg := noCheckoutDEPSCfg(t)
	ctx, _, rm, _, parentRepo, mockChild, mockParent, childCommits, _, _, cleanup := setupNoCheckout(t, cfg)
	defer cleanup()

	// Mock requests for Update().
	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockGetCommit(ctx, "master")
	if len(cfg.TransitiveDeps) > 0 {
		mockChild.MockReadFile(ctx, "DEPS", childCommits[len(childCommits)-1])
	}
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
		if len(cfg.TransitiveDeps) > 0 {
			mockChild.MockReadFile(ctx, "DEPS", hash)
		}
	}
	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)
	require.Equal(t, len(notRolledRevs), len(childCommits)-1)
}

func testNoCheckoutDEPSRepoManagerCreateNewRoll(t *testing.T, cfg *NoCheckoutDEPSRepoManagerConfig) {
	ctx, _, rm, childRepo, parentRepo, mockChild, mockParent, childCommits, _, mockGerrit, cleanup := setupNoCheckout(t, cfg)
	defer cleanup()

	// Mock requests for Update().
	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockGetCommit(ctx, "master")
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
	}
	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)

	// Mock the request to retrieve the DEPS file.
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)

	// Mock the CL upload.
	ci := mockGerrit.MockCreateChange(fakeCommitMsg, "master", parentMaster, map[string]string{
		deps_parser.DepsFileName: fmt.Sprintf(`deps = {
			"%s": "%s@%s",
			"parent/dep": "https://grandchild-in-parent@abc1230000abc1230000abc1230000abc1230000",
		  }`, childPath, childRepo.RepoUrl(), tipRev.Id),
	})

	// Mock the request to set the CQ.
	gerritCfg := codereview.GERRIT_CONFIGS[cfg.Gerrit.Config]
	if gerritCfg.HasCq {
		mockGerrit.MockPost(ci, "", gerritCfg.SetCqLabels, []string{"me@google.com"})
	} else {
		mockGerrit.MockPost(ci, "", gerritCfg.SelfApproveLabels, []string{"me@google.com"})
	}
	if !gerritCfg.HasCq {
		mockGerrit.MockSubmit(ci)
	}

	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, []string{"me@google.com"}, false, fakeCommitMsg)
	require.NoError(t, err)
	require.NotEqual(t, 0, issue)
}

func TestNoCheckoutDEPSRepoManagerCreateNewRoll(t *testing.T) {
	cfg := noCheckoutDEPSCfg(t)
	testNoCheckoutDEPSRepoManagerCreateNewRoll(t, cfg)
}

func TestNoCheckoutDEPSRepoManagerCreateNewRollNoCQ(t *testing.T) {
	cfg := noCheckoutDEPSCfg(t)
	cfg.Gerrit.Config = codereview.GERRIT_CONFIG_CHROMIUM_NO_CQ
	testNoCheckoutDEPSRepoManagerCreateNewRoll(t, cfg)
}

func TestNoCheckoutDEPSRepoManagerCreateNewRollTransitive(t *testing.T) {
	cfg := noCheckoutDEPSCfg(t)
	cfg.TransitiveDeps = []*version_file_common.TransitiveDepConfig{
		{
			Child: &version_file_common.VersionFileConfig{
				ID:   "https://grandchild-in-child",
				Path: "DEPS",
			},
			Parent: &version_file_common.VersionFileConfig{
				ID:   "https://grandchild-in-parent",
				Path: "DEPS",
			},
		},
	}
	ctx, _, rm, childRepo, parentRepo, mockChild, mockParent, childCommits, urlmock, mockGerrit, cleanup := setupNoCheckout(t, cfg)
	defer cleanup()

	// Mock requests for Update().
	mockParent.MockGetCommit(ctx, "master")
	parentMaster, err := git.GitDir(parentRepo.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, "DEPS", parentMaster)
	mockChild.MockGetCommit(ctx, "master")
	mockChild.MockReadFile(ctx, "DEPS", childCommits[len(childCommits)-1])
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))
	for _, hash := range childCommits {
		mockChild.MockGetCommit(ctx, hash)
		mockChild.MockReadFile(ctx, "DEPS", hash)
	}
	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.True(t, urlmock.Empty())
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)

	// Mock the request to retrieve the DEPS file.
	mockParent.MockReadFile(ctx, deps_parser.DepsFileName, parentMaster)

	// Mock the CL upload.
	ci := mockGerrit.MockCreateChange(fakeCommitMsg, "master", parentMaster, map[string]string{
		deps_parser.DepsFileName: fmt.Sprintf(`deps = {
  "%s": "%s@%s",
  "parent/dep": "https://grandchild-in-parent@abc1230000abc1230000abc1230000abc1230000",
}`, childPath, childRepo.RepoUrl(), tipRev.Id),
	})

	// Mock the request to set the CQ.
	gerritCfg := codereview.GERRIT_CONFIGS[cfg.Gerrit.Config]
	if gerritCfg.HasCq {
		mockGerrit.MockPost(ci, "", gerritCfg.SetCqLabels, []string{"me@google.com"})
	} else {
		mockGerrit.MockPost(ci, "", gerritCfg.SelfApproveLabels, []string{"me@google.com"})
	}
	if !gerritCfg.HasCq {
		mockGerrit.MockSubmit(ci)
	}

	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, []string{"me@google.com"}, false, fakeCommitMsg)
	require.NoError(t, err)
	require.NotEqual(t, 0, issue)
}
