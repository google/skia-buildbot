package skip_tasks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	ftestutils "go.skia.org/infra/go/firestore/testutils"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/git/testutils/mem_git"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/mem_gitstore"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func setup(t *testing.T) (*DB, func()) {
	unittest.LargeTest(t)
	c, cleanup := ftestutils.NewClientForTesting(context.Background(), t)
	b, err := New(context.Background(), c)
	require.NoError(t, err)
	return b, cleanup
}

func TestAddRemove(t *testing.T) {
	b1, cleanup1 := setup(t)
	defer cleanup1()

	// Test.
	r1 := &Rule{
		AddedBy:          "test@google.com",
		TaskSpecPatterns: []string{".*"},
		Name:             "My Rule",
	}
	ctx := context.Background()
	require.NoError(t, b1.addRule(ctx, r1))
	// The Firestore emulator doesn't seem to allow different clients to see each
	// other's data, so we use the same client as b1.
	b2, err := New(ctx, b1.client)
	require.NoError(t, err)
	assertEqual := func() {
		require.NoError(t, testutils.EventuallyConsistent(30*time.Second, func() error {
			require.NoError(t, b2.Update(ctx))
			if len(b1.rules) == len(b2.rules) {
				assertdeep.Equal(t, b1.rules, b2.rules)
				return nil
			}
			time.Sleep(100 * time.Millisecond)
			return testutils.TryAgainErr
		}))
	}
	assertEqual()

	require.NoError(t, b1.RemoveRule(ctx, r1.Name))
	assertEqual()
}

func TestRuleCopy(t *testing.T) {
	unittest.SmallTest(t)
	r := &Rule{
		AddedBy:          "me@google.com",
		TaskSpecPatterns: []string{"a", "b"},
		Commits:          []string{"abc123", "def456"},
		Description:      "this is a rule",
		Name:             "example",
	}
	assertdeep.Copy(t, r, r.Copy())
}

func TestRules(t *testing.T) {
	unittest.SmallTest(t)
	type testCase struct {
		taskSpec    string
		commit      string
		expectMatch bool
		msg         string
	}
	tests := []struct {
		rule  Rule
		cases []testCase
	}{
		{
			rule: Rule{
				AddedBy: "test@google.com",
				Name:    "Match all taskSpecs",
				TaskSpecPatterns: []string{
					".*",
				},
			},
			cases: []testCase{
				{
					taskSpec:    "My-TaskSpec",
					commit:      "abc123",
					expectMatch: true,
					msg:         "'.*' should match all taskSpecs.",
				},
			},
		},
		{
			rule: Rule{
				AddedBy: "test@google.com",
				Name:    "Match some taskSpecs",
				TaskSpecPatterns: []string{
					"My.*kSpec",
				},
			},
			cases: []testCase{
				{
					taskSpec:    "My-TaskSpec",
					commit:      "abc123",
					expectMatch: true,
					msg:         "Should match",
				},
				{
					taskSpec:    "Your-TaskSpec",
					commit:      "abc123",
					expectMatch: false,
					msg:         "Should not match",
				},
			},
		},
		{
			rule: Rule{
				AddedBy: "test@google.com",
				Name:    "Match one commit",
				TaskSpecPatterns: []string{
					".*",
				},
				Commits: []string{
					"abc123",
				},
			},
			cases: []testCase{
				{
					taskSpec:    "My-TaskSpec",
					commit:      "abc123",
					expectMatch: true,
					msg:         "Single commit match",
				},
				{
					taskSpec:    "My-TaskSpec",
					commit:      "def456",
					expectMatch: false,
					msg:         "Single commit does not match",
				},
			},
		},
		{
			rule: Rule{
				AddedBy:          "test@google.com",
				Name:             "Commit range",
				TaskSpecPatterns: []string{},
				Commits: []string{
					"abc123",
					"bbadbeef",
				},
			},
			cases: []testCase{
				{
					taskSpec:    "My-TaskSpec",
					commit:      "abc123",
					expectMatch: true,
					msg:         "Inside commit range (1)",
				},
				{
					taskSpec:    "My-TaskSpec",
					commit:      "bbadbeef",
					expectMatch: true,
					msg:         "Inside commit range (2)",
				},
				{
					taskSpec:    "My-TaskSpec",
					commit:      "def456",
					expectMatch: false,
					msg:         "Outside commit range",
				},
			},
		},
	}
	for _, test := range tests {
		for _, c := range test.cases {
			require.Equal(t, c.expectMatch, test.rule.Match(c.taskSpec, c.commit), c.msg)
		}
	}
}

func setupTestRepo(t *testing.T) (context.Context, repograph.Map, []string) {
	ctx := context.Background()
	gs := mem_gitstore.New()
	mg := mem_git.New(t, gs)
	ri, err := gitstore.NewGitStoreRepoImpl(ctx, gs)
	require.NoError(t, err)
	repo, err := repograph.NewWithRepoImpl(ctx, ri)
	require.NoError(t, err)
	mg.AddUpdater(repo)

	commits := []string{}

	/*
	   * commit 8
	   |
	   *   commit 7
	   |\  Merge: 4 6
	   | |
	   | |     Merge branch 'mybranch'
	   | |
	   | * commit 6
	   | |
	   | * commit 5
	   | |
	   * | commit 4
	   | |
	   * | commit 3
	   | |
	   * | commit 2
	   |/
	   * commit 1
	   |
	   * commit 0
	*/

	// 0
	commits = append(commits, mg.Commit("0"))

	// 1
	commits = append(commits, mg.Commit("1"))

	// 2
	commits = append(commits, mg.Commit("2"))

	// 3
	commits = append(commits, mg.Commit("3"))

	// 4
	commits = append(commits, mg.Commit("4"))

	// 5
	mg.NewBranch("mybranch", commits[1])
	commits = append(commits, mg.Commit("5"))

	// 6
	commits = append(commits, mg.Commit("6"))

	// 7
	mg.CheckoutBranch(git.MainBranch)
	commits = append(commits, mg.Merge("mybranch"))

	// 8
	commits = append(commits, mg.Commit("8"))

	repos := repograph.Map{
		"fake.git": repo,
	}
	return ctx, repos, commits
}

