package gitiles

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/git/provider"
)

const (
	gitHash       = "abc123"
	secondGitHash = "def456"
	author        = "somebody@example.org"
	subject       = "Some fix for a bug."
	beginHash     = "1111111"
	endHash       = "2222222"
	filename      = "foo.txt"
)

var (
	errMock = errors.New("this is my mock test error")

	commitDetailsForZeroCommits = []*vcsinfo.LongCommit{}

	commitDetailsForOneCommit = []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash:    gitHash,
				Author:  author,
				Subject: subject,
			},
			Body:      "This is the body",
			Timestamp: time.Time{},
		},
	}

	commitDetailsForTwoCommits = []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: gitHash,
			},
		},
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: secondGitHash,
			},
		},
	}
)

func TestLogEntry_HappyPath(t *testing.T) {
	mockRepo := gitiles_mocks.NewGitilesRepo(t)
	mockRepo.On("Log", testutils.AnyContext, gitHash, gitiles.LogLimit(1)).Return(commitDetailsForOneCommit, nil)

	gp := &Gitiles{
		gr: mockRepo,
	}
	entry, err := gp.LogEntry(context.Background(), gitHash)
	require.NoError(t, err)
	expected := "commit abc123\nAuthor somebody@example.org\nDate 01 Jan 01 00:00 +0000\n\nSome fix for a bug.\n\nThis is the body"
	require.Equal(t, expected, entry)
}

func TestLogEntry_GitilesAPIReturnsError_ReturnsError(t *testing.T) {
	mockRepo := gitiles_mocks.NewGitilesRepo(t)
	mockRepo.On("Log", testutils.AnyContext, gitHash, gitiles.LogLimit(1)).Return(nil, errMock)

	gp := &Gitiles{
		gr: mockRepo,
	}
	_, err := gp.LogEntry(context.Background(), gitHash)
	require.ErrorIs(t, err, errMock)
}

func TestLogEntry_ReturnsZeroEntries_ReturnsError(t *testing.T) {
	mockRepo := gitiles_mocks.NewGitilesRepo(t)
	mockRepo.On("Log", testutils.AnyContext, gitHash, gitiles.LogLimit(1)).Return(commitDetailsForZeroCommits, nil)

	gp := &Gitiles{
		gr: mockRepo,
	}
	_, err := gp.LogEntry(context.Background(), gitHash)
	require.Contains(t, err.Error(), "received 0 log entries")
}

func TestGitHashesInRangeForFile_HappyPath(t *testing.T) {
	mockRepo := gitiles_mocks.NewGitilesRepo(t)
	mockRepo.On("Log", testutils.AnyContext, git.LogFromTo(beginHash, endHash), gitiles.LogPath(filename), gitiles.LogReverse()).Return(commitDetailsForTwoCommits, nil)

	gp := &Gitiles{
		gr: mockRepo,
	}
	hashes, err := gp.GitHashesInRangeForFile(context.Background(), beginHash, endHash, filename)
	require.NoError(t, err)
	require.Equal(t, []string{gitHash, secondGitHash}, hashes)
}

func TestGitHashesInRangeForFile_GitilesAPIReturnsError_ReturnsError(t *testing.T) {
	mockRepo := gitiles_mocks.NewGitilesRepo(t)
	mockRepo.On("Log", testutils.AnyContext, git.LogFromTo(beginHash, endHash), gitiles.LogPath(filename), gitiles.LogReverse()).Return(nil, errMock)

	gp := &Gitiles{
		gr: mockRepo,
	}
	_, err := gp.GitHashesInRangeForFile(context.Background(), beginHash, endHash, filename)
	require.ErrorIs(t, err, errMock)
}

func TestGitHashesInRangeForFile_NoGitHashesInRange_ReturnsEmptySlice(t *testing.T) {
	mockRepo := gitiles_mocks.NewGitilesRepo(t)
	mockRepo.On("Log", testutils.AnyContext, git.LogFromTo(beginHash, endHash), gitiles.LogPath(filename), gitiles.LogReverse()).Return(commitDetailsForZeroCommits, nil)

	gp := &Gitiles{
		gr: mockRepo,
	}
	hashes, err := gp.GitHashesInRangeForFile(context.Background(), beginHash, endHash, filename)
	require.NoError(t, err)
	require.Empty(t, hashes)
}

func TestCommitsFromMostRecentGitHashToHead_HappyPath(t *testing.T) {
	mockRepo := gitiles_mocks.NewGitilesRepo(t)
	mockRepo.On("LogFirstParent", testutils.AnyContext, beginHash, "HEAD").Return(commitDetailsForTwoCommits, nil)

	gp := &Gitiles{
		gr: mockRepo,
	}
	cb := func(c provider.Commit) error {
		require.Contains(t, []string{gitHash, secondGitHash}, c.GitHash)
		return nil
	}
	err := gp.CommitsFromMostRecentGitHashToHead(context.Background(), beginHash, cb)
	require.NoError(t, err)
}

func TestCommitsFromMostRecentGitHashToHead_GitilesAPIReturnsError_ReturnsError(t *testing.T) {
	mockRepo := gitiles_mocks.NewGitilesRepo(t)
	mockRepo.On("LogFirstParent", testutils.AnyContext, beginHash, "HEAD").Return(nil, errMock)

	gp := &Gitiles{
		gr: mockRepo,
	}
	cb := func(c provider.Commit) error {
		require.FailNow(t, "should not be called on error")
		return nil
	}
	err := gp.CommitsFromMostRecentGitHashToHead(context.Background(), beginHash, cb)
	require.ErrorIs(t, err, errMock)
}

func TestCommitsFromMostRecentGitHashToHead_CallbackReturnsError_ReturnsError(t *testing.T) {
	mockRepo := gitiles_mocks.NewGitilesRepo(t)
	mockRepo.On("LogFirstParent", testutils.AnyContext, beginHash, "HEAD").Return(commitDetailsForTwoCommits, nil)

	gp := &Gitiles{
		gr: mockRepo,
	}
	cb := func(c provider.Commit) error {
		return errMock
	}
	err := gp.CommitsFromMostRecentGitHashToHead(context.Background(), beginHash, cb)
	require.ErrorIs(t, err, errMock)
	require.Contains(t, err.Error(), "processing callback")
}

func TestUpdate_AlwaysReturnsNil(t *testing.T) {
	gp := &Gitiles{}
	require.NoError(t, gp.Update(context.Background()))
}
