package git_checkout

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/types"
)

var (
	// StartTime is the time of the first commit.
	StartTime = time.Unix(1680000000, 0)
)

// NewForTest returns all the necessary variables needed to test against infra/go/git.
//
// The repo is populated with 8 commits, one minute apart, starting at StartTime.
//
// The hashes for each commit are going to be random and so are returned also.
func NewForTest(t *testing.T) (context.Context, *testutils.GitBuilder, []string, *config.InstanceConfig) {
	ctx := cipd_git.UseGitFinder(context.Background())
	ctx, cancel := context.WithCancel(ctx)

	// Create a git repo for testing purposes.
	gb := testutils.GitInit(t, ctx)
	hashes := []string{}
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(2*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "bar.txt", StartTime.Add(3*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(4*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(5*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "bar.txt", StartTime.Add(6*time.Minute)))
	hashes = append(hashes, gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(7*time.Minute)))

	// Get tmp dir to use for repo checkout.
	tmpDir, err := ioutil.TempDir("", "git")
	require.NoError(t, err)

	// Create the cleanup function.
	t.Cleanup(func() {
		cancel()
		err = os.RemoveAll(tmpDir)
		assert.NoError(t, err)
		gb.Cleanup()
	})

	instanceConfig := &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			URL: gb.Dir(),
			Dir: filepath.Join(tmpDir, "checkout"),
		},
	}
	return ctx, gb, hashes, instanceConfig
}

func TestParseGitRevLogStream_Success(t *testing.T) {
	r := strings.NewReader(
		`commit 6079a7810530025d9877916895dd14eb8bb454c0
Joe Gregorio <joe@bitworking.org>
Change #9
1584837783`)

	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p provider.Commit) error {
		assert.Equal(t, provider.Commit{
			CommitNumber: types.BadCommitNumber,
			GitHash:      "6079a7810530025d9877916895dd14eb8bb454c0",
			Timestamp:    1584837783,
			Author:       "Joe Gregorio <joe@bitworking.org>",
			Subject:      "Change #9"}, p)
		return nil
	})
	assert.NoError(t, err)
}

func TestParseGitRevLogStream_ErrPropagatesWhenCallbackReturnsError(t *testing.T) {
	r := strings.NewReader(
		`commit 6079a7810530025d9877916895dd14eb8bb454c0
Joe Gregorio <joe@bitworking.org>
Change #9
1584837783`)

	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p provider.Commit) error {
		return fmt.Errorf("This is an error.")
	})
	assert.Contains(t, err.Error(), "This is an error.")
}

func TestParseGitRevLogStream_SuccessForTwoCommits(t *testing.T) {
	r := strings.NewReader(
		`commit 6079a7810530025d9877916895dd14eb8bb454c0
Joe Gregorio <joe@bitworking.org>
Change #9
1584837783
commit 977e0ef44bec17659faf8c5d4025c5a068354817
Joe Gregorio <joe@bitworking.org>
Change #8
1584837780`)
	count := 0
	hashes := []string{"6079a7810530025d9877916895dd14eb8bb454c0", "977e0ef44bec17659faf8c5d4025c5a068354817"}
	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p provider.Commit) error {
		assert.Equal(t, "Joe Gregorio <joe@bitworking.org>", p.Author)
		assert.Equal(t, hashes[count], p.GitHash)
		count++
		return nil
	})
	assert.Equal(t, 2, count)
	assert.NoError(t, err)
}

func TestParseGitRevLogStream_EmptyFile_Success(t *testing.T) {
	r := strings.NewReader("")
	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p provider.Commit) error {
		assert.Fail(t, "Should never get here.")
		return nil
	})
	assert.NoError(t, err)
}

func TestParseGitRevLogStream_ErrMissingTimestamp(t *testing.T) {
	r := strings.NewReader(
		`commit 6079a7810530025d9877916895dd14eb8bb454c0
Joe Gregorio <joe@bitworking.org>
Change #9`)
	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p provider.Commit) error {
		assert.Fail(t, "Should never get here.")
		return nil
	})
	assert.Contains(t, err.Error(), "expecting a timestamp")
}

func TestParseGitRevLogStream_ErrFailedToParseTimestamp(t *testing.T) {
	r := strings.NewReader(
		`commit 6079a7810530025d9877916895dd14eb8bb454c0
Joe Gregorio <joe@bitworking.org>
Change #9
ooops 1584837780`)
	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p provider.Commit) error {
		assert.Fail(t, "Should never get here.")
		return nil
	})
	assert.Contains(t, err.Error(), "Failed to parse timestamp")
}

func TestParseGitRevLogStream_ErrMissingSubject(t *testing.T) {
	r := strings.NewReader(
		`commit 6079a7810530025d9877916895dd14eb8bb454c0
Joe Gregorio <joe@bitworking.org>`)
	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p provider.Commit) error {
		assert.Fail(t, "Should never get here.")
		return nil
	})
	assert.Contains(t, err.Error(), "expecting a subject")
}

func TestParseGitRevLogStream_ErrMissingAuthor(t *testing.T) {
	r := strings.NewReader(
		`commit 6079a7810530025d9877916895dd14eb8bb454c0`)
	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p provider.Commit) error {
		assert.Fail(t, "Should never get here.")
		return nil
	})
	assert.Contains(t, err.Error(), "expecting an author")
}

func TestParseGitRevLogStream_ErrMalformedCommitLine(t *testing.T) {
	r := strings.NewReader(
		`something_not_commit 6079a7810530025d9877916895dd14eb8bb454c0`)
	err := parseGitRevLogStream(ioutil.NopCloser(r), func(p provider.Commit) error {
		assert.Fail(t, "Should never get here.")
		return nil
	})
	assert.Contains(t, err.Error(), "expected commit at")
}

