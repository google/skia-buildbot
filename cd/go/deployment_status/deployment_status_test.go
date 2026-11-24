// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package deployment_status

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/cd/go/stages"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/docker"
	docker_mocks "go.skia.org/infra/go/docker/mocks"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

func mockGerritChange(t *testing.T, urlMock *mockhttpclient.URLMock, gerritURL string, ci *gerrit.ChangeInfo) {
	t.Helper()
	u := gerritURL + "/a" + fmt.Sprintf(gerrit.URLTmplChange, fmt.Sprintf("%d", ci.Issue))
	b, err := json.Marshal(ci)
	require.NoError(t, err)
	b = append([]byte(")]}'\n"), b...)
	urlMock.MockOnce(u, mockhttpclient.MockGetDialogue(b))
}

func mockGerritNotFound(urlMock *mockhttpclient.URLMock, gerritURL string, issueID int64) {
	u := gerritURL + "/a" + fmt.Sprintf(gerrit.URLTmplChange, fmt.Sprintf("%d", issueID))
	urlMock.MockOnce(u, mockhttpclient.MockGetError("not found", http.StatusNotFound))
}

func newTestChecker(c *http.Client, repos []GitRepo, entries []Entry) *Checker {
	return &Checker{
		c:             c,
		repos:         repos,
		cachedEntries: entries,
		lastUpdated:   time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC),
		depsCache:     map[string]deps_parser.DepsEntries{},
		clCache:       map[string]string{},
	}
}

func TestResolveCommitOrCL(t *testing.T) {
	ctx := context.Background()
	const (
		repoURL   = "https://skia.googlesource.com/buildbot.git"
		gerritURL = "https://skia-review.googlesource.com"
		c1        = "1234567000000000000000000000000000000001"
		c2        = "abcdef1000000000000000000000000000000002"
		c3        = "abcdef2000000000000000000000000000000003"
	)
	now := time.Now().UTC()
	rr := makeMemRepographRepo(t, repoURL, []*vcsinfo.LongCommit{
		makeLongCommit(c1, nil, now.Add(-2*time.Hour), "all-digit prefix"),
		makeLongCommit(c2, []string{c1}, now.Add(-1*time.Hour), "two"),
		makeLongCommit(c3, []string{c2}, now, "three"),
	}, c3)

	t.Run("full and short commit hash", func(t *testing.T) {
		chk := newTestChecker(mockhttpclient.NewURLMock().Client(), []GitRepo{rr}, nil)
		c, repo, err := chk.resolveCommitOrCL(ctx, c2)
		require.NoError(t, err)
		require.Equal(t, c2, c.Hash)
		require.Equal(t, rr, repo)

		c, repo, err = chk.resolveCommitOrCL(ctx, "abcdef1")
		require.NoError(t, err)
		require.Equal(t, c2, c.Hash)
		require.Equal(t, rr, repo)
	})

	t.Run("ambiguous prefix and unknown commit return clean errors", func(t *testing.T) {
		chk := newTestChecker(mockhttpclient.NewURLMock().Client(), []GitRepo{rr}, nil)
		_, _, err := chk.resolveCommitOrCL(ctx, "abcdef")
		require.Error(t, err)
		require.Equal(t, `ambiguous commit prefix "abcdef"`, err.Error())

		_, _, err = chk.resolveCommitOrCL(ctx, "deadbeef")
		require.Error(t, err)
		require.Equal(t, `unknown commit "deadbeef"`, err.Error())
	})

	t.Run("numeric CL ID resolved and cached", func(t *testing.T) {
		urlMock := mockhttpclient.NewURLMock()
		mockGerritChange(t, urlMock, gerritURL, &gerrit.ChangeInfo{
			Issue:  54321,
			Status: gerrit.ChangeStatusMerged,
			Revisions: map[string]*gerrit.Revision{
				c1: {Number: 1},
				c2: {Number: 2},
			},
		})
		chk := newTestChecker(urlMock.Client(), []GitRepo{rr}, nil)

		c, repo, err := chk.resolveCommitOrCL(ctx, "54321")
		require.NoError(t, err)
		require.Equal(t, c2, c.Hash)
		require.Equal(t, rr, repo)
		require.True(t, urlMock.Empty())

		// Second call hits clCache without another Gerrit request.
		c, _, err = chk.resolveCommitOrCL(ctx, "54321")
		require.NoError(t, err)
		require.Equal(t, c2, c.Hash)
	})

	t.Run("all-digit short commit hash falls back when Gerrit CL lookup fails", func(t *testing.T) {
		urlMock := mockhttpclient.NewURLMock()
		mockGerritNotFound(urlMock, gerritURL, 1234567)
		chk := newTestChecker(urlMock.Client(), []GitRepo{rr}, nil)

		c, repo, err := chk.resolveCommitOrCL(ctx, "1234567")
		require.NoError(t, err)
		require.Equal(t, c1, c.Hash)
		require.Equal(t, rr, repo)
		require.True(t, urlMock.Empty())
	})

	t.Run("gerrit change URL", func(t *testing.T) {
		urlMock := mockhttpclient.NewURLMock()
		mockGerritChange(t, urlMock, gerritURL, &gerrit.ChangeInfo{
			Issue:  99999,
			Status: gerrit.ChangeStatusMerged,
			Revisions: map[string]*gerrit.Revision{
				c3: {Number: 1},
			},
		})
		chk := newTestChecker(urlMock.Client(), []GitRepo{rr}, nil)

		c, repo, err := chk.resolveCommitOrCL(ctx, "https://skia-review.googlesource.com/c/buildbot/+/99999")
		require.NoError(t, err)
		require.Equal(t, c3, c.Hash)
		require.Equal(t, rr, repo)
		require.True(t, urlMock.Empty())
	})

	t.Run("unmerged CL and invalid ref return clean messages without stack traces", func(t *testing.T) {
		urlMock := mockhttpclient.NewURLMock()
		mockGerritChange(t, urlMock, gerritURL, &gerrit.ChangeInfo{
			Issue:  77777,
			Status: gerrit.ChangeStatusNew,
			Revisions: map[string]*gerrit.Revision{
				c1: {Number: 1},
			},
		})
		chk := newTestChecker(urlMock.Client(), []GitRepo{rr}, nil)

		_, _, err := chk.resolveCommitOrCL(ctx, "77777")
		require.Error(t, err)
		require.Equal(t, "CL 77777 is not merged (status: NEW)", err.Error())
		require.NotContains(t, err.Error(), ". At ")

		_, _, err = chk.resolveCommitOrCL(ctx, "not-a-valid-ref")
		require.Error(t, err)
		require.Equal(t, `invalid commit hash, CL number, or Gerrit URL "not-a-valid-ref"`, err.Error())
		require.NotContains(t, err.Error(), ". At ")
	})
}

