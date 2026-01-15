package child

import (
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/go/git"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

func NewGitilesForTesting(t sktest.TestingT, cfg *config.GitilesChildConfig) (*gitilesChild, *gitiles_mocks.GitilesRepo) {
	c, err := NewGitiles(t.Context(), cfg, nil)
	require.NoError(t, err)
	gitiles := &gitiles_mocks.GitilesRepo{}
	gitiles.On("URL").Return(cfg.Gitiles.RepoUrl)
	c.GitilesRepo.GitilesRepo = gitiles
	return c, gitiles
}

func MockGitiles_GetRevision(childGitiles *gitiles_mocks.GitilesRepo, ref, hash, tipRev string) {
	childGitiles.On("Details", testutils.AnyContext, ref).Return(&vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash: hash,
		},
	}, nil).Once()
	mockGitiles_getRevisionHelper(childGitiles, hash, tipRev)
}

func mockGitiles_getRevisionHelper(childGitiles *gitiles_mocks.GitilesRepo, hash, tipRev string) {
	// Nothing to do right now.
}

func MockGitiles_ConvertRevisions(childGitiles *gitiles_mocks.GitilesRepo, hashes []string, tipRev string) {
	for _, hash := range hashes {
		mockGitiles_getRevisionHelper(childGitiles, hash, tipRev)
	}
}

func MockGitiles_Update(t sktest.TestingT, childGitiles *gitiles_mocks.GitilesRepo, cfg *config.GitilesChildConfig, lastRollRev, tipRev string, childCommits []string, childDepsContentByHash map[string]string) {
	lastRollRevIndex := -1
	tipRevIndex := -1
	for idx, commit := range childCommits {
		if commit == lastRollRev {
			lastRollRevIndex = idx
		}
		if commit == tipRev {
			tipRevIndex = idx
		}
	}
	require.GreaterOrEqual(t, tipRevIndex, 0)
	require.GreaterOrEqual(t, lastRollRevIndex, tipRevIndex)

	notRolledHashes := childCommits[tipRevIndex:lastRollRevIndex]
	notRolledCommits := make([]*vcsinfo.LongCommit, 0, len(notRolledHashes))
	for _, hash := range notRolledHashes {
		notRolledCommits = append(notRolledCommits, &vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: hash,
			},
		})
	}

	MockGitiles_GetRevision(childGitiles, git.MainBranch, tipRev, tipRev)
	MockGitiles_ConvertRevisions(childGitiles, notRolledHashes, tipRev)

	childGitiles.On("LogFirstParent", testutils.AnyContext, lastRollRev, tipRev).Return(notRolledCommits, nil).Once()
	if len(cfg.Gitiles.Dependencies) > 0 {
		childGitiles.On("ReadFileAtRef", testutils.AnyContext, "DEPS", lastRollRev).Return([]byte(childDepsContentByHash[lastRollRev]), nil).Once()
		childGitiles.On("ReadFileAtRef", testutils.AnyContext, "DEPS", tipRev).Return([]byte(childDepsContentByHash[tipRev]), nil).Once()
		for _, hash := range notRolledHashes {
			childGitiles.On("ReadFileAtRef", testutils.AnyContext, "DEPS", hash).Return([]byte(childDepsContentByHash[hash]), nil).Once()
		}
	}
}
