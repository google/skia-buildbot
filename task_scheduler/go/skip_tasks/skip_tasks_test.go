package skip_tasks

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func setup(t *testing.T) (*DB, func()) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(context.Background(), t)
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
	require.NoError(t, b1.addRule(r1))
	// The Firestore emulator doesn't seem to allow different clients to see each
	// other's data, so we use the same client as b1.
	b2, err := New(context.Background(), b1.client)
	require.NoError(t, err)
	assertEqual := func() {
		require.NoError(t, testutils.EventuallyConsistent(30*time.Second, func() error {
			require.NoError(t, b2.Update())
			if len(b1.rules) == len(b2.rules) {
				assertdeep.Equal(t, b1.rules, b2.rules)
				return nil
			}
			time.Sleep(100 * time.Millisecond)
			return testutils.TryAgainErr
		}))
	}
	assertEqual()

	require.NoError(t, b1.RemoveRule(r1.Name))
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

func setupTestRepo(t *testing.T) (context.Context, *git_testutils.GitBuilder, []string) {
	ctx := context.Background()
	gb := git_testutils.GitInit(t, ctx)
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
	gb.Add(ctx, "a.txt", "")
	commits = append(commits, gb.Commit(ctx))

	// 1
	gb.Add(ctx, "a.txt", "\n")
	commits = append(commits, gb.Commit(ctx))

	// 2
	gb.Add(ctx, "a.txt", "\n\n")
	commits = append(commits, gb.Commit(ctx))

	// 3
	gb.Add(ctx, "a.txt", "\n\n\n")
	commits = append(commits, gb.Commit(ctx))

	// 4
	gb.Add(ctx, "a.txt", "\n\n\n\n")
	commits = append(commits, gb.Commit(ctx))

	// 5
	gb.CreateBranchAtCommit(ctx, "mybranch", commits[1])
	gb.Add(ctx, "b.txt", "\n\n")
	commits = append(commits, gb.Commit(ctx))

	// 6
	gb.Add(ctx, "b.txt", "\n\n\n")
	commits = append(commits, gb.Commit(ctx))

	// 7
	gb.CheckoutBranch(ctx, git.DefaultBranch)
	commits = append(commits, gb.MergeBranch(ctx, "mybranch"))

	// 8
	gb.Add(ctx, "a.txt", "\n\n\n\n\n")
	commits = append(commits, gb.Commit(ctx))

	return ctx, gb, commits
}

func TestValidation(t *testing.T) {
	unittest.LargeTest(t)
	// Setup.
	ctx, gb, commits := setupTestRepo(t)
	defer gb.Cleanup()
	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	repos := repograph.Map{}
	repo, err := repograph.NewLocalGraph(ctx, gb.RepoUrl(), tmp)
	require.NoError(t, err)
	repos[gb.RepoUrl()] = repo
	require.NoError(t, repos.Update(ctx))

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
	ctx, gb, commits := setupTestRepo(t)
	defer gb.Cleanup()
	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	repos := repograph.Map{}
	repo, err := repograph.NewLocalGraph(ctx, gb.RepoUrl(), tmp)
	require.NoError(t, err)
	repos[gb.RepoUrl()] = repo
	require.NoError(t, repos.Update(ctx))
	b, cleanup := setup(t)
	defer cleanup()

	// Test.

	// Create a commit range rule.
	startCommit := commits[0]
	endCommit := commits[6]
	rule, err := NewCommitRangeRule(ctx, "commit range", "test@google.com", "...", []string{}, startCommit, endCommit, repos)
	require.NoError(t, err)
	err = b.AddRule(rule, repos)
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
