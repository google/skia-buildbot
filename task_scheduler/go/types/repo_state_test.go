package types

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/git/testutils/mem_git"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/mem_gitstore"
)

func TestCopyPatch(t *testing.T) {
	v := Patch{
		Issue:     "1",
		Patchset:  "2",
		PatchRepo: "https://dummy-repo.git",
		Server:    "volley.com",
	}
	assertdeep.Copy(t, v, v.Copy())
}

func TestCopyRepoState(t *testing.T) {
	v := RepoState{
		Patch: Patch{
			Issue:     "1",
			Patchset:  "2",
			PatchRepo: "https://dummy-repo.git",
			Server:    "volley.com",
		},
		Repo:     "nou.git",
		Revision: "1",
	}
	assertdeep.Copy(t, v, v.Copy())
}

// repoMapSetup creates two test repos, and returns a map from repo URL to
// commits (from c0 to c4), a repograph.Map, and a cleanup function. The layout
// of the repos look like this:
//
// fake1.git:
//
// c0--c1------c3--c4--
//       \-c2-----/
//
// fake2.git:
//
// c0--c1--c2------c4--c5--
//           \-c3-----/
func repoMapSetup(t *testing.T) (map[string][]string, repograph.Map) {

	ctx := context.Background()

	gs1 := mem_gitstore.New()
	mg1 := mem_git.New(t, gs1)
	ri1, err := gitstore.NewGitStoreRepoImpl(ctx, gs1)
	require.NoError(t, err)
	repo1, err := repograph.NewWithRepoImpl(ctx, ri1)
	require.NoError(t, err)
	mg1.AddUpdater(repo1)
	commits1 := mem_git.FillWithBranchingHistory(mg1)

	gs2 := mem_gitstore.New()
	mg2 := mem_git.New(t, gs2)
	ri2, err := gitstore.NewGitStoreRepoImpl(ctx, gs2)
	require.NoError(t, err)
	repo2, err := repograph.NewWithRepoImpl(ctx, ri2)
	require.NoError(t, err)
	mg2.AddUpdater(repo2)
	initialCommitInRepo2 := mg2.Commit("Fake initial commit to ensure that hashes are different between repos")
	commits2 := []string{initialCommitInRepo2}
	commits2 = append(commits2, mem_git.FillWithBranchingHistory(mg2)...)

	commitMap := map[string][]string{
		"fake1.git": commits1,
		"fake2.git": commits2,
	}
	repoMap := repograph.Map{
		"fake1.git": repo1,
		"fake2.git": repo2,
	}
	return commitMap, repoMap
}

func TestGetCommit(t *testing.T) {
	commitMap, repoMap := repoMapSetup(t)

	for repo, commits := range commitMap {
		for _, commit := range commits {
			graphCommit := repoMap[repo].Get(commit)
			require.NotNil(t, graphCommit)
			rs := RepoState{
				Repo:     repo,
				Revision: commit,
			}
			rsCommit, err := rs.GetCommit(repoMap)
			require.NoError(t, err)
			require.Equal(t, graphCommit, rsCommit)
			rs.Patch = Patch{
				Issue:    "1",
				Patchset: "2",
				Server:   "volley.com",
			}
			rsCommit, err = rs.GetCommit(repoMap)
			require.NoError(t, err)
			require.Equal(t, graphCommit, rsCommit)
		}
	}
}

