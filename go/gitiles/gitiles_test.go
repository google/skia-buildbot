package gitiles

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/time/rate"
)

func TestLog(t *testing.T) {
	unittest.MediumTest(t)

	// Setup. The test repo looks like this:
	/*
		$ git log --graph --date-order
		* commit c8
		|
		*   commit c7
		|\  Merge: c5 c6
		| |
		| * commit c6
		| |
		* | commit c5
		| |
		* | commit c4
		| |
		| * commit c3
		| |
		| * commit c2
		|/
		|
		* commit c1
		|
		* commit c0
	*/
	ctx := context.Background()
	gb := git_testutils.GitInit(t, ctx)
	now := time.Now()
	c0 := gb.CommitGenAt(ctx, "file1", now)
	now = now.Add(time.Second)
	c1 := gb.CommitGenAt(ctx, "file1", now)
	now = now.Add(time.Second)
	gb.CreateBranchTrackBranch(ctx, "branch2", "master")
	c2 := gb.CommitGenAt(ctx, "file2", now)
	now = now.Add(time.Second)
	c3 := gb.CommitGenAt(ctx, "file2", now)
	now = now.Add(time.Second)
	gb.CheckoutBranch(ctx, "master")
	c4 := gb.CommitGenAt(ctx, "file1", now)
	now = now.Add(time.Second)
	c5 := gb.CommitGenAt(ctx, "file1", now)
	now = now.Add(time.Second)
	gb.CheckoutBranch(ctx, "branch2")
	c6 := gb.CommitGenAt(ctx, "file2", now)
	now = now.Add(time.Second)
	gb.CheckoutBranch(ctx, "master")
	c7 := gb.MergeBranch(ctx, "branch2")
	now = now.Add(time.Second)
	c8 := gb.CommitGenAt(ctx, "file1", now)

	// Use all of the commit variables so the compiler doesn't complain.
	repo := git.GitDir(gb.Dir())
	commits := []string{c8, c7, c6, c5, c4, c3, c2, c1, c0}
	details := make(map[string]*vcsinfo.LongCommit, len(commits))
	for _, c := range commits {
		d, err := repo.Details(ctx, c)
		require.NoError(t, err)
		details[c] = d
	}

	urlMock := mockhttpclient.NewURLMock()
	r := NewRepo(gb.RepoUrl(), urlMock.Client())
	r.rl.SetLimit(rate.Inf)

	// Helper function for mocking gitiles API calls.
	mockLog := func(from, to string, rvCommits []string) {
		// Create the mock results.
		results := &Log{
			Log: make([]*Commit, 0, len(rvCommits)),
		}
		for _, c := range rvCommits {
			d := details[c]
			results.Log = append(results.Log, &Commit{
				Commit:  d.Hash,
				Parents: d.Parents,
				Author: &Author{
					Name:  "don't care",
					Email: "don't care",
					Time:  d.Timestamp.Format(DATE_FORMAT_NO_TZ),
				},
				Committer: &Author{
					Name:  "don't care",
					Email: "don't care",
					Time:  d.Timestamp.Format(DATE_FORMAT_NO_TZ),
				},
				Message: d.Subject,
			})
		}
		js := testutils.MarshalJSON(t, results)
		js = ")]}'\n" + js
		urlMock.MockOnce(fmt.Sprintf(LOG_URL, gb.RepoUrl(), git.LogFromTo(from, to)), mockhttpclient.MockGetDialogue([]byte(js)))
	}

	// Return a slice of the hashes for the given commits.
	hashes := func(commits []*vcsinfo.LongCommit) []string {
		rv := make([]string, 0, len(commits))
		for _, c := range commits {
			rv = append(rv, c.Hash)
		}
		return rv
	}

	// Verify that we got the correct list of commits from the call to Log.
	checkGitiles := func(fn func(context.Context, string, ...LogOption) ([]*vcsinfo.LongCommit, error), from, to string, expect []string) {
		mockLog(from, to, expect)
		log, err := fn(ctx, git.LogFromTo(from, to))
		require.NoError(t, err)
		deepequal.AssertDeepEqual(t, hashes(log), expect)
		require.True(t, urlMock.Empty())
	}

	// Verify that we get the correct list of commits from Git. This is so
	// that we can ensure that our test expectations of the library are
	// consistent with the actual behavior of Git.
	checkGit := func(from, to string, expect []string, args ...string) {
		cmd := []string{"rev-list", "--date-order"}
		cmd = append(cmd, args...)
		cmd = append(cmd, string(git.LogFromTo(from, to)))
		output, err := repo.Git(ctx, cmd...)
		require.NoError(t, err)
		log := strings.Split(strings.TrimSpace(output), "\n")
		// Empty response results in a single-item list with one empty
		// string...
		if len(log) == 1 && log[0] == "" {
			log = log[1:]
		}
		deepequal.AssertDeepEqual(t, log, expect)
	}

	// Verify that we get the expected list of commits from both Gitiles
	// (Repo.Log) and Git ("git log --date-order").
	checkBasic := func(from, to string, expect []string) {
		checkGit(from, to, expect)
		checkGitiles(r.Log, from, to, expect)
	}

	// Verify that we get the expected list of commits from both Gitiles
	// (Repo.LogLinear) and Git
	// ("git log --date-order --first-parent --ancestry-path).
	checkLinear := func(from, to string, expect []string) {
		checkGit(from, to, expect, "--first-parent", "--ancestry-path")
		mockLog(from, to, expect)
		log, err := r.LogLinear(ctx, from, to)
		require.NoError(t, err)
		deepequal.AssertDeepEqual(t, hashes(log), expect)
		require.True(t, urlMock.Empty())
	}

	// Test cases.
	checkBasic(c0, c8, []string{c8, c7, c6, c5, c4, c3, c2, c1})
	checkLinear(c0, c8, []string{c8, c7, c5, c4, c1})
	checkBasic(c0, c1, []string{c1})
	checkLinear(c0, c1, []string{c1})
	checkBasic(c2, c4, []string{c4})
	checkLinear(c2, c4, []string{})
	checkBasic(c1, c2, []string{c2})
	checkLinear(c1, c2, []string{c2})
	checkBasic(c1, c4, []string{c4})
	checkLinear(c1, c4, []string{c4})
	checkBasic(c5, c7, []string{c7, c6, c3, c2})
	checkLinear(c5, c7, []string{c7})
	checkBasic(c2, c7, []string{c7, c6, c5, c4, c3})
	checkLinear(c2, c7, []string{})
}

