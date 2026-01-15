package repo_manager

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	gerrit_mocks "go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/git"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
)

const (
	noCheckoutParentRepo = "http://fake-parent-gitiles-url.git"
	noCheckoutChildRepo  = "http://fake-child-gitiles-url.git"

	noCheckoutParentHead  = "abcdef1234abcdef1234abcdef1234abcdef1234"
	noCheckoutTipRev      = "dddddddddddddddddddddddddddddddddddddddd"
	noCheckoutLastRollRev = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	noCheckoutGrandchildHashInParent = "abc1230000abc1230000abc1230000abc1230000"
	noCheckoutGrandchildHashInChild  = "def4560000def4560000def4560000def4560000"

	noCheckoutParentDepsContentTmpl = `deps = {
  "child": "%s@%s",
  "parent/dep": "https://grandchild-in-parent@%s",
}`
	noCheckoutChildDepsContentTmpl = `deps = {
  "child/dep": "https://grandchild-in-child@%s",
}`
)

var (
	noCheckoutChildCommits = []string{
		noCheckoutTipRev,
		"cccccccccccccccccccccccccccccccccccccccc",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		noCheckoutLastRollRev,
	}
	noCheckoutParentDepsContent = fmt.Sprintf(noCheckoutParentDepsContentTmpl, noCheckoutChildRepo, noCheckoutLastRollRev, noCheckoutGrandchildHashInParent)

	noCheckoutChildDepsContentsByHash = map[string]string{
		noCheckoutChildCommits[0]: fmt.Sprintf(noCheckoutChildDepsContentTmpl, noCheckoutGrandchildHashInChild),
		noCheckoutChildCommits[1]: fmt.Sprintf(noCheckoutChildDepsContentTmpl, noCheckoutGrandchildHashInChild),
		noCheckoutChildCommits[2]: fmt.Sprintf(noCheckoutChildDepsContentTmpl, noCheckoutGrandchildHashInChild),
		noCheckoutChildCommits[3]: fmt.Sprintf(noCheckoutChildDepsContentTmpl, noCheckoutGrandchildHashInChild),
	}
)

type hasAssertExpectations interface {
	AssertExpectations(mock.TestingT) bool
}

func assertExpectations(t *testing.T, mocks ...hasAssertExpectations) {
	for _, mock := range mocks {
		mock.AssertExpectations(t)
	}
}

func updateAndAssert(t *testing.T, rm RepoManager, mocks ...hasAssertExpectations) (*revision.Revision, *revision.Revision, []*revision.Revision) {
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(t.Context())
	require.NoError(t, err)
	assertExpectations(t, mocks...)
	return lastRollRev, tipRev, notRolledRevs
}

func setupNoCheckout(t *testing.T, cfg *config.ParentChildRepoManagerConfig) (*parentChildRepoManager, *gitiles_mocks.GitilesRepo, *gerrit_mocks.GerritInterface, *gitiles_mocks.GitilesRepo) {
	parentCfg := cfg.GetGitilesParent()
	p, parentGitiles, parentGerrit := parent.NewGitilesFileForTesting(t, parentCfg)
	childCfg := cfg.GetGitilesChild()
	c, childGitiles := child.NewGitilesForTesting(t, childCfg)

	// Create the RepoManager.
	rm := &parentChildRepoManager{
		Parent: p,
		Child:  c,
	}

	// Mock requests for Update().
	fileContents := map[string]string{deps_parser.DepsFileName: noCheckoutParentDepsContent}
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.GetGitilesParent(), noCheckoutParentHead, fileContents)
	child.MockGitiles_GetRevision(childGitiles, noCheckoutLastRollRev, noCheckoutLastRollRev, noCheckoutTipRev)
	child.MockGitiles_Update(t, childGitiles, cfg.GetGitilesChild(), noCheckoutLastRollRev, noCheckoutTipRev, noCheckoutChildCommits, noCheckoutChildDepsContentsByHash)

	// Update.
	_, _, _ = updateAndAssert(t, rm, parentGitiles, parentGerrit, childGitiles)
	return rm, parentGitiles, parentGerrit, childGitiles
}

func noCheckoutDEPSCfg() *config.ParentChildRepoManagerConfig {
	return &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_GitilesParent{
			GitilesParent: &config.GitilesParentConfig{
				Gitiles: &config.GitilesConfig{
					Branch:  git.MainBranch,
					RepoUrl: noCheckoutParentRepo,
				},
				Dep: &config.DependencyConfig{
					Primary: &config.VersionFileConfig{
						Id: noCheckoutChildRepo,
						File: []*config.VersionFileConfig_File{
							{Path: deps_parser.DepsFileName},
						},
					},
				},
				Gerrit: &config.GerritConfig{
					Url:     "https://fake-skia-review.googlesource.com",
					Project: "fake-gerrit-project",
					Config:  config.GerritConfig_CHROMIUM,
				},
			},
		},
		Child: &config.ParentChildRepoManagerConfig_GitilesChild{
			GitilesChild: &config.GitilesChildConfig{
				Gitiles: &config.GitilesConfig{
					Branch:  git.MainBranch,
					RepoUrl: noCheckoutChildRepo,
				},
			},
		},
	}
}

