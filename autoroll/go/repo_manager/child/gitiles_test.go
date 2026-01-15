package child

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

// TODO(borenet): Split up the tests in no_checkout_deps_repo_manager_test.go
// and move the relevant parts here.

// TestGitilesChildPathFilter verifies that GitilesChild filters out Revisions
// which do not modify the configured path.
func TestGitilesChildPathFilter(t *testing.T) {
	commits := []string{
		"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		"dddddddddddddddddddddddddddddddddddddddd",
		"cccccccccccccccccccccccccccccccccccccccc",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	// Create the GitilesChild.
	cfg := config.GitilesChildConfig{
		Gitiles: &config.GitilesConfig{
			Branch:  git.MainBranch,
			RepoUrl: "fake-repo.git",
		},
		Path: "", // Test without Path first.
	}
	c, mockGitiles := NewGitilesForTesting(t, &cfg)

	// Update.
	lastRollRev := commits[len(commits)-1]
	tipRev := commits[0]
	MockGitiles_Update(t, mockGitiles, &cfg, lastRollRev, tipRev, commits, nil)

	tip, notRolled, err := c.Update(t.Context(), &revision.Revision{Id: lastRollRev})
	require.NoError(t, err)
	require.Equal(t, tipRev, tip.Id)
	require.Equal(t, len(commits)-1, len(notRolled))
	mockGitiles.AssertExpectations(t)

	// Now, set Path.
	c.path = "watched-dir"
	mockGitiles.On("Details", testutils.AnyContext, git.MainBranch).Return(&vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash: tipRev,
		},
	}, nil).Once()
	notRolledCommits := []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: commits[1],
			},
		},
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: commits[3],
			},
		},
	}
	mockGitiles.On("Log", testutils.AnyContext, git.LogFromTo(lastRollRev, tipRev), gitiles.LogPath(c.path)).Return(notRolledCommits, nil).Once()

	tip, notRolled, err = c.Update(t.Context(), &revision.Revision{Id: lastRollRev})
	require.NoError(t, err)
	require.Equal(t, commits[1], tip.Id)
	require.Equal(t, 2, len(notRolled))
	mockGitiles.AssertExpectations(t)
}
