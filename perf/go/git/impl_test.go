// Package git is the minimal interface that Perf need to interact with a Git
// repo.
package git

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/types"
)

func TestSpannerDB(t *testing.T) {
	for name, subTest := range subTests {
		t.Run(name, func(t *testing.T) {
			ctx, db, gb, hashes, _, instanceConfig := gittest.NewForTest(t)
			g, err := New(ctx, false, db, instanceConfig)
			require.NoError(t, err)

			subTest(t, ctx, g, gb, hashes)
		})
	}
}

// subTestFunction is a func we will call to test one aspect of *SQLTraceStore.
type subTestFunction func(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string)

// subTests are all the tests we have for *SQLTraceStore.
var subTests = map[string]subTestFunction{
	"testDetails_FailOnBadCommitNumber":                                                  testDetails_FailOnBadCommitNumber,
	"testDetails_Success":                                                                testDetails_Success,
	"testCommitSliceFromCommitNumberSlice_EmptyInputSlice_Success":                       testCommitSliceFromCommitNumberSlice_EmptyInputSlice_Success,
	"testCommitSliceFromCommitNumberSlice_Success":                                       testCommitSliceFromCommitNumberSlice_Success,
	"testUpdate_NewCommitsAreFoundFromGitHashAfterUpdate":                                testUpdate_NewCommitsAreFoundFromGitHashAfterUpdate,
	"testUpdate_UpdateCommitWithoutCommitPosition_NoCommitAddedToDB":                     testUpdate_UpdateCommitWithoutCommitPosition_NoCommitAddedToDB,
	"testCommitNumberFromGitHash_Success":                                                testCommitNumberFromGitHash_Success,
	"testCommitNumberFromGitHash_ErrorOnUnknownGitHash":                                  testCommitNumberFromGitHash_ErrorOnUnknownGitHash,
	"testCommitNumberFromTime_Success":                                                   testCommitNumberFromTime_Success,
	"testCommitNumberFromTime_ErrorOnTimeTooOld":                                         testCommitNumberFromTime_ErrorOnTimeTooOld,
	"testCommitNumberFromTime_SuccessOnZeroTime":                                         testCommitNumberFromTime_SuccessOnZeroTime,
	"testCommitSliceFromTimeRange_Success":                                               testCommitSliceFromTimeRange_Success,
	"testCommitSliceFromTimeRange_ZeroWidthRangeReturnsOneResult":                        testCommitSliceFromTimeRange_ZeroWidthRangeReturnsOneResult,
	"testCommitSliceFromTimeRange_NegativeWidthRangeReturnsZeroResults":                  testCommitSliceFromTimeRange_NegativeWidthRangeReturnsZeroResults,
	"testCommitSliceFromCommitNumberRange_Success":                                       testCommitSliceFromCommitNumberRange_Success,
	"testCommitSliceFromCommitNumberRange_ZeroWidthReturnsOneResult":                     testCommitSliceFromCommitNumberRange_ZeroWidthReturnsOneResult,
	"testCommitSliceFromCommitNumberRange_NegativeWidthReturnsZeroResults":               testCommitSliceFromCommitNumberRange_NegativeWidthReturnsZeroResults,
	"testGitHashFromCommitNumber_Success":                                                testGitHashFromCommitNumber_Success,
	"testGitHashFromCommitNumber_ErrWhenCommitDoesntExist":                               testGitHashFromCommitNumber_ErrWhenCommitDoesntExist,
	"testCommitNumbersWhenFileChangesInCommitNumberRange_Success":                        testCommitNumbersWhenFileChangesInCommitNumberRange_Success,
	"testCommitNumbersWhenFileChangesInCommitNumberRange_EmptySliceIfFileDoesntExist":    testCommitNumbersWhenFileChangesInCommitNumberRange_EmptySliceIfFileDoesntExist,
	"testCommitNumbersWhenFileChangesInCommitNumberRange_RangeIsInclusiveOfBegin":        testCommitNumbersWhenFileChangesInCommitNumberRange_RangeIsInclusiveOfBegin,
	"testCommitNumbersWhenFileChangesInCommitNumberRange_RangeIsInclusiveOfEnd":          testCommitNumbersWhenFileChangesInCommitNumberRange_RangeIsInclusiveOfEnd,
	"testCommitNumbersWhenFileChangesInCommitNumberRange_ResultsWhenBeginEqualsEnd":      testCommitNumbersWhenFileChangesInCommitNumberRange_ResultsWhenBeginEqualsEnd,
	"testCommitNumbersWhenFileChangesInCommitNumberRange_HandlesZeroAsBeginCommitNumber": testCommitNumbersWhenFileChangesInCommitNumberRange_HandlesZeroAsBeginCommitNumber,
	"testLogEntry_Success":                                                               testLogEntry_Success,
	"testLogEntry_BadCommitId_ReturnsError":                                              testLogEntry_BadCommitId_ReturnsError,
	"testPreviousCommitNumberFromCommitNumber_Success":                                   testPreviousCommitNumberFromCommitNumber_Success,
	"testPreviousCommitNumberFromCommitNumber_UnknownCommit_Error":                       testPreviousCommitNumberFromCommitNumber_UnknownCommit_Error,
	"testPreviousCommitNumberFromCommitNumber_NoPreviousCommit_Error":                    testPreviousCommitNumberFromCommitNumber_NoPreviousCommit_Error,
	"testPreviousGitHashFromCommitNumber_Success":                                        testPreviousGitHashFromCommitNumber_Success,
	"testPreviousGitHashFromCommitNumber_UnknownCommit_Error":                            testPreviousGitHashFromCommitNumber_UnknownCommit_Error,
	"testPreviousGitHashFromCommitNumber_NoPreviousCommit_Error":                         testPreviousGitHashFromCommitNumber_NoPreviousCommit_Error,
}

