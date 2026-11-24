// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package deployment_status

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitiles"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

func makeLongCommit(hash string, parents []string, ts time.Time, subject string) *vcsinfo.LongCommit {
	return &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    hash,
			Subject: subject,
		},
		Parents:   parents,
		Timestamp: ts,
	}
}

func makeMemRepographRepo(t *testing.T, repoURL string, commits []*vcsinfo.LongCommit, mainHead string) *RepographRepo {
	t.Helper()
	ctx := context.Background()
	commitsByHash := make(map[string]*vcsinfo.LongCommit, len(commits))
	for _, c := range commits {
		commitsByHash[c.Hash] = c
	}
	branches := []*git.Branch{{Name: git.MainBranch, Head: mainHead}}
	ri := repograph.NewMemCacheRepoImpl(commitsByHash, branches)
	graph, err := repograph.NewWithRepoImpl(ctx, ri)
	require.NoError(t, err)
	require.NoError(t, graph.Update(ctx))

	gr, err := gitiles.NewRepoWithClient(repoURL, mockhttpclient.NewURLMock().Client())
	require.NoError(t, err)
	return NewRepographRepo(gr, graph)
}

func TestRepographRepo_Details(t *testing.T) {
	ctx := context.Background()
	const (
		c1 = "1111111111111111111111111111111111111111"
		c2 = "abcdef1111111111111111111111111111111111"
		c3 = "abcdef2222222222222222222222222222222222"
	)
	now := time.Now().UTC()
	rr := makeMemRepographRepo(t, "https://skia.googlesource.com/buildbot.git", []*vcsinfo.LongCommit{
		makeLongCommit(c1, nil, now.Add(-2*time.Hour), "first"),
		makeLongCommit(c2, []string{c1}, now.Add(-1*time.Hour), "second"),
		makeLongCommit(c3, []string{c2}, now, "third"),
	}, c3)

	// Full hash match.
	got, err := rr.Details(ctx, c2)
	require.NoError(t, err)
	require.Equal(t, c2, got.Hash)

	// Unique short prefix, trimmed and case-insensitive.
	got, err = rr.Details(ctx, "  ABCDEF1 ")
	require.NoError(t, err)
	require.Equal(t, c2, got.Hash)

	// Ambiguous short prefix matches both c2 and c3.
	_, err = rr.Details(ctx, "abcdef")
	require.Error(t, err)
	require.ErrorIs(t, err, errAmbiguousCommitPrefix)

	// Unknown full hash.
	_, err = rr.Details(ctx, "9999999999999999999999999999999999999999")
	require.Error(t, err)
	require.NotErrorIs(t, err, errAmbiguousCommitPrefix)

	// Unknown short prefix.
	_, err = rr.Details(ctx, "deadbeef")
	require.Error(t, err)

	// Non-hex ref.
	_, err = rr.Details(ctx, "not-a-hash")
	require.Error(t, err)
}

func TestRepographRepo_LogFirstParentAndIsAncestor(t *testing.T) {
	ctx := context.Background()
	const (
		c1 = "1111111111111111111111111111111111111111"
		c2 = "2222222222222222222222222222222222222222"
		c3 = "3333333333333333333333333333333333333333"
	)
	now := time.Now().UTC()
	rr := makeMemRepographRepo(t, "https://skia.googlesource.com/buildbot.git", []*vcsinfo.LongCommit{
		makeLongCommit(c1, nil, now.Add(-2*time.Hour), "first"),
		makeLongCommit(c2, []string{c1}, now.Add(-1*time.Hour), "second"),
		makeLongCommit(c3, []string{c2}, now, "third"),
	}, c3)

	ok, err := rr.IsAncestor(ctx, c1, c3)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = rr.IsAncestor(ctx, c3, c1)
	require.NoError(t, err)
	require.False(t, ok)

	log, err := rr.LogFirstParent(ctx, c1, git.MainBranch)
	require.NoError(t, err)
	require.Len(t, log, 2)
	require.Equal(t, c3, log[0].Hash)
	require.Equal(t, c2, log[1].Hash)
}