func TestGetDeps(t *testing.T) {
	const (
		commitHash = "1111111111111111111111111111111111111111"
		depHash    = "2222222222222222222222222222222222222222"
	)
	depsContent := fmt.Sprintf("deps = {\n  'skia': 'https://skia.googlesource.com/skia.git@%s',\n}\n", depHash)
	readmeInfo, err := git.MakeFileInfo("README.md", "100644", git.ObjectTypeBlob, 0)
	require.NoError(t, err)
	depsInfo, err := git.MakeFileInfo(deps_parser.DepsFileName, "100644", git.ObjectTypeBlob, 0)
	require.NoError(t, err)

	t.Run("does not cache transient read or list errors", func(t *testing.T) {
		repo := gitiles_mocks.NewGitilesRepo(t)
		// First call: both ReadFileAtRef and ListDirAtRef fail (e.g. replication-delay 404).
		repo.On("ReadFileAtRef", testutils.AnyContext, deps_parser.DepsFileName, commitHash).Return(nil, errors.New("transient 404")).Once()
		repo.On("ListDirAtRef", testutils.AnyContext, ".", commitHash).Return(nil, errors.New("transient 404")).Once()
		// Second call: ReadFileAtRef fails transiently, but ListDirAtRef shows DEPS exists.
		repo.On("ReadFileAtRef", testutils.AnyContext, deps_parser.DepsFileName, commitHash).Return(nil, errors.New("transient 500")).Once()
		repo.On("ListDirAtRef", testutils.AnyContext, ".", commitHash).Return([]os.FileInfo{readmeInfo, depsInfo}, nil).Once()
		// Third call: ReadFileAtRef succeeds and gets cached.
		repo.On("ReadFileAtRef", testutils.AnyContext, deps_parser.DepsFileName, commitHash).Return([]byte(depsContent), nil).Once()

		chk := newTestChecker(nil, []GitRepo{repo}, nil)

		_, err := chk.getDeps(repo, commitHash)
		require.Error(t, err)

		_, err = chk.getDeps(repo, commitHash)
		require.Error(t, err)

		deps, err := chk.getDeps(repo, commitHash)
		require.NoError(t, err)
		entry := deps.Get("https://skia.googlesource.com/skia.git")
		require.NotNil(t, entry)
		require.Equal(t, depHash, entry.Version)

		// Fourth call uses the cache (no further Gitiles calls).
		deps2, err := chk.getDeps(repo, commitHash)
		require.NoError(t, err)
		require.Equal(t, deps, deps2)
	})

	t.Run("caches empty deps when root listing confirms no DEPS file", func(t *testing.T) {
		repo := gitiles_mocks.NewGitilesRepo(t)
		repo.On("ReadFileAtRef", testutils.AnyContext, deps_parser.DepsFileName, commitHash).Return(nil, errors.New("404")).Once()
		repo.On("ListDirAtRef", testutils.AnyContext, ".", commitHash).Return([]os.FileInfo{readmeInfo}, nil).Once()

		chk := newTestChecker(nil, []GitRepo{repo}, nil)

		deps, err := chk.getDeps(repo, commitHash)
		require.NoError(t, err)
		require.Empty(t, deps)

		// Second call hits cache without calling ReadFileAtRef or ListDirAtRef again.
		deps2, err := chk.getDeps(repo, commitHash)
		require.NoError(t, err)
		require.Empty(t, deps2)
	})
}