func testUpdate_NewCommitsAreFoundFromGitHashAfterUpdate(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	// Add new commit to repo, it shouldn't appear in the database.
	newHash := gb.CommitGenAt(ctx, "foo.txt", gittest.StartTime.Add(4*time.Minute))
	_, err := g.CommitNumberFromGitHash(ctx, newHash)
	require.Error(t, err)

	// After Update step we should find it.
	err = g.Update(ctx)
	require.NoError(t, err)
	commitNumber, err := g.CommitNumberFromGitHash(ctx, newHash)
	assert.Equal(t, types.CommitNumber(len(hashes)), commitNumber)
}

func testUpdate_UpdateCommitWithoutCommitPosition_NoCommitAddedToDB(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	newHash := gb.CommitGenAt(ctx, "foo.txt", gittest.StartTime.Add(4*time.Minute))
	_, err := g.CommitNumberFromGitHash(ctx, newHash)
	require.Error(t, err)

	g.repoSuppliedCommitNumber = true
	g.commitNumberRegex = regexp.MustCompile("Cr-Commit-Position: refs/heads/(main|master)@\\{#(.*)\\}")
	err = g.Update(ctx)
	require.NoError(t, err)
	commitNumber, err := g.CommitNumberFromGitHash(ctx, newHash)
	assert.Equal(t, types.BadCommitNumber, commitNumber)
}

func testCommitNumberFromGitHash_Success(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commitNumber, err := g.CommitNumberFromGitHash(ctx, hashes[0])
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(0), commitNumber)
	commitNumber, err = g.CommitNumberFromGitHash(ctx, hashes[2])
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(2), commitNumber)
}

func testPreviousCommitNumberFromCommitNumber_Success(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commitNumber, err := g.PreviousCommitNumberFromCommitNumber(ctx, types.CommitNumber(1))
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(0), commitNumber)
	commitNumber, err = g.PreviousCommitNumberFromCommitNumber(ctx, types.CommitNumber(7))
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(6), commitNumber)
}

func testPreviousCommitNumberFromCommitNumber_UnknownCommit_Error(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commitNumber, err := g.PreviousCommitNumberFromCommitNumber(ctx, types.BadCommitNumber)
	assert.Error(t, err)
	assert.Equal(t, types.BadCommitNumber, commitNumber)
}

