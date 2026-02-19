package repo_manager

import (
	"context"
	"fmt"
	"path"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	gerrit_mocks "go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/gitiles"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

const (
	ftChildPath   = "src/third_party/freetype/src"
	ftIncludeTmpl = `%s header %d


commit %d
`
	ftReadmeTmpl = `Fake README.chromium
blah blah
Version: %s
Revision: %s
CPEPrefix: cpe:/a:freetype:freetype:%s
blah blah`
	ftVersionTmpl  = "VER-0-0-%d"
	cpeVersionTmpl = "0.0.%d"
)

func setupFreeType(t *testing.T) (context.Context, *config.FreeTypeRepoManagerConfig, RepoManager, *gitiles_testutils.MockRepo, *gitiles_mocks.GitilesRepo, *gerrit_mocks.GerritInterface, *mockhttpclient.URLMock, []string, func()) {
	ctx := cipd_git.UseGitFinder(t.Context())

	// Create child repo. This is needed for git tags, though presumably we
	// could find a way to do it using gitiles.
	childRepo := git_testutils.GitInit(t, ctx)
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		for idx, h := range parent.FtIncludesToMerge {
			childRepo.Add(ctx, path.Join(parent.FtIncludeSrc, h), fmt.Sprintf(ftIncludeTmpl, "child", idx, i))
		}
		childCommits = append(childCommits, childRepo.Commit(ctx))
		_, err := git.CheckoutDir(childRepo.Dir()).Git(ctx, "tag", "-a", fmt.Sprintf(ftVersionTmpl, i), "-m", fmt.Sprintf("Version %d", i))
		require.NoError(t, err)
	}
	urlmock := mockhttpclient.NewURLMock()
	mockChild := gitiles_testutils.NewMockRepo(t, childRepo.RepoUrl(), git.CheckoutDir(childRepo.Dir()), urlmock)

	cfg := &config.FreeTypeRepoManagerConfig{
		Parent: &config.FreeTypeParentConfig{
			Gitiles: &config.GitilesParentConfig{
				Gitiles: &config.GitilesConfig{
					Branch:  git.MainBranch,
					RepoUrl: noCheckoutParentRepo,
				},
				Dep: &config.DependencyConfig{
					Primary: &config.VersionFileConfig{
						Id: childRepo.RepoUrl(),
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
		Child: &config.GitilesChildConfig{
			Gitiles: &config.GitilesConfig{
				Branch:  git.MainBranch,
				RepoUrl: childRepo.RepoUrl(),
			},
		},
	}

	p, parentGitiles, parentGerrit, cleanup := parent.NewFreeTypeForTesting(t, cfg.Parent)
	repo := gitiles.NewRepo(cfg.Child.Gitiles.RepoUrl, urlmock.Client())
	c, err := child.NewGitiles(ctx, cfg.Child, repo)
	require.NoError(t, err)

	// Create the RepoManager.
	rm := &parentChildRepoManager{
		Parent: p,
		Child:  c,
	}

	fileContents := map[string]string{
		deps_parser.DepsFileName: fmt.Sprintf(`deps = {
  "%s": "%s@%s",
}`, ftChildPath, childRepo.RepoUrl(), childCommits[0]),
	}

	// Mock requests for Update().
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.Parent.Gitiles, noCheckoutParentHead, fileContents)
	mockChild.MockGetCommit(ctx, git.MainBranch)
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))

	// Update.
	_, _, _ = updateAndAssert(t, rm, parentGitiles, parentGerrit)

	rvCleanup := func() {
		cleanup()
		childRepo.Cleanup()
		assertExpectations(t, parentGitiles, parentGerrit, urlmock)
	}
	return ctx, cfg, rm, mockChild, parentGitiles, parentGerrit, urlmock, childCommits, rvCleanup
}

func TestFreeTypeRepoManagerUpdate(t *testing.T) {
	ctx, cfg, rm, mockChild, parentGitiles, parentGerrit, urlmock, childCommits, cleanup := setupFreeType(t)
	defer cleanup()

	// Mock requests for Update().
	fileContents := map[string]string{
		deps_parser.DepsFileName: fmt.Sprintf(`deps = {
  "%s": "%s@%s",
}`, ftChildPath, cfg.Child.Gitiles.RepoUrl, childCommits[0]),
	}
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.Parent.Gitiles, noCheckoutParentHead, fileContents)
	mockChild.MockGetCommit(ctx, git.MainBranch)
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))

	// Update.
	lastRollRev, tipRev, notRolledRevs := updateAndAssert(t, rm, parentGitiles, parentGerrit, urlmock)
	require.Equal(t, lastRollRev.Id, childCommits[0])
	require.Equal(t, tipRev.Id, childCommits[len(childCommits)-1])
	require.Equal(t, len(notRolledRevs), len(childCommits)-1)
}