func TestLogPagination(t *testing.T) {
	unittest.MediumTest(t)

	// Gitiles API paginates logs over 100 commits long.
	ctx := context.Background()
	repoUrl := "https://fake/repo"
	urlMock := mockhttpclient.NewURLMock()
	repo := NewRepo(repoUrl, urlMock.Client())
	repo.rl.SetLimit(rate.Inf)
	next := 0
	hash := func() string {
		next++
		return fmt.Sprintf("%040d", next)
	}
	ts := time.Now().Truncate(time.Second).UTC()
	var last *vcsinfo.LongCommit
	commit := func() *vcsinfo.LongCommit {
		ts = ts.Add(time.Second)
		rv := &vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    hash(),
				Author:  "don't care (don't care)",
				Subject: "don't care",
			},
			Timestamp: ts,
		}
		if last != nil {
			rv.Parents = []string{last.Hash}
		}
		last = rv
		return rv
	}
	mock := func(from, to *vcsinfo.LongCommit, commits []*vcsinfo.LongCommit, start, next string) {
		// Create the mock results.
		results := &Log{
			Log:  make([]*Commit, 0, len(commits)),
			Next: next,
		}
		for i := len(commits) - 1; i >= 0; i-- {
			c := commits[i]
			results.Log = append(results.Log, &Commit{
				Commit:  c.Hash,
				Parents: c.Parents,
				Author: &Author{
					Name:  "don't care",
					Email: "don't care",
					Time:  c.Timestamp.Format(DATE_FORMAT_NO_TZ),
				},
				Committer: &Author{
					Name:  "don't care",
					Email: "don't care",
					Time:  c.Timestamp.Format(DATE_FORMAT_NO_TZ),
				},
				Message: "don't care",
			})
		}
		js := testutils.MarshalJSON(t, results)
		js = ")]}'\n" + js
		url1 := fmt.Sprintf(LOG_URL, repoUrl, git.LogFromTo(from.Hash, to.Hash))
		url2 := fmt.Sprintf(LOG_URL, repoUrl, to.Hash)
		if start != "" {
			url1 += "&s=" + start
			url2 += "&s=" + start
		}
		urlMock.MockOnce(url1, mockhttpclient.MockGetDialogue([]byte(js)))
		urlMock.MockOnce(url2, mockhttpclient.MockGetDialogue([]byte(js)))
	}
	check := func(from, to *vcsinfo.LongCommit, expectCommits []*vcsinfo.LongCommit) {
		// The expectations are in chronological order for convenience
		// of the caller. But git logs are in reverse chronological
		// order. Sort the expectations.
		expect := make([]*vcsinfo.LongCommit, len(expectCommits))
		copy(expect, expectCommits)
		sort.Sort(vcsinfo.LongCommitSlice(expect))

		// Test standard Log(from, to) function.
		log, err := repo.Log(ctx, git.LogFromTo(from.Hash, to.Hash))
		require.NoError(t, err)
		deepequal.AssertDeepEqual(t, expect, log)

		// Test LogFn
		log = make([]*vcsinfo.LongCommit, 0, len(expect))
		require.NoError(t, repo.LogFn(ctx, to.Hash, func(ctx context.Context, c *vcsinfo.LongCommit) error {
			if c.Hash == from.Hash {
				return ErrStopIteration
			}
			log = append(log, c)
			return nil
		}))
		deepequal.AssertDeepEqual(t, expect, log)
		require.True(t, urlMock.Empty())
	}

	// Create some fake commits.
	commits := []*vcsinfo.LongCommit{}
	for i := 0; i < 10; i++ {
		commits = append(commits, commit())
	}

	// Most basic test case; no pagination.
	mock(commits[0], commits[5], commits[1:5], "", "")
	check(commits[0], commits[5], commits[1:5])

	// Two pages.
	split := 5
	mock(commits[0], commits[len(commits)-1], commits[split:], "", commits[split].Hash)
	mock(commits[0], commits[len(commits)-1], commits[1:split], commits[split].Hash, "")
	check(commits[0], commits[len(commits)-1], commits[1:])

	// Three pages.
	split1 := 7
	split2 := 3
	mock(commits[0], commits[len(commits)-1], commits[split1:], "", commits[split1].Hash)
	mock(commits[0], commits[len(commits)-1], commits[split2:split1], commits[split1].Hash, commits[split2].Hash)
	mock(commits[0], commits[len(commits)-1], commits[1:split2], commits[split2].Hash, "")
	check(commits[0], commits[len(commits)-1], commits[1:])
}

