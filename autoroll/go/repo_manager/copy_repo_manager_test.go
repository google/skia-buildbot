package repo_manager

import (
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	gerrit_mocks "go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/git"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/testutils"
)

func copyCfg() *config.ParentChildRepoManagerConfig {
	return &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_CopyParent{
			CopyParent: &config.CopyParentConfig{
				Gitiles: &config.GitilesParentConfig{
					Gitiles: &config.GitilesConfig{
						Branch:  git.MainBranch,
						RepoUrl: "http://fake.parent",
					},
					Dep: &config.DependencyConfig{
						Primary: &config.VersionFileConfig{
							Id: "http://fake.child",
							File: []*config.VersionFileConfig_File{
								{Path: filepath.Join(childPath, "version.sha1")},
							},
						},
					},
					Gerrit: &config.GerritConfig{
						Url:     "https://fake-skia-review.googlesource.com",
						Project: "fake-gerrit-project",
						Config:  config.GerritConfig_CHROMIUM,
					},
				},
				Copies: []*config.CopyParentConfig_CopyEntry{
					{
						SrcRelPath: path.Join("child-dir", "child-file.txt"),
						DstRelPath: path.Join(childPath, "parent-file.txt"),
					},
					{
						SrcRelPath: path.Join("child-dir", "child-subdir"),
						DstRelPath: path.Join(childPath, "parent-dir"),
					},
				},
			},
		},
		Child: &config.ParentChildRepoManagerConfig_GitilesChild{
			GitilesChild: &config.GitilesChildConfig{
				Gitiles: &config.GitilesConfig{
					Branch:  git.MainBranch,
					RepoUrl: "todo.git",
				},
			},
		},
	}
}

func setupCopy(t *testing.T) (*parentChildRepoManager, *gitiles_mocks.GitilesRepo, *gerrit_mocks.GerritInterface, *gitiles_mocks.GitilesRepo) {
	cfg := copyCfg()
	childCfg := cfg.GetGitilesChild()
	c, childGitiles := child.NewGitilesForTesting(t, childCfg)
	parentCfg := cfg.GetCopyParent()
	p, parentGitiles, parentGerrit := parent.NewCopyForTesting(t, parentCfg, c)

	// Create the RepoManager.
	rm := &parentChildRepoManager{
		Parent: p,
		Child:  c,
	}

	// Mock requests for Update.
	fileContents := map[string]string{
		parentCfg.Gitiles.Dep.Primary.File[0].Path: noCheckoutLastRollRev + "\n",
	}
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.GetCopyParent().Gitiles, noCheckoutParentHead, fileContents)
	child.MockGitiles_GetRevision(childGitiles, noCheckoutLastRollRev, noCheckoutLastRollRev, noCheckoutTipRev)
	child.MockGitiles_Update(t, childGitiles, cfg.GetGitilesChild(), noCheckoutLastRollRev, noCheckoutTipRev, noCheckoutChildCommits, noCheckoutChildDepsContentsByHash)

	// Update.
	_, _, _ = updateAndAssert(t, rm, parentGitiles, parentGerrit, childGitiles)
	return rm, parentGitiles, parentGerrit, childGitiles
}

// TestCopyRepoManager tests all aspects of the CopyRepoManager.
func TestCopyRepoManager(t *testing.T) {
	cfg := copyCfg()
	rm, parentGitiles, parentGerrit, childGitiles := setupCopy(t)

	// Mock requests for Update.
	fileContents := map[string]string{
		cfg.GetCopyParent().Gitiles.Dep.Primary.File[0].Path: noCheckoutLastRollRev + "\n",
	}
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.GetCopyParent().Gitiles, noCheckoutParentHead, fileContents)
	child.MockGitiles_GetRevision(childGitiles, noCheckoutLastRollRev, noCheckoutLastRollRev, noCheckoutTipRev)
	child.MockGitiles_Update(t, childGitiles, cfg.GetGitilesChild(), noCheckoutLastRollRev, noCheckoutTipRev, noCheckoutChildCommits, noCheckoutChildDepsContentsByHash)

	// Update.
	lastRollRev, tipRev, notRolledRevs := updateAndAssert(t, rm, parentGitiles, parentGerrit, childGitiles)
	require.Equal(t, noCheckoutLastRollRev, lastRollRev.Id)
	require.Equal(t, noCheckoutTipRev, tipRev.Id)
	require.Equal(t, len(noCheckoutChildCommits)-1, len(notRolledRevs))

}

