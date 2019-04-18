// Package vcsinfo/testutils contains a set of tests to test vcsinfo.VCS implementations.
package testutils

import (
	"context"
	"fmt"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

// InitTempRepo creates a temporary git repository from ./testdata/testrepo.zip.
// It returns the path to the repo directory and a cleanup function that should
// be called in a deferred.
func InitTempRepo() (string, func()) {
	tr := util.NewTempRepo()
	sklog.Infof("YYY: %s", tr.Dir)
	return tr.Dir, tr.Cleanup
}

func TestDisplay(t testutils.TestingT, vcs vcsinfo.VCS) {
	ctx := context.Background()
	// All hashes refer to the repository in ./testdata/testrepo.zip unzipped by NewTempRepo().
	testCases := []struct {
		hash    string
		author  string
		subject string
	}{
		{
			hash:    "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f",
			author:  "Joe Gregorio (jcgregorio@google.com)",
			subject: "First \"checkin\"",
		},
		{
			hash:    "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18",
			author:  "Joe Gregorio (jcgregorio@google.com)",
			subject: "Added code. No body.",
		},
	}
	for _, tc := range testCases {
		details, err := vcs.Details(ctx, tc.hash, false)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := details.Author, tc.author; got != want {
			t.Errorf("Details author mismatch: Got %q, Want %q", got, want)
		}
		if got, want := details.Subject, tc.subject; got != want {
			t.Errorf("Details subject mismatch: Got %q, Want %q", got, want)
		}
	}
}

func TestFrom(t testutils.TestingT, vcs vcsinfo.VCS) {
	// All timestamps refer to the repository in ./testdata/testrepo.zip unzipped by NewTempRepo().
	// The two commits in the master branch of the repo have timestamps:
	// 1406721715 and 1406721642.
	testCases := []struct {
		ts     int64
		length int
	}{
		{
			ts:     1406721715,
			length: 0,
		},
		{
			ts:     1406721714,
			length: 1,
		},
		{
			ts:     1406721642,
			length: 1,
		},
		{
			ts:     1406721641,
			length: 2,
		},
	}
	for _, tc := range testCases {
		hashes := vcs.From(time.Unix(tc.ts, 0))
		if got, want := len(hashes), tc.length; got != want {
			t.Errorf("For ts: %d Length returned is wrong: Got %d Want %d", tc.ts, got, want)
		}
	}
}

func TestByIndex(t testutils.TestingT, vcs vcsinfo.VCS) {
	// All hashes refer to the repository in ./testdata/testrepo.zip unzipped by NewTempRepo().
	ctx := context.Background()
	commit, err := vcs.ByIndex(ctx, 0)
	assert.NoError(t, err)
	assert.Equal(t, "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f", commit.Hash)
	commit, err = vcs.ByIndex(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18", commit.Hash)
	commit, err = vcs.ByIndex(ctx, -1)
	assert.Error(t, err)
}

func TestLastNIndex(t testutils.TestingT, vcs vcsinfo.VCS) {
	// All hashes refer to the repository in ./testdata/testrepo.zip unzipped by NewTempRepo().
	c1 := &vcsinfo.IndexCommit{
		Hash:      "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f",
		Index:     0,
		Timestamp: time.Unix(1406721642, 0).UTC(),
	}
	c2 := &vcsinfo.IndexCommit{
		Hash:      "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18",
		Index:     1,
		Timestamp: time.Unix(1406721715, 0).UTC(),
	}
	testCases := []struct {
		n        int
		expected []*vcsinfo.IndexCommit
	}{
		{
			n:        0,
			expected: []*vcsinfo.IndexCommit{},
		},
		{
			n:        1,
			expected: []*vcsinfo.IndexCommit{c2},
		},
		{
			n:        2,
			expected: []*vcsinfo.IndexCommit{c1, c2},
		},
		{
			n:        5,
			expected: []*vcsinfo.IndexCommit{c1, c2},
		},
	}
	for _, tc := range testCases {
		actual := vcs.LastNIndex(tc.n)
		assert.Equal(t, len(tc.expected), len(actual))
		deepequal.AssertDeepEqual(t, tc.expected, actual)
	}
}

func TestIndexOf(t testutils.TestingT, vcs vcsinfo.VCS) {
	// All hashes refer to the repository in ./testdata/testrepo.zip unzipped by NewTempRepo().
	ctx := context.Background()
	idx, err := vcs.IndexOf(ctx, "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f")
	assert.NoError(t, err)
	assert.Equal(t, 0, idx)
	idx, err = vcs.IndexOf(ctx, "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18")
	assert.NoError(t, err)
	assert.Equal(t, 1, idx)
	idx, err = vcs.IndexOf(ctx, "foo")
	assert.Error(t, err)
}

func TestRange(t testutils.TestingT, vcs vcsinfo.VCS) {
	// All hashes refer to the repository in ./testdata/testrepo.zip unzipped by NewTempRepo().
	ts1 := time.Unix(1406721642, 0).UTC()
	ts2 := time.Unix(1406721715, 0).UTC()

	c1 := &vcsinfo.IndexCommit{
		Hash:      "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f",
		Index:     0,
		Timestamp: ts1,
	}
	c2 := &vcsinfo.IndexCommit{
		Hash:      "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18",
		Index:     1,
		Timestamp: ts2,
	}
	testCases := []struct {
		begin    time.Time
		end      time.Time
		expected []*vcsinfo.IndexCommit
		message  string
	}{
		{
			begin:    ts1.Add(-5 * time.Second),
			end:      ts1.Add(-4 * time.Second),
			expected: []*vcsinfo.IndexCommit{},
			message:  "No match, too early",
		},
		{
			begin:    ts1.Add(4 * time.Second),
			end:      ts1.Add(5 * time.Second),
			expected: []*vcsinfo.IndexCommit{},
			message:  "No match, too late",
		},
		{
			begin:    ts2.Add(-1 * time.Millisecond),
			end:      ts2,
			expected: []*vcsinfo.IndexCommit{},
			message:  "Test the end of the half open interval.",
		},
		{
			begin:    ts2,
			end:      ts2.Add(1 * time.Millisecond),
			expected: []*vcsinfo.IndexCommit{c2},
			message:  "Test the beginning of the half open interval.",
		},
		{
			begin:    ts1,
			end:      ts2.Add(1 * time.Millisecond),
			expected: []*vcsinfo.IndexCommit{c1, c2},
			message:  "Test just a little past the second value.",
		},
		{
			begin:    ts1.Add(-1 * time.Second),
			end:      ts2.Add(5 * time.Second),
			expected: []*vcsinfo.IndexCommit{c1, c2},
			message:  "Test larger margins.",
		},
	}
	for idx, tc := range testCases {
		actual := vcs.Range(tc.begin, tc.end)
		assert.Equal(t, len(tc.expected), len(actual), fmt.Sprintf("%d %#v", idx, tc))
		deepequal.AssertDeepEqual(t, tc.expected, actual)
	}
}

func TestBranchInfo(t assert.TestingT, vcs vcsinfo.VCS, branches []string) {
	// All hashes refer to the repository in ./testdata/testrepo.zip unzipped by NewTempRepo().
	ctx := context.Background()
	assert.Equal(t, 2, len(branches))

	// Make sure commits across all branches show up.
	commits := []string{
		"7a669cfa3f4cd3482a4fd03989f75efcc7595f7f",
		"8652a6df7dc8a7e6addee49f6ed3c2308e36bd18",
		"3f5a807d432ac232a952bbf223bc6952e4b49b2c",
	}
	found := vcs.From(time.Unix(1406721641, 0))
	assert.Equal(t, commits, found)

	// The timestamps of the three commits commits in the entire repository start
	// at timestamp 1406721642.
	testCases := []struct {
		commitHash string
		branchName string
		branches   map[string]bool
	}{
		{
			commitHash: "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18",
			branchName: "master",
			branches:   map[string]bool{"master": true, "test-branch-1": true},
		},
		{
			commitHash: "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f",
			branchName: "master",
			branches:   map[string]bool{"master": true, "test-branch-1": true},
		},
		{
			commitHash: "3f5a807d432ac232a952bbf223bc6952e4b49b2c",
			branchName: "test-branch-1",
			branches:   map[string]bool{"test-branch-1": true},
		},
	}

	for _, tc := range testCases {
		details, err := vcs.Details(ctx, tc.commitHash, true)
		assert.NoError(t, err)
		assert.True(t, details.Branches[tc.branchName])
		assert.Equal(t, tc.branches, details.Branches)
	}
}