func TestIsAncestorOrTransitiveDep(t *testing.T) {
	ctx := context.Background()
	const (
		urlA = "https://skia.googlesource.com/a.git"
		urlB = "https://skia.googlesource.com/b.git"
		urlC = "https://skia.googlesource.com/c.git"

		a1 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1"
		b1 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb1"
		b2 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb2"
		c1 = "ccccccccccccccccccccccccccccccccccccccc1"
		c2 = "ccccccccccccccccccccccccccccccccccccccc2"
	)
	now := time.Now().UTC()
	lcA1 := makeLongCommit(a1, nil, now, "a1")
	lcB1 := makeLongCommit(b1, nil, now, "b1")
	lcB2 := makeLongCommit(b2, []string{b1}, now, "b2")
	lcC1 := makeLongCommit(c1, nil, now, "c1")
	lcC2 := makeLongCommit(c2, []string{c1}, now, "c2")

	t.Run("same repo direct and error", func(t *testing.T) {
		repoB := gitiles_mocks.NewGitilesRepo(t)
		repoB.On("URL").Return(urlB)
		repoB.On("IsAncestor", testutils.AnyContext, b1, b2).Return(true, nil).Once()
		repoB.On("IsAncestor", testutils.AnyContext, b2, b1).Return(false, errors.New("rpc failed")).Once()

		chk := newTestChecker(nil, []GitRepo{repoB}, nil)

		ok, err := chk.isAncestorOrTransitiveDep(ctx, lcB1, repoB, lcB1, repoB, map[string]bool{})
		require.NoError(t, err)
		require.True(t, ok)

		ok, err = chk.isAncestorOrTransitiveDep(ctx, lcB2, repoB, lcB1, repoB, map[string]bool{})
		require.NoError(t, err)
		require.True(t, ok)

		ok, err = chk.isAncestorOrTransitiveDep(ctx, lcB1, repoB, lcB2, repoB, map[string]bool{})
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("transitive DEPS A -> B -> C", func(t *testing.T) {
		repoA := gitiles_mocks.NewGitilesRepo(t)
		repoB := gitiles_mocks.NewGitilesRepo(t)
		repoC := gitiles_mocks.NewGitilesRepo(t)
		repoA.On("URL").Return(urlA)
		repoB.On("URL").Return(urlB)
		repoC.On("URL").Return(urlC)

		depsA := fmt.Sprintf("deps = {\n  'b': '%s@%s',\n}\n", urlB, b2)
		depsB := fmt.Sprintf("deps = {\n  'c': '%s@%s',\n}\n", urlC, c2)
		repoA.On("ReadFileAtRef", testutils.AnyContext, deps_parser.DepsFileName, a1).Return([]byte(depsA), nil).Once()
		repoB.On("Details", testutils.AnyContext, b2).Return(lcB2, nil).Once()
		repoB.On("ReadFileAtRef", testutils.AnyContext, deps_parser.DepsFileName, b2).Return([]byte(depsB), nil).Once()
		repoC.On("Details", testutils.AnyContext, c2).Return(lcC2, nil).Once()
		repoC.On("IsAncestor", testutils.AnyContext, c1, c2).Return(true, nil).Once()

		chk := newTestChecker(nil, []GitRepo{repoA, repoB, repoC}, nil)
		ok, err := chk.isAncestorOrTransitiveDep(ctx, lcA1, repoA, lcC1, repoC, map[string]bool{})
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("DEPS cycle terminates via visited set", func(t *testing.T) {
		repoA := gitiles_mocks.NewGitilesRepo(t)
		repoB := gitiles_mocks.NewGitilesRepo(t)
		repoC := gitiles_mocks.NewGitilesRepo(t)
		repoA.On("URL").Return(urlA)
		repoB.On("URL").Return(urlB)
		repoC.On("URL").Return(urlC)

		depsA := fmt.Sprintf("deps = {\n  'b': '%s@%s',\n}\n", urlB, b1)
		depsB := fmt.Sprintf("deps = {\n  'a': '%s@%s',\n}\n", urlA, a1)
		repoA.On("ReadFileAtRef", testutils.AnyContext, deps_parser.DepsFileName, a1).Return([]byte(depsA), nil).Once()
		repoB.On("Details", testutils.AnyContext, b1).Return(lcB1, nil).Once()
		repoB.On("ReadFileAtRef", testutils.AnyContext, deps_parser.DepsFileName, b1).Return([]byte(depsB), nil).Once()
		repoA.On("Details", testutils.AnyContext, a1).Return(lcA1, nil).Once()

		chk := newTestChecker(nil, []GitRepo{repoA, repoB, repoC}, nil)
		ok, err := chk.isAncestorOrTransitiveDep(ctx, lcA1, repoA, lcC1, repoC, map[string]bool{})
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("DEPS lookup error propagates as indeterminate", func(t *testing.T) {
		repoA := gitiles_mocks.NewGitilesRepo(t)
		repoC := gitiles_mocks.NewGitilesRepo(t)
		repoA.On("URL").Return(urlA)
		repoC.On("URL").Return(urlC)
		repoA.On("ReadFileAtRef", testutils.AnyContext, deps_parser.DepsFileName, a1).Return(nil, errors.New("transient error")).Once()
		repoA.On("ListDirAtRef", testutils.AnyContext, ".", a1).Return(nil, errors.New("transient error")).Once()

		chk := newTestChecker(nil, []GitRepo{repoA, repoC}, nil)
		ok, err := chk.isAncestorOrTransitiveDep(ctx, lcA1, repoA, lcC1, repoC, map[string]bool{})
		require.Error(t, err)
		require.False(t, ok)
	})

	t.Run("repo without DEPS file returns false with no error", func(t *testing.T) {
		readmeInfo, err := git.MakeFileInfo("README.md", "100644", git.ObjectTypeBlob, 0)
		require.NoError(t, err)

		repoA := gitiles_mocks.NewGitilesRepo(t)
		repoC := gitiles_mocks.NewGitilesRepo(t)
		repoA.On("URL").Return(urlA)
		repoC.On("URL").Return(urlC)
		repoA.On("ReadFileAtRef", testutils.AnyContext, deps_parser.DepsFileName, a1).Return(nil, errors.New("404")).Once()
		repoA.On("ListDirAtRef", testutils.AnyContext, ".", a1).Return([]os.FileInfo{readmeInfo}, nil).Once()

		chk := newTestChecker(nil, []GitRepo{repoA, repoC}, nil)
		ok, err := chk.isAncestorOrTransitiveDep(ctx, lcA1, repoA, lcC1, repoC, map[string]bool{})
		require.NoError(t, err)
		require.False(t, ok)
	})
}

func TestQueryEntries(t *testing.T) {
	ctx := context.Background()
	const (
		repoURLA = "https://skia.googlesource.com/a.git"
		repoURLB = "https://skia.googlesource.com/b.git"
		c1       = "1111111111111111111111111111111111111111"
		c2       = "2222222222222222222222222222222222222222"
		cUnknown = "9999999999999999999999999999999999999999"
		cOther   = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb1"
	)
	now := time.Now().UTC()
	lc1 := makeLongCommit(c1, nil, now.Add(-time.Hour), "older")
	lc2 := makeLongCommit(c2, []string{c1}, now, "newer")
	lcOther := makeLongCommit(cOther, nil, now, "other repo")

	repoA := gitiles_mocks.NewGitilesRepo(t)
	repoA.On("URL").Return(repoURLA)
	repoA.On("Details", testutils.AnyContext, c1).Return(lc1, nil)
	repoA.On("Details", testutils.AnyContext, c2).Return(lc2, nil)
	repoA.On("Details", testutils.AnyContext, cUnknown).Return(nil, errors.New("not found"))
	repoA.On("Details", testutils.AnyContext, cOther).Return(nil, errors.New("not found"))
	repoA.On("IsAncestor", testutils.AnyContext, c2, c1).Return(false, nil)

	repoB := gitiles_mocks.NewGitilesRepo(t)
	repoB.On("URL").Return(repoURLB)
	repoB.On("Details", testutils.AnyContext, cOther).Return(lcOther, nil)
	repoB.On("Details", testutils.AnyContext, cUnknown).Return(nil, errors.New("not found"))
	repoB.On("ReadFileAtRef", testutils.AnyContext, deps_parser.DepsFileName, cOther).Return(nil, errors.New("transient error"))
	repoB.On("ListDirAtRef", testutils.AnyContext, ".", cOther).Return(nil, errors.New("transient error"))

	entries := []Entry{
		{Cluster: "skia-public", Pod: "app-new", Image: skiaPublicImagePrefix + "app@sha256:1", Commit: c2, CommitSubject: "newer"},
		{Cluster: "skia-public", Pod: "app-old", Image: skiaPublicImagePrefix + "app@sha256:2", Commit: c1, CommitSubject: "older"},
		{Cluster: "skia-public", Pod: "app-unknown-commit", Image: skiaPublicImagePrefix + "app@sha256:3", Commit: cUnknown},
		{Cluster: "skia-public", Pod: "app-deps-err", Image: skiaPublicImagePrefix + "other@sha256:4", Commit: cOther},
		{Cluster: "skia-public", Pod: "ext-no-commit", Image: "docker.io/redis@sha256:5", Commit: ""},
	}
	chk := newTestChecker(mockhttpclient.NewURLMock().Client(), []GitRepo{repoA, repoB}, entries)

	t.Run("ignore_external default and regex filtering", func(t *testing.T) {
		resp, err := chk.QueryEntries(ctx, url.Values{"pod": {"APP-NEW"}})
		require.NoError(t, err)
		require.Equal(t, len(entries), resp.Total)
		require.Equal(t, chk.lastUpdated, resp.LastUpdated)
		require.Len(t, resp.Entries, 1)
		require.Equal(t, "app-new", resp.Entries[0].Pod)
	})

	t.Run("invalid regex populates FilterErrors", func(t *testing.T) {
		resp, err := chk.QueryEntries(ctx, url.Values{"pod": {"[invalid"}})
		require.NoError(t, err)
		require.Empty(t, resp.Entries)
		require.Contains(t, resp.FilterErrors, "pod")
	})

	t.Run("ancestor populates true, false, and nil when indeterminate", func(t *testing.T) {
		resp, err := chk.QueryEntries(ctx, url.Values{
			"ancestor":        {c2},
			"ignore_external": {"false"},
		})
		require.NoError(t, err)
		require.Empty(t, resp.AncestorError)
		require.Len(t, resp.Entries, 5)

		byPod := map[string]Entry{}
		for _, e := range resp.Entries {
			byPod[e.Pod] = e
		}
		require.NotNil(t, byPod["app-new"].IsAncestor)
		require.True(t, *byPod["app-new"].IsAncestor)

		require.NotNil(t, byPod["app-old"].IsAncestor)
		require.False(t, *byPod["app-old"].IsAncestor)

		// Unknown commit, DEPS lookup failure, and empty commit all remain nil.
		require.Nil(t, byPod["app-unknown-commit"].IsAncestor)
		require.Nil(t, byPod["app-deps-err"].IsAncestor)
		require.Nil(t, byPod["ext-no-commit"].IsAncestor)
	})

	t.Run("invalid ancestor sets clean AncestorError", func(t *testing.T) {
		resp, err := chk.QueryEntries(ctx, url.Values{"ancestor": {"not-valid"}})
		require.NoError(t, err)
		require.Equal(t, `invalid commit hash, CL number, or Gerrit URL "not-valid"`, resp.AncestorError)
		for _, e := range resp.Entries {
			require.Nil(t, e.IsAncestor)
		}
	})
}

func TestNewChecker_PartialResults(t *testing.T) {
	ctx := context.Background()
	promURL := baseURL + url.QueryEscape(query)

	t.Run("returns nil checker when Prometheus fails", func(t *testing.T) {
		urlMock := mockhttpclient.NewURLMock()
		urlMock.MockOnce(promURL, mockhttpclient.MockGetError("down", http.StatusInternalServerError))
		chk, err := NewChecker(ctx, urlMock.Client(), docker_mocks.NewClient(t), nil)
		require.Error(t, err)
		require.Nil(t, chk)
	})

	t.Run("returns usable checker and error on partial GCR failure", func(t *testing.T) {
		digestOK := "sha256:" + strings.Repeat("a", 64)
		digestFail := "sha256:" + strings.Repeat("b", 64)
		hash := strings.Repeat("1", 40)
		ts := time.Date(2026, 5, 10, 14, 30, 0, 0, time.UTC)

		urlMock := mockhttpclient.NewURLMock()
		urlMock.MockOnce(promURL, mockhttpclient.MockGetDialogue(makePromPayload(t, []struct {
			Cluster   string
			Namespace string
			Pod       string
			Container string
			ImageID   string
		}{
			{Cluster: "c1", Namespace: "ns", Pod: "p-ok", Container: "c", ImageID: skiaPublicImagePrefix + "ok@" + digestOK},
			{Cluster: "c1", Namespace: "ns", Pod: "p-fail", Container: "c", ImageID: skiaPublicImagePrefix + "fail@" + digestFail},
		})))

		d := docker_mocks.NewClient(t)
		d.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/ok").Return(map[string]*docker.ImageInstance{
			digestOK: {Tags: []string{stages.GitTagPrefix + hash}},
		}, nil).Once()
		d.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/fail").Return(nil, errors.New("boom")).Once()

		repo := gitiles_mocks.NewGitilesRepo(t)
		repo.On("Details", testutils.AnyContext, hash).Return(makeLongCommit(hash, nil, ts, "subject"), nil).Once()

		chk, err := NewChecker(ctx, urlMock.Client(), d, []GitRepo{repo})
		require.Error(t, err)
		require.NotNil(t, chk)

		resp, qErr := chk.QueryEntries(ctx, url.Values{"ignore_external": {"false"}})
		require.NoError(t, qErr)
		require.False(t, resp.LastUpdated.IsZero())
		require.NotEmpty(t, resp.RefreshError)
		require.Len(t, resp.Entries, 2)
		require.Equal(t, "", resp.Entries[0].Commit)   // p-fail
		require.Equal(t, hash, resp.Entries[1].Commit) // p-ok

		// A subsequent fully successful refresh clears RefreshError.
		urlMock.MockOnce(promURL, mockhttpclient.MockGetDialogue(makePromPayload(t, []struct {
			Cluster   string
			Namespace string
			Pod       string
			Container string
			ImageID   string
		}{
			{Cluster: "c1", Namespace: "ns", Pod: "p-ok", Container: "c", ImageID: skiaPublicImagePrefix + "ok@" + digestOK},
		})))
		d.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/ok").Return(map[string]*docker.ImageInstance{
			digestOK: {Tags: []string{stages.GitTagPrefix + hash}},
		}, nil).Once()
		repo.On("Details", testutils.AnyContext, hash).Return(makeLongCommit(hash, nil, ts, "subject"), nil).Once()

		require.NoError(t, chk.updateEntries(ctx))
		resp, qErr = chk.QueryEntries(ctx, url.Values{})
		require.NoError(t, qErr)
		require.Empty(t, resp.RefreshError)
		require.Len(t, resp.Entries, 1)
	})

	t.Run("carries over cached commit info on subsequent partial GCR failure", func(t *testing.T) {
		digestOK := "sha256:" + strings.Repeat("a", 64)
		digestFlaky := "sha256:" + strings.Repeat("b", 64)
		hashOK := strings.Repeat("1", 40)
		hashFlaky := strings.Repeat("2", 40)
		hashFlakyNew := strings.Repeat("3", 40)
		ts1 := time.Date(2026, 5, 10, 14, 30, 0, 0, time.UTC)
		ts2 := time.Date(2026, 5, 10, 15, 0, 0, 0, time.UTC)

		promPayload := makePromPayload(t, []struct {
			Cluster   string
			Namespace string
			Pod       string
			Container string
			ImageID   string
		}{
			{Cluster: "c1", Namespace: "ns", Pod: "p-flaky", Container: "c", ImageID: skiaPublicImagePrefix + "flaky@" + digestFlaky},
			{Cluster: "c1", Namespace: "ns", Pod: "p-ok", Container: "c", ImageID: skiaPublicImagePrefix + "ok@" + digestOK},
		})

		urlMock := mockhttpclient.NewURLMock()
		d := docker_mocks.NewClient(t)
		repo := gitiles_mocks.NewGitilesRepo(t)

		// Tick 1: both repositories succeed.
		urlMock.MockOnce(promURL, mockhttpclient.MockGetDialogue(promPayload))
		d.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/ok").Return(map[string]*docker.ImageInstance{
			digestOK: {Tags: []string{stages.GitTagPrefix + hashOK}},
		}, nil).Once()
		d.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/flaky").Return(map[string]*docker.ImageInstance{
			digestFlaky: {Tags: []string{stages.GitTagPrefix + hashFlaky}},
		}, nil).Once()
		repo.On("Details", testutils.AnyContext, hashOK).Return(makeLongCommit(hashOK, nil, ts1, "ok subject"), nil).Once()
		repo.On("Details", testutils.AnyContext, hashFlaky).Return(makeLongCommit(hashFlaky, nil, ts1, "flaky subject"), nil).Once()

		chk, err := NewChecker(ctx, urlMock.Client(), d, []GitRepo{repo})
		require.NoError(t, err)

		// Tick 2: flaky GCR repository fails; commit info should carry over so
		// p-flaky remains visible under default ignore_external filtering.
		urlMock.MockOnce(promURL, mockhttpclient.MockGetDialogue(promPayload))
		d.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/ok").Return(map[string]*docker.ImageInstance{
			digestOK: {Tags: []string{stages.GitTagPrefix + hashOK}},
		}, nil).Once()
		d.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/flaky").Return(nil, errors.New("transient error")).Once()
		repo.On("Details", testutils.AnyContext, hashOK).Return(makeLongCommit(hashOK, nil, ts1, "ok subject"), nil).Once()

		require.Error(t, chk.updateEntries(ctx))
		resp, qErr := chk.QueryEntries(ctx, url.Values{})
		require.NoError(t, qErr)
		require.NotEmpty(t, resp.RefreshError)
		require.Len(t, resp.Entries, 2)
		require.Equal(t, "p-flaky", resp.Entries[0].Pod)
		require.Equal(t, hashFlaky, resp.Entries[0].Commit)
		require.Equal(t, ts1.Local().Format("2006-01-02 15:04"), resp.Entries[0].CommitTime)
		require.Equal(t, "flaky subject", resp.Entries[0].CommitSubject)
		require.Equal(t, "p-ok", resp.Entries[1].Pod)
		require.Equal(t, hashOK, resp.Entries[1].Commit)

		// Tick 3: flaky recovers with an updated commit tag; fresh metadata replaces carried-over values.
		urlMock.MockOnce(promURL, mockhttpclient.MockGetDialogue(promPayload))
		d.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/ok").Return(map[string]*docker.ImageInstance{
			digestOK: {Tags: []string{stages.GitTagPrefix + hashOK}},
		}, nil).Once()
		d.On("ListInstances", testutils.AnyContext, "gcr.io", "skia-public/flaky").Return(map[string]*docker.ImageInstance{
			digestFlaky: {Tags: []string{stages.GitTagPrefix + hashFlakyNew}},
		}, nil).Once()
		repo.On("Details", testutils.AnyContext, hashOK).Return(makeLongCommit(hashOK, nil, ts1, "ok subject"), nil).Once()
		repo.On("Details", testutils.AnyContext, hashFlakyNew).Return(makeLongCommit(hashFlakyNew, nil, ts2, "flaky new subject"), nil).Once()

		require.NoError(t, chk.updateEntries(ctx))
		resp, qErr = chk.QueryEntries(ctx, url.Values{})
		require.NoError(t, qErr)
		require.Empty(t, resp.RefreshError)
		require.Len(t, resp.Entries, 2)
		require.Equal(t, hashFlakyNew, resp.Entries[0].Commit)
		require.Equal(t, ts2.Local().Format("2006-01-02 15:04"), resp.Entries[0].CommitTime)
		require.Equal(t, "flaky new subject", resp.Entries[0].CommitSubject)
	})
}
