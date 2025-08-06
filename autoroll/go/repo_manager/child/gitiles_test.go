package child

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/chrome_branch/mocks"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

// TODO(borenet): Split up the tests in no_checkout_deps_repo_manager_test.go
// and move the relevant parts here.

// TODO(borenet): This was copied from no_checkout_deps_repo_manager_test.go.
func defaultBranchTmpl(t *testing.T) *config_vars.Template {
	tmpl, err := config_vars.NewTemplate(git.MainBranch)
	require.NoError(t, err)
	return tmpl
}

// TODO(borenet): This was copied from repo_manager_test.go.
func setupRegistry(t *testing.T) *config_vars.Registry {
	cbc := &mocks.Client{}
	cbc.On("Get", mock.Anything).Return(config_vars.FakeVars().Branches.Chromium, config_vars.FakeVars().Branches.ActiveMilestones, nil)
	reg, err := config_vars.NewRegistry(context.Background(), cbc)
	require.NoError(t, err)
	return reg
}

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
	reg := setupRegistry(t)

	c, mockGitiles := NewGitilesForTesting(t, &cfg, reg)

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