func TestFreeTypeRepoManagerCreateNewRoll(t *testing.T) {
	ctx, cfg, rm, mockChild, parentGitiles, parentGerrit, urlmock, childCommits, cleanup := setupFreeType(t)
	defer cleanup()

	// Mock requests for Update().
	oldContent := map[string]string{
		deps_parser.DepsFileName: fmt.Sprintf(`deps = {
  "%s": "%s@%s",
}`, ftChildPath, cfg.Child.Gitiles.RepoUrl, childCommits[0]),
		parent.FtReadmePath: fmt.Sprintf(ftReadmeTmpl, fmt.Sprintf(ftVersionTmpl, 0), childCommits[0], fmt.Sprintf(cpeVersionTmpl, 0)),
	}
	for idx, h := range parent.FtIncludesToMerge {
		oldContent[path.Join(parent.FtIncludeDest, h)] = fmt.Sprintf(ftIncludeTmpl, "parent", idx, 0)
	}

	parent.MockGitilesFileForUpdate(parentGitiles, cfg.Parent.Gitiles, noCheckoutParentHead, oldContent)
	mockChild.MockGetCommit(ctx, git.MainBranch)
	mockChild.MockLog(ctx, git.LogFromTo(childCommits[0], childCommits[len(childCommits)-1]))

	// Update.
	lastRollRev, tipRev, notRolledRevs := updateAndAssert(t, rm, parentGitiles, parentGerrit, urlmock)
	require.Equal(t, lastRollRev.Id, childCommits[0])
	require.Equal(t, tipRev.Id, childCommits[len(childCommits)-1])
	require.Equal(t, len(notRolledRevs), len(childCommits)-1)

	// Mock requests for CreateNewRoll().
	newFtVersion := len(childCommits) - 1
	// TODO(borenet): Where is this suffix coming from?
	newFtVersionStr := fmt.Sprintf(ftVersionTmpl, newFtVersion) + "-0-g" + tipRev.Id[:7]
	newContent := map[string]string{
		deps_parser.DepsFileName: fmt.Sprintf(`deps = {
  "%s": "%s@%s",
}`, ftChildPath, cfg.Child.Gitiles.RepoUrl, tipRev.Id),
		parent.FtReadmePath: fmt.Sprintf(ftReadmeTmpl, newFtVersionStr, tipRev.Id, fmt.Sprintf(cpeVersionTmpl, newFtVersion)),
	}
	parent.MockGitilesFileForCreateNewRoll(parentGitiles, parentGerrit, cfg.Parent.Gitiles, noCheckoutParentHead, fakeCommitMsgMock, oldContent, newContent, fakeReviewers)

	// Mock the request to retrieve and edit the README.chromium file.
	parentGitiles.On("ResolveRef", testutils.AnyContext, noCheckoutParentHead).Return(noCheckoutParentHead, nil).Once()
	gitiles_testutils.MockReadObject_File(parentGitiles, noCheckoutParentHead, parent.FtReadmePath, oldContent[parent.FtReadmePath])
	parentGerrit.On("EditFile", testutils.AnyContext, mock.Anything, parent.FtReadmePath, newContent[parent.FtReadmePath]).Return(nil).Once()

	// Mock the requests to retrieve and edit the headers.
	for idx, h := range parent.FtIncludesToMerge {
		fp := path.Join(parent.FtIncludeDest, h)
		newContents := fmt.Sprintf(ftIncludeTmpl, "parent", idx, newFtVersion)
		newContent[fp] = newContents
		gitiles_testutils.MockReadObject_File(parentGitiles, noCheckoutParentHead, fp, oldContent[fp])
		parentGerrit.On("EditFile", testutils.AnyContext, mock.Anything, fp, newContent[fp]).Return(nil).Once()
	}

	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, fakeReviewers, false, false, fakeCommitMsg)
	require.NoError(t, err)
	require.NotEqual(t, 0, issue)
	assertExpectations(t, parentGitiles, parentGerrit, urlmock)
}