func testPreviousCommitNumberFromCommitNumber_NoPreviousCommit_Error(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commitNumber, err := g.PreviousCommitNumberFromCommitNumber(ctx, types.CommitNumber(0))
	assert.Error(t, err)
	assert.Equal(t, types.BadCommitNumber, commitNumber)
}

func testPreviousGitHashFromCommitNumber_Success(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	githash, err := g.PreviousGitHashFromCommitNumber(ctx, types.CommitNumber(1))
	assert.NoError(t, err)
	assert.Equal(t, hashes[0], githash)
	githash, err = g.PreviousGitHashFromCommitNumber(ctx, types.CommitNumber(6))
	assert.NoError(t, err)
	assert.Equal(t, hashes[5], githash)
}

func testPreviousGitHashFromCommitNumber_UnknownCommit_Error(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	githash, err := g.PreviousGitHashFromCommitNumber(ctx, types.BadCommitNumber)
	assert.Error(t, err)
	assert.Equal(t, "", githash)
}

func testPreviousGitHashFromCommitNumber_NoPreviousCommit_Error(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	githash, err := g.PreviousGitHashFromCommitNumber(ctx, types.CommitNumber(0))
	assert.Error(t, err)
	assert.Equal(t, "", githash)
}

func testDetails_FailOnBadCommitNumber(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	_, err := g.CommitFromCommitNumber(ctx, types.BadCommitNumber)
	require.Error(t, err)
}

func testDetails_Success(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commitNumber := types.CommitNumber(1)
	assert.False(t, g.cache.Contains(commitNumber))
	commit, err := g.CommitFromCommitNumber(ctx, commitNumber)
	require.NoError(t, err)

	// The prefix of the URL will change, so just confirm it has the right suffix.
	require.True(t, strings.HasSuffix(commit.URL, commit.GitHash))

	assert.Equal(t, provider.Commit{
		Timestamp:    gittest.StartTime.Add(time.Minute).Unix(),
		GitHash:      hashes[1],
		Author:       "test <test@google.com>",
		Subject:      "501233450539197794",
		URL:          commit.URL,
		CommitNumber: commitNumber,
	}, commit)
	assert.True(t, g.cache.Contains(commitNumber))
}

func testCommitSliceFromCommitNumberSlice_EmptyInputSlice_Success(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	resp, err := g.CommitSliceFromCommitNumberSlice(ctx, []types.CommitNumber{})
	require.NoError(t, err)
	assert.Empty(t, resp)
}

func testCommitSliceFromCommitNumberSlice_Success(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commitNumbers := []types.CommitNumber{1, 3}
	assert.False(t, g.cache.Contains(commitNumbers[0]))
	assert.False(t, g.cache.Contains(commitNumbers[1]))
	commits, err := g.CommitSliceFromCommitNumberSlice(ctx, commitNumbers)
	require.NoError(t, err)

	// The prefix of the URLs will change, so just confirm it has the right suffix.
	require.True(t, strings.HasSuffix(commits[0].URL, commits[0].GitHash))
	require.True(t, strings.HasSuffix(commits[1].URL, commits[1].GitHash))

	assert.Equal(t, provider.Commit{
		Timestamp:    gittest.StartTime.Add(time.Minute).Unix(),
		GitHash:      hashes[1],
		Author:       "test <test@google.com>",
		Subject:      "501233450539197794",
		URL:          commits[0].URL,
		CommitNumber: commitNumbers[0],
	}, commits[0])
	assert.Equal(t, provider.Commit{
		Timestamp:    gittest.StartTime.Add(3 * time.Minute).Unix(),
		GitHash:      hashes[3],
		Author:       "test <test@google.com>",
		Subject:      "6044372234677422456",
		URL:          commits[1].URL,
		CommitNumber: commitNumbers[1],
	}, commits[1])

	assert.True(t, g.cache.Contains(commitNumbers[0]))
	assert.True(t, g.cache.Contains(commitNumbers[1]))
}

func testCommitNumberFromGitHash_ErrorOnUnknownGitHash(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	_, err := g.CommitNumberFromGitHash(ctx, hashes[0]+"obviously_not_a_valid_hash")
	assert.Error(t, err)
}

