package gitinfo

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/git"
	vcstu "go.skia.org/infra/go/vcsinfo/testutils"
)

func TestVCSSuite(t *testing.T) {
	repoDir, cleanup := vcstu.InitTempRepo(t)
	defer cleanup()

	ctx := cipd_git.UseGitFinder(context.Background())
	r, err := NewGitInfo(ctx, repoDir, false, false)
	require.NoError(t, err)
	vcstu.TestDisplay(ctx, t, r)
}

func TestFrom(t *testing.T) {
	repoDir, cleanup := vcstu.InitTempRepo(t)
	defer cleanup()

	ctx := cipd_git.UseGitFinder(context.Background())
	r, err := NewGitInfo(ctx, repoDir, false, false)
	require.NoError(t, err)
	vcstu.TestFrom(ctx, t, r)
}

func TestLastN(t *testing.T) {
	repoDir, cleanup := vcstu.InitTempRepo(t)
	defer cleanup()

	ctx := cipd_git.UseGitFinder(context.Background())
	r, err := NewGitInfo(ctx, repoDir, false, false)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		n      int
		values []string
	}{
		{
			n:      0,
			values: []string{},
		},
		{
			n:      1,
			values: []string{"8652a6df7dc8a7e6addee49f6ed3c2308e36bd18"},
		},
		{
			n:      2,
			values: []string{"7a669cfa3f4cd3482a4fd03989f75efcc7595f7f", "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18"},
		},
		{
			n:      5,
			values: []string{"7a669cfa3f4cd3482a4fd03989f75efcc7595f7f", "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18"},
		},
	}
	for _, tc := range testCases {
		assert.ElementsMatch(t, tc.values, r.LastN(ctx, tc.n))
	}
}

func TestByIndex(t *testing.T) {
	repoDir, cleanup := vcstu.InitTempRepo(t)
	defer cleanup()

	ctx := cipd_git.UseGitFinder(context.Background())
	r, err := NewGitInfo(ctx, repoDir, false, false)
	require.NoError(t, err)
	vcstu.TestByIndex(ctx, t, r)
}

func TestLastNIndex(t *testing.T) {
	repoDir, cleanup := vcstu.InitTempRepo(t)
	defer cleanup()

	ctx := cipd_git.UseGitFinder(context.Background())
	r, err := NewGitInfo(ctx, repoDir, false, false)
	require.NoError(t, err)
	vcstu.TestLastNIndex(ctx, t, r)
}

func TestIndexOf(t *testing.T) {
	repoDir, cleanup := vcstu.InitTempRepo(t)
	defer cleanup()

	ctx := cipd_git.UseGitFinder(context.Background())
	r, err := NewGitInfo(ctx, repoDir, false, false)
	if err != nil {
		t.Fatal(err)
	}
	vcstu.TestIndexOf(ctx, t, r)
	require.Equal(t, "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f", r.firstCommit)
}

func TestRange(t *testing.T) {
	repoDir, cleanup := vcstu.InitTempRepo(t)
	defer cleanup()

	ctx := cipd_git.UseGitFinder(context.Background())
	r, err := NewGitInfo(ctx, repoDir, false, false)
	require.NoError(t, err)
	vcstu.TestRange(ctx, t, r)
}
func TestLog(t *testing.T) {
	repoDir, cleanup := vcstu.InitTempRepo(t)
	defer cleanup()

	ctx := cipd_git.UseGitFinder(context.Background())
	r, err := NewGitInfo(ctx, repoDir, false, false)
	if err != nil {
		t.Fatal(err)
	}

	got, err := r.Log(ctx, "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f", "")
	if err != nil {
		t.Fatal(err)
	}
	want := `commit 7a669cfa3f4cd3482a4fd03989f75efcc7595f7f
Author: Joe Gregorio <jcgregorio@google.com>
Date:   Wed Jul 30 08:00:42 2014 -0400

    First "checkin"
    ` + "\n" + `    With quotes.

README.txt
`

	if got != want {
		t.Errorf("Log failed: \nGot  %q \nWant %q", got, want)
	}

	got, err = r.Log(ctx, "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f", "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18")
	if err != nil {
		t.Fatal(err)
	}
	want = `commit 8652a6df7dc8a7e6addee49f6ed3c2308e36bd18
Author: Joe Gregorio <jcgregorio@google.com>
Date:   Wed Jul 30 08:01:55 2014 -0400

    Added code. No body.

README.txt
hello.go
`

	if got != want {
		t.Errorf("Log failed: \nGot  %q \nWant %q", got, want)
	}

}

func TestLogFine(t *testing.T) {
	repoDir, cleanup := vcstu.InitTempRepo(t)
	defer cleanup()

	ctx := cipd_git.UseGitFinder(context.Background())
	r, err := NewGitInfo(ctx, repoDir, false, false)
	if err != nil {
		t.Fatal(err)
	}

	got, err := r.LogFine(ctx, "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f", "", "--format=format:%H")
	if err != nil {
		t.Fatal(err)
	}
	want := `7a669cfa3f4cd3482a4fd03989f75efcc7595f7f`

	if got != want {
		t.Errorf("Log failed: \nGot  %q \nWant %q", got, want)
	}

	got, err = r.LogFine(ctx, "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f", "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18", "--format=format:%H")
	if err != nil {
		t.Fatal(err)
	}
	want = `8652a6df7dc8a7e6addee49f6ed3c2308e36bd18`

	if got != want {
		t.Errorf("Log failed: \nGot  %q \nWant %q", got, want)
	}
}