func TestListDir(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	repoUrl := "https://skia.googlesource.com/buildbot.git"
	urlMock := mockhttpclient.NewURLMock()
	repo := NewRepo(repoUrl, urlMock.Client())
	repo.rl.SetLimit(rate.Inf)

	resp1 := base64.StdEncoding.EncodeToString([]byte(`100644 blob 573680d74f404d64a7c3441f8a502c007fdcd3b7    gitiles.go
100644 blob c2b8be8049e8503391239bbf00877ebf5880493c    gitiles_test.go
040000 tree 81b1fde7557bd75ad0392143a9d79ed78d0ed4ab    testutils`))
	urlMock.MockOnce(repoUrl+"/+/my/ref/go/gitiles?format=TEXT", mockhttpclient.MockGetDialogue([]byte(resp1)))

	files, dirs, err := repo.ListDirAtRef(ctx, "go/gitiles", "my/ref")
	require.NoError(t, err)
	deepequal.AssertDeepEqual(t, []string{"gitiles.go", "gitiles_test.go"}, files)
	deepequal.AssertDeepEqual(t, []string{"testutils"}, dirs)

	resp2 := `)]}'
{
  "commit": "bbadbbadbbadbbadbbadbbadbbadbbadbbadbbad",
  "tree": "beefbeefbeefbeefbeefbeefbeefbeefbeefbeef",
  "parents": [
    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  ],
  "author": {
    "name": "Me",
    "email": "me@google.com",
    "time": "Tue Oct 15 10:45:49 2019 -0400"
  },
  "committer": {
    "name": "Skia Commit-Bot",
    "email": "skia-commit-bot@chromium.org",
    "time": "Wed Oct 16 17:24:55 2019 +0000"
  },
  "message": "Subject\n\nblah blah blah",
  "tree_diff": []
}
`
	urlMock.MockOnce(repoUrl+"/+/my/other/ref?format=JSON", mockhttpclient.MockGetDialogue([]byte(resp2)))
	urlMock.MockOnce(repoUrl+"/+/bbadbbadbbadbbadbbadbbadbbadbbadbbadbbad/go/gitiles?format=TEXT", mockhttpclient.MockGetDialogue([]byte(resp1)))
	resp3 := base64.StdEncoding.EncodeToString([]byte(`100644 blob 6e5cdd994551045ab24a4246906c7723cb12c12e    testutils.go`))
	urlMock.MockOnce(repoUrl+"/+/bbadbbadbbadbbadbbadbbadbbadbbadbbadbbad/go/gitiles/testutils?format=TEXT", mockhttpclient.MockGetDialogue([]byte(resp3)))
	files, err = repo.ListFilesRecursiveAtRef(ctx, "go/gitiles", "my/other/ref")
	require.NoError(t, err)
	deepequal.AssertDeepEqual(t, []string{"gitiles.go", "gitiles_test.go", "testutils/testutils.go"}, files)
}