func testCommitNumberFromTime_Success(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	// Exact time of commit number 1. See gittest.NewForTest().
	commitNumber, err := g.CommitNumberFromTime(ctx, gittest.StartTime.Add(1*time.Minute))
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(1), commitNumber)

	// A second beyond commit number 1.
	commitNumber, err = g.CommitNumberFromTime(ctx, gittest.StartTime.Add(1*time.Minute+time.Second))
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(1), commitNumber)

	// A second before commit number 1.
	commitNumber, err = g.CommitNumberFromTime(ctx, gittest.StartTime.Add(1*time.Minute-time.Second))
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(0), commitNumber)
}

func testCommitNumberFromTime_SuccessOnZeroTime(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commitNumber, err := g.CommitNumberFromTime(ctx, time.Time{})
	assert.NoError(t, err)
	assert.Equal(t, types.CommitNumber(len(hashes)-1), commitNumber)
}

func testCommitNumberFromTime_ErrorOnTimeTooOld(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commitNumber, err := g.CommitNumberFromTime(ctx, gittest.StartTime.Add(-1*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, types.BadCommitNumber, commitNumber)
}

func testCommitSliceFromTimeRange_Success(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commits, err := g.CommitSliceFromTimeRange(ctx, gittest.StartTime.Add(1*time.Minute), gittest.StartTime.Add(3*time.Minute))
	require.NoError(t, err)
	assert.Len(t, commits, 3)
	assert.Equal(t, int64(1680000060), commits[0].Timestamp)
	assert.Equal(t, types.CommitNumber(1), commits[0].CommitNumber)
	assert.Equal(t, int64(1680000120), commits[1].Timestamp)
	assert.Equal(t, types.CommitNumber(2), commits[1].CommitNumber)
	assert.Contains(t, commits[1].URL, "+show/497e33d39ae58fa3339f67b9366f887a4c72871c")
}

func testCommitSliceFromTimeRange_ZeroWidthRangeReturnsOneResult(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commits, err := g.CommitSliceFromTimeRange(ctx, gittest.StartTime.Add(1*time.Minute), gittest.StartTime.Add(1*time.Minute))
	require.NoError(t, err)
	require.Len(t, commits, 1)
	assert.Equal(t, int64(1680000060), commits[0].Timestamp)
}

func testCommitSliceFromTimeRange_NegativeWidthRangeReturnsZeroResults(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commits, err := g.CommitSliceFromTimeRange(ctx, gittest.StartTime.Add(2*time.Minute), gittest.StartTime.Add(1*time.Minute))
	require.NoError(t, err)
	assert.Empty(t, commits)
}

func testCommitSliceFromCommitNumberRange_Success(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commits, err := g.CommitSliceFromCommitNumberRange(ctx, 1, 2)
	require.NoError(t, err)
	require.Len(t, commits, 2)
	assert.Equal(t, int64(1680000060), commits[0].Timestamp)
	assert.Equal(t, types.CommitNumber(1), commits[0].CommitNumber)
	assert.Equal(t, int64(1680000120), commits[1].Timestamp)
	assert.Equal(t, types.CommitNumber(2), commits[1].CommitNumber)
}

func testCommitSliceFromCommitNumberRange_ZeroWidthReturnsOneResult(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commits, err := g.CommitSliceFromCommitNumberRange(ctx, 2, 2)
	require.NoError(t, err)
	require.Len(t, commits, 1)
	assert.Equal(t, int64(1680000120), commits[0].Timestamp)
	assert.Equal(t, types.CommitNumber(2), commits[0].CommitNumber)
}

func testCommitSliceFromCommitNumberRange_NegativeWidthReturnsZeroResults(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commits, err := g.CommitSliceFromCommitNumberRange(ctx, 3, 2)
	require.NoError(t, err)
	require.Empty(t, commits)
}

func testGitHashFromCommitNumber_Success(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	gitHash, err := g.GitHashFromCommitNumber(ctx, types.CommitNumber(2))
	require.NoError(t, err)
	assert.Equal(t, hashes[2], gitHash)
}

func testGitHashFromCommitNumber_ErrWhenCommitDoesntExist(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	_, err := g.GitHashFromCommitNumber(ctx, types.BadCommitNumber)
	require.Error(t, err)
}

func testCommitNumbersWhenFileChangesInCommitNumberRange_Success(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commits, err := g.CommitNumbersWhenFileChangesInCommitNumberRange(ctx, types.CommitNumber(1), types.CommitNumber(7), "bar.txt")
	require.NoError(t, err)
	assert.Equal(t, []types.CommitNumber{3, 6}, commits)
}

func testCommitNumbersWhenFileChangesInCommitNumberRange_EmptySliceIfFileDoesntExist(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commits, err := g.CommitNumbersWhenFileChangesInCommitNumberRange(ctx, types.CommitNumber(1), types.CommitNumber(7), "this-file-doesnt-exist.txt")
	require.NoError(t, err)
	assert.Empty(t, commits)
}

func testCommitNumbersWhenFileChangesInCommitNumberRange_RangeIsInclusiveOfBegin(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commits, err := g.CommitNumbersWhenFileChangesInCommitNumberRange(ctx, types.CommitNumber(3), types.CommitNumber(7), "bar.txt")
	require.NoError(t, err)
	assert.Equal(t, []types.CommitNumber{3, 6}, commits)
}

func testCommitNumbersWhenFileChangesInCommitNumberRange_RangeIsInclusiveOfEnd(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commits, err := g.CommitNumbersWhenFileChangesInCommitNumberRange(ctx, types.CommitNumber(5), types.CommitNumber(6), "bar.txt")
	require.NoError(t, err)
	assert.Equal(t, []types.CommitNumber{6}, commits)
}

func testCommitNumbersWhenFileChangesInCommitNumberRange_ResultsWhenBeginEqualsEnd(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commits, err := g.CommitNumbersWhenFileChangesInCommitNumberRange(ctx, types.CommitNumber(6), types.CommitNumber(6), "bar.txt")
	require.NoError(t, err)
	assert.Equal(t, []types.CommitNumber{6}, commits)
}

func testCommitNumbersWhenFileChangesInCommitNumberRange_HandlesZeroAsBeginCommitNumber(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	commits, err := g.CommitNumbersWhenFileChangesInCommitNumberRange(ctx, types.CommitNumber(0), types.CommitNumber(4), "bar.txt")
	require.NoError(t, err)
	assert.Equal(t, []types.CommitNumber{3}, commits)
}

func testLogEntry_Success(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	got, err := g.LogEntry(ctx, types.CommitNumber(1))
	require.NoError(t, err)
	expected := `commit 881dfc43620250859549bb7e0301b6910d9b8e70
Author: test <test@google.com>
Date:   Tue Mar 28 10:41:00 2023 +0000

    501233450539197794
`
	require.Equal(t, expected, got)
}

func testLogEntry_BadCommitId_ReturnsError(t *testing.T, ctx context.Context, g *Impl, gb *testutils.GitBuilder, hashes []string) {
	_, err := g.LogEntry(ctx, types.BadCommitNumber)
	require.Error(t, err)
}

func TestURLFromParts_DebounceCommitURL_Success(t *testing.T) {

	const debounceURL = "https://some.other.url.example.org"
	instanceConfig := &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			URL:              "https://skia.googlesource.com/skia",
			DebouceCommitURL: true,
		},
	}
	commit := provider.Commit{
		GitHash: "6079a7810530025d9877916895dd14eb8bb454c0",
		Subject: debounceURL,
	}
	assert.Equal(t, debounceURL, urlFromParts(instanceConfig, commit))
}