func TestGetCommitError(t *testing.T) {
	commitMap, repoMap := repoMapSetup(t)

	existingRepo := ""
	existingRevision := ""
	for repo, commits := range commitMap {
		existingRepo = repo
		existingRevision = commits[0]
		break
	}

	_, err := RepoState{
		Repo:     "nou.git",
		Revision: existingRevision,
	}.GetCommit(repoMap)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unknown repo")

	_, err = RepoState{
		Repo:     existingRepo,
		Revision: "abc123",
	}.GetCommit(repoMap)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unknown revision")

	// Verify test data.
	c, err := RepoState{
		Repo:     existingRepo,
		Revision: existingRevision,
	}.GetCommit(repoMap)
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestParentsTryJob(t *testing.T) {
	input := RepoState{
		Patch: Patch{
			Issue:    "1",
			Patchset: "2",
			Server:   "volley.com",
		},
		Repo:     "nou.git",
		Revision: "1",
	}
	// Unused.
	var repoMap repograph.Map
	parents, err := input.Parents(repoMap)
	require.NoError(t, err)
	require.Equal(t, 1, len(parents))
	assertdeep.Equal(t, RepoState{
		Repo:     "nou.git",
		Revision: "1",
	}, parents[0])
}

func TestParentsSingle(t *testing.T) {
	commitMap, repoMap := repoMapSetup(t)

	test := func(repo, commit, parent string) {
		input := RepoState{
			Repo:     repo,
			Revision: commit,
		}
		actual, err := input.Parents(repoMap)
		require.NoError(t, err)
		require.Equal(t, 1, len(actual))
		expected := RepoState{
			Repo:     repo,
			Revision: parent,
		}
		assertdeep.Equal(t, expected, actual[0])
	}

	repo := "fake1.git"
	commits := commitMap[repo]
	test(repo, commits[1], commits[0])
	test(repo, commits[2], commits[1])
	test(repo, commits[3], commits[1])

	repo = "fake2.git"
	commits = commitMap[repo]
	test(repo, commits[1], commits[0])
	test(repo, commits[2], commits[1])
	test(repo, commits[3], commits[2])
	test(repo, commits[4], commits[2])
}

func TestParentsDouble(t *testing.T) {
	commitMap, repoMap := repoMapSetup(t)

	test := func(repo, commit string, parents []string) {
		input := RepoState{
			Repo:     repo,
			Revision: commit,
		}
		actual, err := input.Parents(repoMap)
		require.NoError(t, err)
		require.Len(t, actual, len(parents))
		for idx, parent := range parents {
			expected := RepoState{
				Repo:     repo,
				Revision: parent,
			}
			assertdeep.Equal(t, expected, actual[idx])
		}
	}

	repo1 := "fake1.git"
	commits1 := commitMap[repo1]
	test(repo1, commits1[4], []string{commits1[3], commits1[2]})

	repo2 := "fake2.git"
	commits2 := commitMap[repo2]
	test(repo2, commits2[5], []string{commits2[4], commits2[3]})
}

func TestParentsNone(t *testing.T) {
	commitMap, repoMap := repoMapSetup(t)

	for repo, commits := range commitMap {
		input := RepoState{
			Repo:     repo,
			Revision: commits[0],
		}
		actual, err := input.Parents(repoMap)
		require.NoError(t, err)
		require.Equal(t, 0, len(actual))
	}
}

func TestParentsError(t *testing.T) {
	// Empty.
	var repoMap repograph.Map
	input := RepoState{
		Repo:     "nou.git",
		Revision: "1",
	}
	_, err := input.Parents(repoMap)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Unknown repo")
}

func TestRepoStateRowKey(t *testing.T) {

	check := func(rs RepoState, expect string) {
		require.Equal(t, expect, rs.RowKey())
	}

	// Simple, no patch.
	check(RepoState{
		Repo:     common.REPO_SKIA,
		Revision: "abc123",
	}, "2#abc123#skia.googlesource.com/skia#####")
	// Add a patch.
	check(RepoState{
		Repo:     common.REPO_SKIA,
		Revision: "abc123",
		Patch: Patch{
			Issue:     "12345",
			Patchset:  "2",
			Server:    "fake.server.com",
			PatchRepo: "https://skia.googlesource.com/other.git",
		},
	}, "2#abc123#skia.googlesource.com/skia#45#12345#2#skia.googlesource.com/other#fake.server.com")
	// Patches are valid without a PatchRepo.
	check(RepoState{
		Repo:     common.REPO_SKIA,
		Revision: "abc123",
		Patch: Patch{
			Issue:    "12345",
			Patchset: "2",
			Server:   "fake.server.com",
		},
	}, "2#abc123#skia.googlesource.com/skia#45#12345#2##fake.server.com")
}