func TestFindCommit(t *testing.T) {
	ctx := context.Background()
	const (
		c1 = "abcdef1111111111111111111111111111111111"
		c2 = "abcdef2222222222222222222222222222222222"
		c3 = "3333333333333333333333333333333333333333"
	)
	now := time.Now().UTC()
	rr1 := makeMemRepographRepo(t, "https://skia.googlesource.com/buildbot.git", []*vcsinfo.LongCommit{
		makeLongCommit(c1, nil, now.Add(-time.Hour), "one"),
		makeLongCommit(c2, []string{c1}, now, "two"),
	}, c2)
	rr2 := makeMemRepographRepo(t, "https://skia.googlesource.com/skia.git", []*vcsinfo.LongCommit{
		makeLongCommit(c3, nil, now, "three"),
	}, c3)

	// Invalid hash rejected before hitting repos.
	_, _, err := findCommit(ctx, []GitRepo{rr1, rr2}, "not-hex!")
	require.Error(t, err)

	// Found in second repo.
	c, repo, err := findCommit(ctx, []GitRepo{rr1, rr2}, c3)
	require.NoError(t, err)
	require.Equal(t, c3, c.Hash)
	require.Equal(t, rr2, repo)

	// Ambiguous prefix in first repo is preserved rather than overwritten by rr2.
	_, _, err = findCommit(ctx, []GitRepo{rr1, rr2}, "abcdef")
	require.Error(t, err)
	require.ErrorIs(t, err, errAmbiguousCommitPrefix)
}

func TestGetMostRecentCommit(t *testing.T) {
	ctx := context.Background()
	const (
		c1 = "1111111111111111111111111111111111111111"
		c2 = "2222222222222222222222222222222222222222"
		c3 = "3333333333333333333333333333333333333333"
		c4 = "4444444444444444444444444444444444444444"
	)
	now := time.Now().UTC()
	lc1 := makeLongCommit(c1, nil, now.Add(-3*time.Hour), "c1")
	lc2 := makeLongCommit(c2, []string{c1}, now.Add(-2*time.Hour), "c2")
	lc3 := makeLongCommit(c3, []string{c2}, now.Add(-1*time.Hour), "c3")

	t.Run("empty", func(t *testing.T) {
		require.Nil(t, getMostRecentCommit(ctx, nil, nil))
	})

	t.Run("single hash", func(t *testing.T) {
		repo := gitiles_mocks.NewGitilesRepo(t)
		repo.On("Details", testutils.AnyContext, c1).Return(lc1, nil).Once()
		got := getMostRecentCommit(ctx, []GitRepo{repo}, []string{c1})
		require.Equal(t, lc1, got)
	})

	t.Run("picks newest commit from LogFirstParent", func(t *testing.T) {
		repo := gitiles_mocks.NewGitilesRepo(t)
		repo.On("Details", testutils.AnyContext, c1).Return(lc1, nil).Once()
		// LogFirstParent returns newest-first (c3, c2).
		repo.On("LogFirstParent", testutils.AnyContext, c1, git.MainBranch).Return([]*vcsinfo.LongCommit{lc3, lc2}, nil).Once()
		got := getMostRecentCommit(ctx, []GitRepo{repo}, []string{c1, c2, c3})
		require.Equal(t, lc3, got)
	})

	t.Run("falls back to startCommit when none newer on first-parent path", func(t *testing.T) {
		repo := gitiles_mocks.NewGitilesRepo(t)
		repo.On("Details", testutils.AnyContext, c3).Return(lc3, nil).Once()
		repo.On("LogFirstParent", testutils.AnyContext, c3, git.MainBranch).Return([]*vcsinfo.LongCommit{}, nil).Once()
		got := getMostRecentCommit(ctx, []GitRepo{repo}, []string{c3, c1})
		require.Equal(t, lc3, got)
	})

	t.Run("falls back to startCommit when LogFirstParent fails", func(t *testing.T) {
		repo := gitiles_mocks.NewGitilesRepo(t)
		repo.On("Details", testutils.AnyContext, c2).Return(lc2, nil).Once()
		repo.On("LogFirstParent", testutils.AnyContext, c2, git.MainBranch).Return(nil, errors.New("not on main")).Once()
		repo.On("URL").Return("https://skia.googlesource.com/buildbot.git").Once()
		got := getMostRecentCommit(ctx, []GitRepo{repo}, []string{c2, c3})
		require.Equal(t, lc2, got)
	})

	t.Run("skips unknown hashes until one resolves", func(t *testing.T) {
		repo := gitiles_mocks.NewGitilesRepo(t)
		repo.On("Details", testutils.AnyContext, c4).Return(nil, errors.New("not found")).Once()
		repo.On("Details", testutils.AnyContext, c2).Return(lc2, nil).Once()
		repo.On("LogFirstParent", testutils.AnyContext, c2, git.MainBranch).Return([]*vcsinfo.LongCommit{lc3}, nil).Once()
		got := getMostRecentCommit(ctx, []GitRepo{repo}, []string{c4, c2})
		require.Equal(t, lc2, got)
	})
}