func TestURLFromParts_CommitURLSupplied_Success(t *testing.T) {

	instanceConfig := &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			URL:       "https://github.com/google/skia",
			CommitURL: "%s/commit/%s",
		},
	}
	commit := provider.Commit{
		GitHash: "6079a7810530025d9877916895dd14eb8bb454c0",
	}
	assert.Equal(t, "https://github.com/google/skia/commit/6079a7810530025d9877916895dd14eb8bb454c0", urlFromParts(instanceConfig, commit))
}

func TestURLFromParts_DefaultCommitURL_Success(t *testing.T) {

	instanceConfig := &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			URL: "https://skia.googlesource.com/skia",
		},
	}
	commit := provider.Commit{
		GitHash: "6079a7810530025d9877916895dd14eb8bb454c0",
	}
	assert.Equal(t, "https://skia.googlesource.com/skia/+show/6079a7810530025d9877916895dd14eb8bb454c0", urlFromParts(instanceConfig, commit))
}

func TestCommit_Display(t *testing.T) {

	c := provider.Commit{
		CommitNumber: 10223,
		GitHash:      "d261e1075a93677442fdf7fe72aba7e583863664",
		Timestamp:    1498176000,
		Author:       "Robert Phillips <robertphillips@google.com>",
		Subject:      "Re-enable opList dependency tracking",
		URL:          "https://skia.googlesource.com/skia/+show/d261e1075a93677442fdf7fe72aba7e583863664",
	}
	assert.Equal(t, "d261e10 -  2y 40w - Re-enable opList dependency tracking", c.Display(time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC)))
}