func TestValidation(t *testing.T) {
	unittest.SmallTest(t)
	// Setup.
	_, repos, commits := setupTestRepo(t)

	// Test.
	tests := []struct {
		rule   Rule
		expect error
		msg    string
	}{
		{
			rule: Rule{
				AddedBy:          "",
				Name:             "My rule",
				TaskSpecPatterns: []string{".*"},
				Commits:          []string{},
			},
			expect: fmt.Errorf("Rules must have an AddedBy user."),
			msg:    "No AddedBy",
		},
		{
			rule: Rule{
				AddedBy:          "test@google.com",
				Name:             "",
				TaskSpecPatterns: []string{".*"},
				Commits:          []string{},
			},
			expect: fmt.Errorf("Rules must have a name."),
			msg:    "No Name",
		},
		{
			rule: Rule{
				AddedBy:          "test@google.com",
				Name:             "01234567890123456789012345678901234567890123456789",
				TaskSpecPatterns: []string{".*"},
				Commits:          []string{},
			},
			expect: nil,
			msg:    "Long Name",
		},
		{
			rule: Rule{
				AddedBy:          "test@google.com",
				Name:             "012345678901234567890123456789012345678901234567890",
				TaskSpecPatterns: []string{".*"},
				Commits:          []string{},
			},
			expect: fmt.Errorf("Rule names must be shorter than 50 characters. Use the Description field for detailed information."),
			msg:    "Too Long Name",
		},
		{
			rule: Rule{
				AddedBy:          "test@google.com",
				Name:             "My rule",
				TaskSpecPatterns: []string{},
				Commits:          []string{},
			},
			expect: fmt.Errorf("Rules must include a taskSpec pattern and/or a commit/range."),
			msg:    "No taskSpecs or commits",
		},
		{
			rule: Rule{
				AddedBy:          "test@google.com",
				Name:             "My rule",
				TaskSpecPatterns: []string{".*"},
				Commits:          []string{},
			},
			expect: nil,
			msg:    "One taskSpec pattern, no commits",
		},
		{
			rule: Rule{
				AddedBy: "test@google.com",
				Name:    "My rule",
				TaskSpecPatterns: []string{
					"A.*B",
					"[C|D]{42}",
					"[Your|My]-TaskSpec",
					"Test.*",
					"Some-TaskSpec",
				},
				Commits: []string{},
			},
			expect: nil,
			msg:    "Five taskSpec patterns, no commits",
		},
		{
			rule: Rule{
				AddedBy:          "test@google.com",
				Name:             "My rule",
				TaskSpecPatterns: []string{},
				Commits:          commits[8:9],
			},
			expect: nil,
			msg:    "One commit",
		},
		{
			rule: Rule{
				AddedBy:          "test@google.com",
				Name:             "My rule",
				TaskSpecPatterns: []string{},
				Commits: []string{
					commits[8][:38],
				},
			},
			expect: fmt.Errorf("Unable to find commit %s in any repo.", commits[8][:38]),
			msg:    "Invalid commit",
		},
		{
			rule: Rule{
				AddedBy:          "test@google.com",
				Name:             "My rule",
				TaskSpecPatterns: []string{},
				Commits:          commits[0:5],
			},
			expect: nil,
			msg:    "Five commits",
		},
	}
	for _, test := range tests {
		require.Equal(t, test.expect, ValidateRule(&test.rule, repos), test.msg)
	}
}

func TestCommitRange(t *testing.T) {
	unittest.LargeTest(t)
	// Setup.
	ctx, repos, commits := setupTestRepo(t)
	b, cleanup := setup(t)
	defer cleanup()

	// Test.

	// Create a commit range rule.
	startCommit := commits[0]
	endCommit := commits[6]
	rule, err := NewCommitRangeRule(ctx, "commit range", "test@google.com", "...", []string{}, startCommit, endCommit, repos)
	require.NoError(t, err)
	err = b.AddRule(ctx, rule, repos)
	require.NoError(t, err)

	// Ensure that we got the expected list of commits.
	require.Equal(t, []string{
		commits[5],
		commits[1],
		commits[0],
	}, b.rules[rule.Name].Commits)

	// Test a few commits.
	tc := []struct {
		commit string
		expect bool
		msg    string
	}{
		{
			commit: "",
			expect: false,
			msg:    "empty commit does not match",
		},
		{
			commit: startCommit,
			expect: true,
			msg:    "startCommit matches",
		},
		{
			commit: endCommit,
			expect: false,
			msg:    "endCommit does not match",
		},
		{
			commit: commits[1],
			expect: true,
			msg:    "middle of range matches",
		},
		{
			commit: commits[8],
			expect: false,
			msg:    "out of range does not match",
		},
	}
	for _, c := range tc {
		require.Equal(t, c.expect, b.Match("", c.commit))
	}
}