func TestLogEntry_Success(t *testing.T) {
	ctx, _, hashes, instanceConfig := NewForTest(t)
	g, err := New(ctx, instanceConfig)
	require.NoError(t, err)

	got, err := g.LogEntry(ctx, hashes[1])
	require.NoError(t, err)
	expected := `commit 881dfc43620250859549bb7e0301b6910d9b8e70
Author: test <test@google.com>
Date:   Tue Mar 28 10:41:00 2023 +0000

    501233450539197794
`
	require.Equal(t, expected, got)
}

func TestLogEntry_BadCommitId_ReturnsError(t *testing.T) {
	ctx, _, _, instanceConfig := NewForTest(t)
	g, err := New(ctx, instanceConfig)
	require.NoError(t, err)

	_, err = g.LogEntry(ctx, "this-is-not-a-known-git-hash")
	require.Error(t, err)
}

func TestUpdate_SuccessAndNewCommitAppears(t *testing.T) {
	ctx, gb, _, instanceConfig := NewForTest(t)
	g, err := New(ctx, instanceConfig)
	require.NoError(t, err)

	_, err = g.LogEntry(ctx, "this-is-not-a-known-git-hash")
	require.Error(t, err)

	newHash := gb.CommitGenAt(ctx, "foo.txt", StartTime.Add(4*time.Minute))

	err = g.Update(ctx)
	require.NoError(t, err)
	_, err = g.LogEntry(ctx, newHash)
	require.NoError(t, err)
}

func TestGitHashesInRangeForFile_FileIsChangedAtBeginHash_BeginHashIsExcludedFromResponse(t *testing.T) {
	// The 'bar.txt' file is only changed commit 3 and 6.
	ctx, _, hashes, instanceConfig := NewForTest(t)
	g, err := New(ctx, instanceConfig)
	require.NoError(t, err)

	// GitHashesInRangeForFile is exclusive of 'begin', so it should not be in
	// the results.
	changedAt, err := g.GitHashesInRangeForFile(ctx, hashes[3], hashes[7], "bar.txt")
	require.NoError(t, err)
	require.Equal(t, []string{hashes[6]}, changedAt)
}

func TestGitHashesInRangeForFile_BeginHashIsEmpty_SearchGoesToBeginningOfRepoHistory(t *testing.T) {
	// The 'bar.txt' file is only changed commit 3 and 6.
	ctx, _, hashes, instanceConfig := NewForTest(t)
	g, err := New(ctx, instanceConfig)
	require.NoError(t, err)

	changedAt, err := g.GitHashesInRangeForFile(ctx, "", hashes[7], "bar.txt")
	require.NoError(t, err)
	require.Equal(t, []string{hashes[3], hashes[6]}, changedAt)
}

func TestGitHashesInRangeForFile_BeginHashIsEmptyButStartCommitIsSet_SearchGoesToBeginningOfRepoHistory(t *testing.T) {
	// The 'bar.txt' file is only changed commit 3 and 6.
	ctx, _, hashes, instanceConfig := NewForTest(t)
	// We change the StartCommit to 3, so we should only see the change at 6.
	instanceConfig.GitRepoConfig.StartCommit = hashes[3]
	g, err := New(ctx, instanceConfig)
	require.NoError(t, err)

	changedAt, err := g.GitHashesInRangeForFile(ctx, "", hashes[7], "bar.txt")
	require.NoError(t, err)
	require.Equal(t, []string{hashes[6]}, changedAt)
}

func TestCommitsFromMostRecentGitHashToHead_ProvideEmptyGitHash_ReceiveAllHashesInRepo(t *testing.T) {
	ctx, _, hashes, instanceConfig := NewForTest(t)
	g, err := New(ctx, instanceConfig)
	require.NoError(t, err)

	err = g.CommitsFromMostRecentGitHashToHead(ctx, "", func(c provider.Commit) error {
		require.Equal(t, hashes[0], c.GitHash)
		hashes = hashes[1:]
		return nil
	})
	require.NoError(t, err)
}

func TestCommitsFromMostRecentGitHashToHead_ProvideEmptyGitHashButStartCommitIsSet_ReceiveAllHashesInRepoStartingFromStartCommit(t *testing.T) {
	ctx, _, hashes, instanceConfig := NewForTest(t)
	instanceConfig.GitRepoConfig.StartCommit = hashes[2]
	g, err := New(ctx, instanceConfig)
	require.NoError(t, err)

	// StartCommit is set to 2, so we should get all commits after that.
	expected := hashes[3:]
	err = g.CommitsFromMostRecentGitHashToHead(ctx, "", func(c provider.Commit) error {
		require.Equal(t, expected[0], c.GitHash)
		expected = expected[1:]
		return nil
	})
	require.NoError(t, err)
}

func TestCommitsFromMostRecentGitHashToHead_ProvideNonEmptyGitHash_ReceiveAllNewerHashesInRepo(t *testing.T) {
	ctx, _, hashes, instanceConfig := NewForTest(t)
	g, err := New(ctx, instanceConfig)
	require.NoError(t, err)

	// Note we use 3 here, because we pass hashes[2] below, so
	// CommitsFromMostRecentGitHashToHead will return all commits newer than
	// hashes[2] exclusive.
	expected := hashes[3:]
	err = g.CommitsFromMostRecentGitHashToHead(ctx, hashes[2], func(c provider.Commit) error {
		require.Equal(t, expected[0], c.GitHash)
		expected = expected[1:]
		return nil
	})
	require.NoError(t, err)
}