func TestNoCheckoutDEPSRepoManagerUpdate(t *testing.T) {
	cfg := noCheckoutDEPSCfg()
	rm, parentGitiles, parentGerrit, childGitiles := setupNoCheckout(t, cfg)

	// Mock requests for Update().
	fileContents := map[string]string{deps_parser.DepsFileName: noCheckoutParentDepsContent}
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.GetGitilesParent(), noCheckoutParentHead, fileContents)
	child.MockGitiles_GetRevision(childGitiles, noCheckoutLastRollRev, noCheckoutLastRollRev, noCheckoutTipRev)
	child.MockGitiles_Update(t, childGitiles, cfg.GetGitilesChild(), noCheckoutLastRollRev, noCheckoutTipRev, noCheckoutChildCommits, noCheckoutChildDepsContentsByHash)

	// Update.
	lastRollRev, tipRev, notRolledRevs := updateAndAssert(t, rm, parentGitiles, parentGerrit, childGitiles)
	require.Equal(t, noCheckoutChildCommits[len(noCheckoutChildCommits)-1], lastRollRev.Id)
	require.Equal(t, noCheckoutChildCommits[0], tipRev.Id)
	require.Equal(t, len(notRolledRevs), len(noCheckoutChildCommits)-1)
}

func testNoCheckoutDEPSRepoManagerCreateNewRoll(t *testing.T, cfg *config.ParentChildRepoManagerConfig) {
	rm, parentGitiles, parentGerrit, childGitiles := setupNoCheckout(t, cfg)

	// Mock requests for Update().
	oldContent := map[string]string{
		deps_parser.DepsFileName: noCheckoutParentDepsContent,
	}
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.GetGitilesParent(), noCheckoutParentHead, oldContent)
	child.MockGitiles_GetRevision(childGitiles, noCheckoutLastRollRev, noCheckoutLastRollRev, noCheckoutTipRev)
	child.MockGitiles_Update(t, childGitiles, cfg.GetGitilesChild(), noCheckoutLastRollRev, noCheckoutTipRev, noCheckoutChildCommits, noCheckoutChildDepsContentsByHash)

	// Update.
	lastRollRev, tipRev, notRolledRevs := updateAndAssert(t, rm, parentGitiles, parentGerrit, childGitiles)
	require.Equal(t, noCheckoutChildCommits[len(noCheckoutChildCommits)-1], lastRollRev.Id)
	require.Equal(t, noCheckoutChildCommits[0], tipRev.Id)

	// Mock requests for CreateNewRoll().
	newDepsContent := strings.Replace(oldContent[deps_parser.DepsFileName], lastRollRev.Id, tipRev.Id, -1)
	if len(cfg.GetGitilesParent().Dep.Transitive) > 0 {
		newDepsContent = strings.Replace(newDepsContent, noCheckoutGrandchildHashInParent, noCheckoutGrandchildHashInChild, -1)
	}
	newContent := map[string]string{
		deps_parser.DepsFileName: newDepsContent,
	}
	parent.MockGitilesFileForCreateNewRoll(parentGitiles, parentGerrit, cfg.GetGitilesParent(), noCheckoutParentHead, fakeCommitMsgMock, oldContent, newContent, fakeReviewers)

	issue, err := rm.CreateNewRoll(t.Context(), lastRollRev, tipRev, notRolledRevs, fakeReviewers, false, false, fakeCommitMsg)
	require.NoError(t, err)
	require.NotEqual(t, 0, issue)
	assertExpectations(t, parentGitiles, parentGerrit, childGitiles)
}

func TestNoCheckoutDEPSRepoManagerCreateNewRoll(t *testing.T) {
	cfg := noCheckoutDEPSCfg()
	testNoCheckoutDEPSRepoManagerCreateNewRoll(t, cfg)
}

func TestNoCheckoutDEPSRepoManagerCreateNewRollNoCQ(t *testing.T) {
	cfg := noCheckoutDEPSCfg()
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_GitilesParent).GitilesParent
	parentCfg.Gerrit.Config = config.GerritConfig_CHROMIUM_NO_CQ
	testNoCheckoutDEPSRepoManagerCreateNewRoll(t, cfg)
}

func TestNoCheckoutDEPSRepoManagerCreateNewRollTransitive(t *testing.T) {
	cfg := noCheckoutDEPSCfg()
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_GitilesParent).GitilesParent
	parentCfg.Dep.Transitive = []*config.TransitiveDepConfig{
		{
			Child: &config.VersionFileConfig{
				Id: "https://grandchild-in-child",
				File: []*config.VersionFileConfig_File{
					{Path: "DEPS"},
				},
			},
			Parent: &config.VersionFileConfig{
				Id: "https://grandchild-in-parent",
				File: []*config.VersionFileConfig_File{
					{Path: "DEPS"},
				},
			},
		},
	}
	childCfg := cfg.Child.(*config.ParentChildRepoManagerConfig_GitilesChild).GitilesChild
	childCfg.Gitiles.Dependencies = []*config.VersionFileConfig{
		{
			Id: "https://grandchild-in-child",
			File: []*config.VersionFileConfig_File{
				{Path: "DEPS"},
			},
		},
	}
	testNoCheckoutDEPSRepoManagerCreateNewRoll(t, cfg)
}
