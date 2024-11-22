package child

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/chrome_branch/mocks"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/gitiles"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
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

	ctx := cipd_git.UseGitFinder(context.Background())
	repo := git_testutils.GitInit(t, ctx)
	commits := []string{}

	// Initial commit: set up the repo structure.
	repo.AddGen(ctx, "top-file.txt")
	repo.AddGen(ctx, "ignored-dir/ignored-file.txt")
	repo.AddGen(ctx, "watched-dir/watched-file.txt")
	commits = append(commits, repo.Commit(ctx))

	// Second commit: does not modify the watched dir.
	repo.AddGen(ctx, "top-file.txt")
	commits = append(commits, repo.Commit(ctx))

	// Third commit: modifies the watched dir.
	repo.AddGen(ctx, "watched-dir/watched-file.txt")
	commits = append(commits, repo.Commit(ctx))

	// Fourth commit: does not modify the watched dir.
	repo.AddGen(ctx, "ignored-dir/ignored-file.txt")
	commits = append(commits, repo.Commit(ctx))

	// Fifth commit: adds a file in the watched dir.
	repo.AddGen(ctx, "watched-dir/watched-file2.txt")
	commits = append(commits, repo.Commit(ctx))

	// Sixth commit: another unrelated file.
	repo.AddGen(ctx, "other.txt")
	commits = append(commits, repo.Commit(ctx))

	// Create the GitilesChild.
	cfg := config.GitilesChildConfig{
		Gitiles: &config.GitilesConfig{
			Branch:  git.MainBranch,
			RepoUrl: repo.RepoUrl(),
		},
		Path: "", // Test without Path first.
	}
	reg := setupRegistry(t)
	urlMock := mockhttpclient.NewURLMock()
	mockGitiles := gitiles_testutils.NewMockRepo(t, repo.RepoUrl(), git.CheckoutDir(repo.Dir()), urlMock)
	c, err := NewGitiles(ctx, &cfg, reg, urlMock.Client())
	require.NoError(t, err)

	// Update.
	lastRollRev := &revision.Revision{Id: commits[0]}
	mockGitiles.MockGetCommit(ctx, git.MainBranch)
	mockGitiles.MockLog(ctx, git.LogFromTo(commits[0], commits[len(commits)-1]))
	for _, c := range commits[1:] {
		mockGitiles.MockGetCommit(ctx, c)
	}
	tip, notRolled, err := c.Update(ctx, lastRollRev)
	require.NoError(t, err)
	require.Equal(t, commits[len(commits)-1], tip.Id)
	require.Equal(t, len(commits)-1, len(notRolled))
	require.True(t, urlMock.Empty())

	// Now, set Path.
	cfg.Path = "watched-dir"
	c, err = NewGitiles(ctx, &cfg, reg, urlMock.Client())
	mockGitiles.MockGetCommit(ctx, git.MainBranch)
	mockGitiles.MockLog(ctx, git.LogFromTo(commits[0], commits[len(commits)-1]), gitiles.LogPath(cfg.Path))
	mockGitiles.MockGetCommit(ctx, commits[2])
	mockGitiles.MockGetCommit(ctx, commits[4])
	tip, notRolled, err = c.Update(ctx, lastRollRev)
	require.NoError(t, err)
	require.Equal(t, commits[4], tip.Id)
	require.Equal(t, 2, len(notRolled))
	require.True(t, urlMock.Empty())
}