func TestGetCommitNumberFromGitLog(t *testing.T) {
	for name, subTest := range getCommitNumberSubTests {
		t.Run(name, func(t *testing.T) {
			ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
			g, err := New(ctx, false, db, instanceConfig)
			require.NoError(t, err)

			g.commitNumberRegex = regexp.MustCompile("Cr-Commit-Position: refs/heads/(main|master)@\\{#(.*)\\}")

			subTest.SubTestFunction(t, subTest.body, g)
		})
	}
}

// getCommitNumberSubTestFunction is a func we will call to test one aspect of *SQLTraceStore.
type getCommitNumberSubTestFunction func(t *testing.T, body string, g *Impl)

// subTests are all the tests we have for *SQLTraceStore.
var getCommitNumberSubTests = map[string]struct {
	SubTestFunction getCommitNumberSubTestFunction
	body            string
}{
	"testGetCommitNumberFromCommit_Master_Success": {testGetCommitNumberFromCommit_Master_Success, `Bug: 1030266
		Change-Id: I08e3f59e0a3d03ce77b6f669e1cfa1a72fae2ad1
		Reviewed-on: https://chromium-review.googlesource.com/c/chromium/src/+/1985760
		Reviewed-by: Brian White \u003cbcwhite@chromium.org\u003e
		Commit-Queue: Michael van Ouwerkerk \u003cmvanouwerkerk@chromium.org\u003e
		Cr-Commit-Position: refs/heads/master@{#727989}
		`},
	"testGetCommitNumberFromCommit_Main_Success": {testGetCommitNumberFromCommit_Main_Success, `Bug: 1030266
		Change-Id: I08e3f59e0a3d03ce77b6f669e1cfa1a72fae2ad1
		Reviewed-on: https://chromium-review.googlesource.com/c/chromium/src/+/1985760
		Reviewed-by: Brian White \u003cbcwhite@chromium.org\u003e
		Commit-Queue: Michael van Ouwerkerk \u003cmvanouwerkerk@chromium.org\u003e
		Cr-Commit-Position: refs/heads/main@{#727990}
		`},
	"testGetCommitNumberFromCommit_OtherBranch_ReturnsError": {testGetCommitNumberFromCommit_OtherBranch_ReturnsError, `Bug: 1030266
		Change-Id: I08e3f59e0a3d03ce77b6f669e1cfa1a72fae2ad1
		Reviewed-on: https://chromium-review.googlesource.com/c/chromium/src/+/1985760
		Reviewed-by: Brian White \u003cbcwhite@chromium.org\u003e
		Commit-Queue: Michael van Ouwerkerk \u003cmvanouwerkerk@chromium.org\u003e
		Cr-Commit-Position: refs/heads/otherbranch@{#727990}
		`},
	"testGetCommitNumberFromCommit_NoCommitNumber_ReturnsError": {testGetCommitNumberFromCommit_NoCommitNumber_ReturnsError, `Bug: 1030266
		Change-Id: I08e3f59e0a3d03ce77b6f669e1cfa1a72fae2ad1
		`},
	"testGetCommitNumberFromCommit_InvalidCommitNumber_ReturnsError": {testGetCommitNumberFromCommit_InvalidCommitNumber_ReturnsError, `Bug: 1030266
		Change-Id: I08e3f59e0a3d03ce77b6f669e1cfa1a72fae2ad1
		Reviewed-on: https://chromium-review.googlesource.com/c/chromium/src/+/1985760
		Reviewed-by: Brian White \u003cbcwhite@chromium.org\u003e
		Commit-Queue: Michael van Ouwerkerk \u003cmvanouwerkerk@chromium.org\u003e
		Cr-Commit-Position: refs/heads/master@{#727a989}
		`},
	"testGetCommitNumberFromCommit_MultipleCommitNumbers_GetTheLastCommitNumber": {testGetCommitNumberFromCommit_MultipleCommitNumbers_GetTheLastCommitNumber, `> Bug: 1411197
		> Bug: 1375174
		> Change-Id: I113036c11d2c29b902577220243001b818bc940f
		> Reviewed-on: https://chromium-review.googlesource.com/c/chromium/src/+/4173488
		> Reviewed-by: Yoshisato Yanagisawa <yyanagisawa@chromium.org>
		> Reviewed-by: Hiroki Nakagawa <nhiroki@chromium.org>
		> Reviewed-by: Shunya Shishido <sisidovski@chromium.org>
		> Reviewed-by: Takashi Toyoshima <toyoshim@chromium.org>
		> Commit-Queue: Minoru Chikamune <chikamune@chromium.org>
		> Reviewed-by: Kenichi Ishibashi <bashi@chromium.org>
		> Cr-Commit-Position: refs/heads/main@{#1100878}

		Bug: 1412756
		Bug: 1411197
		Bug: 1375174
		Change-Id: Ib03e7ceb51858576956dc713017adbe7b133be05
		Reviewed-on: https://chromium-review.googlesource.com/c/chromium/src/+/4222534
		Reviewed-by: Kenichi Ishibashi <bashi@chromium.org>
		Commit-Queue: Minoru Chikamune <chikamune@chromium.org>
		Reviewed-by: Hiroki Nakagawa <nhiroki@chromium.org>
		Reviewed-by: Shunya Shishido <sisidovski@chromium.org>
		Reviewed-by: Takashi Toyoshima <toyoshim@chromium.org>
		Cr-Commit-Position: refs/heads/main@{#1101482}
		`},
}

