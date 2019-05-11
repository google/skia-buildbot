package types

import (
	"context"
	"io/ioutil"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

func TestCopyPatch(t *testing.T) {
	unittest.SmallTest(t)
	v := Patch{
		Issue:     "1",
		Patchset:  "2",
		PatchRepo: "https://dummy-repo.git",
		Server:    "volley.com",
	}
	deepequal.AssertCopy(t, v, v.Copy())
}

func TestCopyRepoState(t *testing.T) {
	unittest.SmallTest(t)
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
	deepequal.AssertCopy(t, v, v.Copy())
}

// repoMapSetup creates two test repos, and returns a map from repo URL to
// commits (from c0 to c4), a repograph.Map, and a cleanup function. The layout
// of each repo looks like this:
//
// c0--c1------c3--c4--
//       \-c2-----/
func repoMapSetup(t *testing.T) (map[string][]string, repograph.Map, func()) {
	unittest.MediumTest(t)

	ctx := context.Background()
	gb1 := git_testutils.GitInit(t, ctx)
	commits1 := git_testutils.GitSetup(ctx, gb1)

	gb2 := git_testutils.GitInit(t, ctx)
	commits2 := git_testutils.GitSetup(ctx, gb2)

	commitMap := map[string][]string{
		gb1.RepoUrl(): commits1,
		gb2.RepoUrl(): commits2,
	}

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	repoMap, err := repograph.NewLocalMap(ctx, []string{gb1.RepoUrl(), gb2.RepoUrl()}, tmp)
	assert.NoError(t, err)
	assert.NoError(t, repoMap.Update(ctx))

	cleanup := func() {
		gb1.Cleanup()
		gb2.Cleanup()
		util.RemoveAll(tmp)
	}
	return commitMap, repoMap, cleanup
}

func TestGetCommit(t *testing.T) {
	commitMap, repoMap, cleanup := repoMapSetup(t)
	defer cleanup()

	for repo, commits := range commitMap {
		for _, commit := range commits {
			graphCommit := repoMap[repo].Get(commit)
			assert.NotNil(t, graphCommit)
			rs := RepoState{
				Repo:     repo,
				Revision: commit,
			}
			rsCommit, err := rs.GetCommit(repoMap)
			assert.NoError(t, err)
			assert.Equal(t, graphCommit, rsCommit)
			rs.Patch = Patch{
				Issue:    "1",
				Patchset: "2",
				Server:   "volley.com",
			}
			rsCommit, err = rs.GetCommit(repoMap)
			assert.NoError(t, err)
			assert.Equal(t, graphCommit, rsCommit)
		}
	}
}

func TestGetCommitError(t *testing.T) {
	commitMap, repoMap, cleanup := repoMapSetup(t)
	defer cleanup()

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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unknown repo")

	_, err = RepoState{
		Repo:     existingRepo,
		Revision: "abc123",
	}.GetCommit(repoMap)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unknown revision")

	// Verify test data.
	c, err := RepoState{
		Repo:     existingRepo,
		Revision: existingRevision,
	}.GetCommit(repoMap)
	assert.NoError(t, err)
	assert.NotNil(t, c)
}

func TestParentsTryJob(t *testing.T) {
	unittest.SmallTest(t)
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
	assert.NoError(t, err)
	assert.Equal(t, 1, len(parents))
	deepequal.AssertDeepEqual(t, RepoState{
		Repo:     "nou.git",
		Revision: "1",
	}, parents[0])
}

func TestParentsSingle(t *testing.T) {
	commitMap, repoMap, cleanup := repoMapSetup(t)
	defer cleanup()

	test := func(repo, commit, parent string) {
		input := RepoState{
			Repo:     repo,
			Revision: commit,
		}
		actual, err := input.Parents(repoMap)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(actual))
		expected := RepoState{
			Repo:     repo,
			Revision: parent,
		}
		deepequal.AssertDeepEqual(t, expected, actual[0])
	}

	for repo, commits := range commitMap {
		test(repo, commits[1], commits[0])
		test(repo, commits[2], commits[1])
		test(repo, commits[3], commits[1])
	}
}

func TestParentsDouble(t *testing.T) {
	commitMap, repoMap, cleanup := repoMapSetup(t)
	defer cleanup()

	for repo, commits := range commitMap {
		input := RepoState{
			Repo:     repo,
			Revision: commits[4],
		}
		actual, err := input.Parents(repoMap)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(actual))

		expected := []RepoState{
			{
				Repo:     repo,
				Revision: commits[2],
			},
			{
				Repo:     repo,
				Revision: commits[3],
			},
		}
		if actual[0].Revision != expected[0].Revision {
			expected[0], expected[1] = expected[1], expected[0]
		}
		deepequal.AssertDeepEqual(t, expected, actual)
	}
}

func TestParentsNone(t *testing.T) {
	commitMap, repoMap, cleanup := repoMapSetup(t)
	defer cleanup()

	for repo, commits := range commitMap {
		input := RepoState{
			Repo:     repo,
			Revision: commits[0],
		}
		actual, err := input.Parents(repoMap)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(actual))
	}
}

func TestParentsError(t *testing.T) {
	unittest.SmallTest(t)
	// Empty.
	var repoMap repograph.Map
	input := RepoState{
		Repo:     "nou.git",
		Revision: "1",
	}
	_, err := input.Parents(repoMap)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unknown repo")
}

func TestRepoStateRowKey(t *testing.T) {
	unittest.SmallTest(t)

	check := func(rs RepoState, expect string) {
		assert.Equal(t, expect, rs.RowKey())
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
