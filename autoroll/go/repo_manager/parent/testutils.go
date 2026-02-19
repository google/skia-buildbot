package parent

import (
	"os"
	"strings"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/go/gerrit"
	gerrit_mocks "go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/git"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

func NewGitilesFileForTesting(t sktest.TestingT, cfg *config.GitilesParentConfig) (*gitilesParent, *gitiles_mocks.GitilesRepo, *gerrit_mocks.GerritInterface) {
	gitiles := &gitiles_mocks.GitilesRepo{}
	gerrit := &gerrit_mocks.GerritInterface{}
	p, err := NewGitilesFile(t.Context(), cfg, gitiles, gerrit, "")
	require.NoError(t, err)
	p.GitilesRepo.GitilesRepo = gitiles

	p.gerrit = gerrit
	return p, gitiles, gerrit
}

func NewCopyForTesting(t sktest.TestingT, cfg *config.CopyParentConfig, child child.Child) (*gitilesParent, *gitiles_mocks.GitilesRepo, *gerrit_mocks.GerritInterface) {
	gitiles := &gitiles_mocks.GitilesRepo{}
	gerrit := &gerrit_mocks.GerritInterface{}
	p, err := NewCopy(t.Context(), cfg, gitiles, gerrit, "", child)
	require.NoError(t, err)
	return p, gitiles, gerrit
}

func NewFreeTypeForTesting(t sktest.TestingT, cfg *config.FreeTypeParentConfig) (*gitilesParent, *gitiles_mocks.GitilesRepo, *gerrit_mocks.GerritInterface, func()) {
	gitiles := &gitiles_mocks.GitilesRepo{}
	gerrit := &gerrit_mocks.GerritInterface{}
	workdir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	p, err := NewFreeTypeParent(t.Context(), cfg, workdir, gitiles, gerrit, "")
	require.NoError(t, err)
	return p, gitiles, gerrit, func() { testutils.RemoveAll(t, workdir) }
}

func MockGitilesFileForUpdate(parentGitiles *gitiles_mocks.GitilesRepo, cfg *config.GitilesParentConfig, parentHead string, fileContent map[string]string) {
	parentGitiles.On("ResolveRef", testutils.AnyContext, git.MainBranch).Return(parentHead, nil).Once()
	for _, path := range pathsFromDep(cfg.Dep) {
		parentGitiles.On("ReadFileAtRef", testutils.AnyContext, path, parentHead).Return([]byte(fileContent[path]), nil).Once()
	}
}

func pathsFromDep(cfg *config.DependencyConfig) []string {
	// Remove duplicates, since fetches are cached.
	paths := util.StringSet{}
	for _, file := range cfg.Primary.File {
		paths[file.Path] = true
	}
	for _, dep := range cfg.Transitive {
		for _, file := range dep.Parent.File {
			paths[file.Path] = true
		}
	}
	return paths.Keys()
}

func MockGitilesFileForCreateNewRoll(parentGitiles *gitiles_mocks.GitilesRepo, parentGerrit *gerrit_mocks.GerritInterface, cfg *config.GitilesParentConfig, parentHead, commitMsg string, oldContent, newContent map[string]string, reviewers []string) {
	for _, path := range pathsFromDep(cfg.Dep) {
		parentGitiles.On("ReadFileAtRef", testutils.AnyContext, path, parentHead).Return([]byte(oldContent[path]), nil).Once()
	}

	// Mock the initial change creation.
	changeInfo := &gerrit.ChangeInfo{
		ChangeId: "123",
		Project:  "test-project",
		Branch:   "test-branch",
		Id:       "123",
		Issue:    123,
		Revisions: map[string]*gerrit.Revision{
			"ps1": {
				ID:     "ps1",
				Number: 1,
			},
			"ps2": {
				ID:     "ps2",
				Number: 2,
			},
		},
		WorkInProgress: true,
	}
	subject := strings.Split(commitMsg, "\n")[0]
	parentGerrit.On("CreateChange", testutils.AnyContext, cfg.Gerrit.Project, cfg.Gitiles.Branch, subject, parentHead, "").Return(changeInfo, nil)
	parentGerrit.On("SetCommitMessage", testutils.AnyContext, changeInfo, commitMsg).Return(nil)
	for _, path := range pathsFromDep(cfg.Dep) {
		parentGerrit.On("EditFile", testutils.AnyContext, changeInfo, path, newContent[path]).Return(nil)
	}
	parentGerrit.On("PublishChangeEdit", testutils.AnyContext, changeInfo).Return(nil)
	parentGerrit.On("GetIssueProperties", testutils.AnyContext, changeInfo.Issue).Return(changeInfo, nil)
	parentGerrit.On("SetReadyForReview", testutils.AnyContext, changeInfo).Return(nil)
	gerritConfig := codereview.GerritConfigs[cfg.Gerrit.Config]
	parentGerrit.On("Config").Return(gerritConfig)
	labels := gerrit.MergeLabels(gerritConfig.SelfApproveLabels, gerritConfig.SetCqLabels)
	parentGerrit.On("SetReview", testutils.AnyContext, changeInfo, "", labels, reviewers, gerrit.NotifyDefault, gerrit.NotifyDetails(nil), "", 0, []*gerrit.AttentionSetInput(nil)).Return(nil)
	if !gerritConfig.HasCq {
		parentGerrit.On("Submit", testutils.AnyContext, changeInfo).Return(nil)
	}
}