func testGetCommitNumberFromCommit_Master_Success(t *testing.T, body string, g *Impl) {
	commitNumber, err := g.getCommitNumberFromCommit(body)
	require.NoError(t, err)
	assert.Equal(t, types.CommitNumber(727989), commitNumber)
}

func testGetCommitNumberFromCommit_Main_Success(t *testing.T, body string, g *Impl) {
	commitNumber, err := g.getCommitNumberFromCommit(body)
	require.NoError(t, err)
	assert.Equal(t, types.CommitNumber(727990), commitNumber)
}

func testGetCommitNumberFromCommit_OtherBranch_ReturnsError(t *testing.T, body string, g *Impl) {
	commitNumber, err := g.getCommitNumberFromCommit(body)
	require.Error(t, err)
	assert.Equal(t, types.BadCommitNumber, commitNumber)
}

func testGetCommitNumberFromCommit_NoCommitNumber_ReturnsError(t *testing.T, body string, g *Impl) {
	commitNumber, err := g.getCommitNumberFromCommit(body)
	require.Error(t, err)
	assert.Equal(t, types.BadCommitNumber, commitNumber)
}

func testGetCommitNumberFromCommit_InvalidCommitNumber_ReturnsError(t *testing.T, body string, g *Impl) {
	commitNumber, err := g.getCommitNumberFromCommit(body)
	require.Error(t, err)
	assert.Equal(t, types.BadCommitNumber, commitNumber)
}

func testGetCommitNumberFromCommit_MultipleCommitNumbers_GetTheLastCommitNumber(t *testing.T, body string, g *Impl) {
	commitNumber, err := g.getCommitNumberFromCommit(body)
	require.NoError(t, err)
	assert.Equal(t, types.CommitNumber(1101482), commitNumber)
}