func TestCopyRepoManagerCreateNewRoll_ChildContentsCopiedIntoParentFiles(t *testing.T) {
	cfg := copyCfg()
	rm, parentGitiles, parentGerrit, childGitiles := setupCopy(t)

	// Mock requests for Update.
	parentCfg := cfg.GetCopyParent()
	pinPath := parentCfg.Gitiles.Dep.Primary.File[0].Path
	oldContent := map[string]string{
		pinPath: noCheckoutLastRollRev + "\n",
	}
	parent.MockGitilesFileForUpdate(parentGitiles, parentCfg.Gitiles, noCheckoutParentHead, oldContent)
	child.MockGitiles_GetRevision(childGitiles, noCheckoutLastRollRev, noCheckoutLastRollRev, noCheckoutTipRev)
	child.MockGitiles_Update(t, childGitiles, cfg.GetGitilesChild(), noCheckoutLastRollRev, noCheckoutTipRev, noCheckoutChildCommits, noCheckoutChildDepsContentsByHash)

	// Update.
	lastRollRev, tipRev, notRolledRevs := updateAndAssert(t, rm, parentGitiles, parentGerrit, childGitiles)
	require.Equal(t, noCheckoutLastRollRev, lastRollRev.Id)
	require.Equal(t, noCheckoutTipRev, tipRev.Id)
	require.Equal(t, len(noCheckoutChildCommits)-1, len(notRolledRevs))

	// Mock requests for CreateNewRoll.
	newContent := map[string]string{
		pinPath: noCheckoutTipRev + "\n",
	}
	parent.MockGitilesFileForCreateNewRoll(parentGitiles, parentGerrit, parentCfg.Gitiles, noCheckoutParentHead, fakeCommitMsgMock, oldContent, newContent, fakeReviewers)

	// In addition to the typical requests sent by the GitilesFile parent, we
	// read file contents from both child and parent.
	oldContent = map[string]string{
		parentCfg.Copies[0].DstRelPath:                 "old-contents1",
		path.Join(parentCfg.Copies[1].DstRelPath, "a"): "old-contents-a",
		path.Join(parentCfg.Copies[1].DstRelPath, "b"): "old-contents-b",
		path.Join(parentCfg.Copies[1].DstRelPath, "c"): "old-contents-c",
	}
	parentGitiles.On("ResolveRef", testutils.AnyContext, noCheckoutParentHead).Return(noCheckoutParentHead, nil).Once()
	gitiles_testutils.MockReadObject_Dir(parentGitiles, noCheckoutParentHead, parentCfg.Copies[1].DstRelPath, []string{"a", "b", "c"})
	for path, contents := range oldContent {
		gitiles_testutils.MockReadObject_File(parentGitiles, noCheckoutParentHead, path, contents)
	}

	childGitiles.On("ResolveRef", testutils.AnyContext, noCheckoutTipRev).Return(noCheckoutTipRev, nil).Once()
	childContent := map[string]string{
		parentCfg.Copies[0].SrcRelPath:                 "new-contents1",
		path.Join(parentCfg.Copies[1].SrcRelPath, "a"): "new-contents-a",
		path.Join(parentCfg.Copies[1].SrcRelPath, "b"): "new-contents-b",
		path.Join(parentCfg.Copies[1].SrcRelPath, "c"): "new-contents-c",
	}
	gitiles_testutils.MockReadObject_Dir(childGitiles, noCheckoutTipRev, parentCfg.Copies[1].SrcRelPath, []string{"a", "b", "c"})
	for path, contents := range childContent {
		gitiles_testutils.MockReadObject_File(childGitiles, noCheckoutTipRev, path, contents)
	}

	newContent = map[string]string{
		parentCfg.Copies[0].DstRelPath:                 "new-contents1",
		path.Join(parentCfg.Copies[1].DstRelPath, "a"): "new-contents-a",
		path.Join(parentCfg.Copies[1].DstRelPath, "b"): "new-contents-b",
		path.Join(parentCfg.Copies[1].DstRelPath, "c"): "new-contents-c",
	}
	for path, contents := range newContent {
		parentGerrit.On("EditFile", testutils.AnyContext, mock.Anything, path, contents).Return(nil).Once()
	}

	// Upload the CL.
	issue, err := rm.CreateNewRoll(t.Context(), lastRollRev, tipRev, notRolledRevs, fakeReviewers, false, false, fakeCommitMsg)
	require.NoError(t, err)
	require.Equal(t, int64(123), issue)
	assertExpectations(t, parentGitiles, parentGerrit, childGitiles)
}