func TestLogArgs(t *testing.T) {
	repoDir, cleanup := vcstu.InitTempRepo(t)
	defer cleanup()

	ctx := cipd_git.UseGitFinder(context.Background())
	r, err := NewGitInfo(ctx, repoDir, false, false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.LogArgs(ctx, "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f..8652a6df7dc8a7e6addee49f6ed3c2308e36bd18", "--format=format:%H")
	if err != nil {
		t.Fatal(err)
	}
	want := `8652a6df7dc8a7e6addee49f6ed3c2308e36bd18`
	if got != want {
		t.Errorf("Log failed: \nGot  %q \nWant %q", got, want)
	}

}

func TestShortList(t *testing.T) {
	repoDir, cleanup := vcstu.InitTempRepo(t)
	defer cleanup()

	ctx := cipd_git.UseGitFinder(context.Background())
	r, err := NewGitInfo(ctx, repoDir, false, false)
	if err != nil {
		t.Fatal(err)
	}

	l, err := r.ShortList(ctx, "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18", "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(l.Commits), 0; got != want {
		t.Fatalf("Wrong number of zero results: Got %v Want %v", got, want)
	}

	c, err := r.ShortList(ctx, "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f", "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18")
	if err != nil {
		t.Fatal(err)
	}

	expected := []struct {
		Hash    string
		Author  string
		Subject string
	}{
		{
			Hash:    "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18",
			Author:  "Joe Gregorio",
			Subject: "Added code. No body.",
		},
	}
	if got, want := len(c.Commits), len(expected); got != want {
		t.Fatalf("Wrong number of results: Got %v Want %v", got, want)
	}
	for i, o := range c.Commits {
		if got, want := o.Hash, expected[i].Hash; got != want {
			t.Errorf("Wrong hash: Got %v Want %v", got, want)
		}
		if got, want := o.Author, expected[i].Author; got != want {
			t.Errorf("Wrong author: Got %v Want %v", got, want)
		}
		if got, want := o.Subject, expected[i].Subject; got != want {
			t.Errorf("Wrong subject: Got %v Want %v", got, want)
		}
	}
}

func TestRevList(t *testing.T) {
	repoDir, cleanup := vcstu.InitTempRepo(t)
	defer cleanup()

	ctx := cipd_git.UseGitFinder(context.Background())
	r, err := NewGitInfo(ctx, repoDir, false, true)
	if err != nil {
		t.Fatal(err)
	}

	const rev1 = "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f"
	const rev2 = "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18"
	revs := []string{rev2, rev1} // rev-list is reverse-chronological.
	testCases := []struct {
		Input    []string
		Expected []string
	}{
		{
			Input:    []string{git.MasterBranch},
			Expected: revs,
		},
		{
			Input:    []string{"HEAD"},
			Expected: revs,
		},
		{
			Input:    []string{"7a669cf..8652a6d"},
			Expected: []string{rev2},
		},
		{
			Input:    []string{"8652a6d", "^7a669cf"},
			Expected: []string{rev2},
		},
		{
			Input:    []string{"8652a6d..7a669cf"},
			Expected: []string{},
		},
	}
	for _, tc := range testCases {
		actual, err := r.RevList(ctx, tc.Input...)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, tc.Expected, actual, "Failed test for: git rev-list %s\nGot:  %v\nWant: %v", strings.Join(tc.Input, " "), actual, tc.Expected)
	}
}

func TestBranchInfo(t *testing.T) {
	repoDir, cleanup := vcstu.InitTempRepo(t)
	defer cleanup()

	ctx := cipd_git.UseGitFinder(context.Background())
	r, err := NewGitInfo(ctx, repoDir, false, true)
	if err != nil {
		t.Fatal(err)
	}

	allBranches, err := r.GetBranches(ctx)
	require.NoError(t, err)
	branches := []string{}
	for _, b := range allBranches {
		branches = append(branches, b.Name)
	}
	vcstu.TestBranchInfo(ctx, t, r, branches)
}

func TestSetBranch(t *testing.T) {
	repoDir, cleanup := vcstu.InitTempRepo(t)
	defer cleanup()

	ctx := cipd_git.UseGitFinder(context.Background())
	r, err := NewGitInfo(ctx, repoDir, false, true)
	if err != nil {
		t.Fatal(err)
	}

	branches, err := r.GetBranches(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, len(branches))

	err = r.Checkout(ctx, "test-branch-1")
	require.NoError(t, err)

	commits := r.LastN(ctx, 10)
	require.Equal(t, 3, len(commits))
	require.Equal(t, "3f5a807d432ac232a952bbf223bc6952e4b49b2c", commits[2])
}
