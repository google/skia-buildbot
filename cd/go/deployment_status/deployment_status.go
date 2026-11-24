// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package deployment_status

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

// filterColumns are the column names supported for regex filtering.
var filterColumns = []string{
	"cluster",
	"namespace",
	"pod",
	"container",
	"commit",
	"commit_time",
	"commit_subject",
	"image",
}

// Checker manages querying, filtering, and checking commit ancestry across
// container deployment entries.
type Checker struct {
	c             *http.Client
	d             docker.Client
	repos         []GitRepo
	mtx           sync.RWMutex
	cachedEntries []Entry
	lastUpdated   time.Time
	refreshError  string
	depsMtx       sync.Mutex
	depsCache     map[string]deps_parser.DepsEntries
	clMtx         sync.Mutex
	clCache       map[string]string
}

// NewChecker returns a new Checker instance, pre-filled with Entries. If commit
// metadata fails to resolve for some images or repositories, both a usable
// Checker (populated with partial results) and a non-nil error are returned.
func NewChecker(ctx context.Context, c *http.Client, d docker.Client, repos []GitRepo) (*Checker, error) {
	chk := &Checker{
		c:         c,
		d:         d,
		repos:     repos,
		depsCache: map[string]deps_parser.DepsEntries{},
		clCache:   map[string]string{},
	}
	if err := chk.updateEntries(ctx); err != nil {
		chk.mtx.RLock()
		hasEntries := chk.cachedEntries != nil
		chk.mtx.RUnlock()
		if hasEntries {
			return chk, skerr.Wrap(err)
		}
		return nil, skerr.Wrap(err)
	}
	return chk, nil
}

// resolveCLCommit retrieves the commit hash for the given merged Gerrit CL.
// Returned errors are formatted as user-facing messages without stack traces.
func (s *Checker) resolveCLCommit(ctx context.Context, gerritURL string, issueID int64) (string, error) {
	cacheKey := fmt.Sprintf("%s#%d", gerritURL, issueID)
	s.clMtx.Lock()
	if hash, ok := s.clCache[cacheKey]; ok {
		s.clMtx.Unlock()
		return hash, nil
	}
	s.clMtx.Unlock()

	g, err := gerrit.NewGerrit(gerritURL, s.c)
	if err != nil {
		sklog.Warningf("Failed to initialize Gerrit client for %s: %s", gerritURL, err)
		return "", fmt.Errorf("failed to look up CL %d", issueID)
	}
	ci, err := g.GetIssueProperties(ctx, issueID)
	if err != nil {
		sklog.Warningf("Failed to look up CL %d on %s: %s", issueID, gerritURL, err)
		return "", fmt.Errorf("failed to look up CL %d", issueID)
	}
	if !ci.IsMerged() {
		return "", fmt.Errorf("CL %d is not merged (status: %s)", issueID, ci.Status)
	}
	if len(ci.Patchsets) == 0 {
		return "", fmt.Errorf("CL %d has no patchsets", issueID)
	}
	commitHash := ci.Patchsets[len(ci.Patchsets)-1].ID
	s.clMtx.Lock()
	s.clCache[cacheKey] = commitHash
	s.clMtx.Unlock()
	return commitHash, nil
}

// resolveCommitOrCL resolves a full or shortened commit hash, a numeric Gerrit
// CL ID, or a Gerrit change URL to a LongCommit and its corresponding GitRepo.
// Returned errors are formatted as user-facing messages without stack traces.
func (s *Checker) resolveCommitOrCL(ctx context.Context, ref string) (*vcsinfo.LongCommit, GitRepo, error) {
	ref = strings.TrimSpace(ref)
	var lastErr error
	if issueID, err := strconv.ParseInt(ref, 10, 64); err == nil {
		seenGerritURLs := map[string]bool{}
		for _, repo := range s.repos {
			gerritURL, err := gerrit.GerritURLFromRepoURL(repo.URL())
			if err != nil || seenGerritURLs[gerritURL] {
				continue
			}
			seenGerritURLs[gerritURL] = true
			commitHash, err := s.resolveCLCommit(ctx, gerritURL, issueID)
			if err != nil {
				lastErr = err
				continue
			}
			c, matchedRepo, err := findCommit(ctx, s.repos, commitHash)
			if err == nil {
				return c, matchedRepo, nil
			}
			sklog.Warningf("Failed to find commit %s for CL %d: %s", commitHash, issueID, err)
			lastErr = fmt.Errorf("commit %s for CL %d not found in tracked repositories", commitHash, issueID)
		}
	}
	if git.IsCommitHash(ref) {
		c, repo, err := findCommit(ctx, s.repos, ref)
		if err == nil {
			return c, repo, nil
		}
		if lastErr == nil {
			if errors.Is(err, errAmbiguousCommitPrefix) {
				lastErr = fmt.Errorf("ambiguous commit prefix %q", ref)
			} else {
				lastErr = fmt.Errorf("unknown commit %q", ref)
			}
		}
	}
	if lastErr != nil {
		return nil, nil, lastErr
	}
	gerritURL, issueID, err := gerrit.ParseChangeURL(ref)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid commit hash, CL number, or Gerrit URL %q", ref)
	}
	commitHash, err := s.resolveCLCommit(ctx, gerritURL, issueID)
	if err != nil {
		return nil, nil, err
	}
	c, matchedRepo, err := findCommit(ctx, s.repos, commitHash)
	if err != nil {
		sklog.Warningf("Failed to find commit %s for CL %d: %s", commitHash, issueID, err)
		return nil, nil, fmt.Errorf("commit %s for CL %d not found in tracked repositories", commitHash, issueID)
	}
	return c, matchedRepo, nil
}

// updateEntries updates any RepographRepo instances, fetches all entries from
// Prometheus and GCR, and updates the in-memory cache. If updating a repo or
// resolving commit metadata for some images/repositories fails, partial results
// are still cached and the combined error is returned.
func (s *Checker) updateEntries(ctx context.Context) error {
	var errs []error
	for _, repo := range s.repos {
		if rr, ok := repo.(*RepographRepo); ok {
			if err := rr.graph.Update(ctx); err != nil {
				errs = append(errs, skerr.Wrapf(err, "failed to update repograph for %s", repo.URL()))
			}
		}
	}
	entries, fetchErr := fetchEntries(ctx, s.c, s.d, s.repos)
	if fetchErr != nil {
		errs = append(errs, skerr.Wrap(fetchErr))
	}
	combinedErr := errors.Join(errs...)
	s.mtx.Lock()
	if entries != nil {
		if fetchErr != nil && len(s.cachedEntries) > 0 {
			prevByImage := make(map[string]Entry, len(s.cachedEntries))
			for _, prev := range s.cachedEntries {
				if prev.Commit != "" {
					prevByImage[prev.Image] = prev
				}
			}
			for i := range entries {
				if entries[i].Commit == "" {
					if prev, ok := prevByImage[entries[i].Image]; ok {
						entries[i].Commit = prev.Commit
						entries[i].CommitTime = prev.CommitTime
						entries[i].CommitSubject = prev.CommitSubject
					}
				}
			}
		}
		s.cachedEntries = entries
		s.lastUpdated = time.Now()
	}
	if combinedErr != nil {
		s.refreshError = "Last refresh completed with errors; some container data may be missing or stale."
	} else {
		s.refreshError = ""
	}
	s.mtx.Unlock()
	if combinedErr != nil {
		return skerr.Wrap(combinedErr)
	}
	return nil
}

// StartAutoRefresh starts a background goroutine that refreshes entries at the
// given interval, waiting one interval before the first refresh since
// NewChecker already populates the cache.
func (s *Checker) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.updateEntries(ctx); err != nil {
					sklog.Errorf("Failed to update entries: %s", err)
				}
			}
		}
	}()
}

func (s *Checker) getDeps(repo GitRepo, commitHash string) (deps_parser.DepsEntries, error) {
	s.depsMtx.Lock()
	if deps, ok := s.depsCache[commitHash]; ok {
		s.depsMtx.Unlock()
		return deps, nil
	}
	s.depsMtx.Unlock()

	// Use background context so in-flight DEPS fetches still populate the cache
	// even if the client aborts an intermediate keystroke request.
	ctx := context.Background()
	b, readErr := repo.ReadFileAtRef(ctx, deps_parser.DepsFileName, commitHash)
	if readErr != nil {
		// Distinguish a commit without a DEPS file from a transient Gitiles
		// error (e.g. replication-delay 404) by listing the root directory.
		infos, listErr := repo.ListDirAtRef(ctx, ".", commitHash)
		if listErr != nil {
			return nil, skerr.Wrap(readErr)
		}
		for _, fi := range infos {
			if fi.Name() == deps_parser.DepsFileName && !fi.IsDir() {
				return nil, skerr.Wrap(readErr)
			}
		}
		empty := deps_parser.DepsEntries{}
		s.depsMtx.Lock()
		s.depsCache[commitHash] = empty
		s.depsMtx.Unlock()
		return empty, nil
	}
	deps, err := deps_parser.ParseDeps(string(b))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	s.depsMtx.Lock()
	s.depsCache[commitHash] = deps
	s.depsMtx.Unlock()
	return deps, nil
}

func isSameOrAncestor(ctx context.Context, repo GitRepo, ancestor, descendant string) (bool, error) {
	if ancestor == descendant {
		return true, nil
	}
	return repo.IsAncestor(ctx, ancestor, descendant)
}

// isAncestorOrTransitiveDep returns true if targetCommit (in targetRepo) is an
// ancestor of commit (in repo), either directly in the same repository or
// transitively via DEPS pins. It returns an error if ancestry could not be
// determined due to a lookup failure.
func (s *Checker) isAncestorOrTransitiveDep(ctx context.Context, commit *vcsinfo.LongCommit, repo GitRepo, targetCommit *vcsinfo.LongCommit, targetRepo GitRepo, visited map[string]bool) (bool, error) {
	if commit == nil || repo == nil || targetCommit == nil || targetRepo == nil {
		return false, skerr.Fmt("missing commit or repo for ancestry check")
	}
	if repo.URL() == targetRepo.URL() {
		return isSameOrAncestor(ctx, repo, targetCommit.Hash, commit.Hash)
	}
	if visited[commit.Hash] {
		return false, nil
	}
	visited[commit.Hash] = true

	deps, err := s.getDeps(repo, commit.Hash)
	if err != nil {
		return false, skerr.Wrapf(err, "failed to read DEPS for %s at %s", repo.URL(), commit.Hash)
	}
	var lookupErr error
	if dep := deps.Get(targetRepo.URL()); dep != nil && dep.Version != "" {
		depCommit, err := targetRepo.Details(ctx, dep.Version)
		if err != nil {
			lookupErr = skerr.Wrapf(err, "failed to resolve %s@%s", targetRepo.URL(), dep.Version)
		} else {
			ok, err := isSameOrAncestor(ctx, targetRepo, targetCommit.Hash, depCommit.Hash)
			if err != nil {
				lookupErr = skerr.Wrapf(err, "failed to check ancestry in %s", targetRepo.URL())
			} else if ok {
				return true, nil
			}
		}
	}
	for _, otherRepo := range s.repos {
		if otherRepo.URL() == repo.URL() || otherRepo.URL() == targetRepo.URL() {
			continue
		}
		if dep := deps.Get(otherRepo.URL()); dep != nil && dep.Version != "" {
			depCommit, err := otherRepo.Details(ctx, dep.Version)
			if err != nil {
				lookupErr = skerr.Wrapf(err, "failed to resolve %s@%s", otherRepo.URL(), dep.Version)
				continue
			}
			ok, err := s.isAncestorOrTransitiveDep(ctx, depCommit, otherRepo, targetCommit, targetRepo, visited)
			if err != nil {
				lookupErr = err
				continue
			}
			if ok {
				return true, nil
			}
		}
	}
	return false, lookupErr
}

// QueryEntries filters the cached entries and evaluates commit ancestry
// according to the query parameters in q.
func (s *Checker) QueryEntries(ctx context.Context, q url.Values) (*EntriesResponse, error) {
	regexes := map[string]*regexp.Regexp{}
	filterErrors := map[string]string{}
	for _, col := range filterColumns {
		pattern := q.Get(col)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			filterErrors[col] = err.Error()
			continue
		}
		regexes[col] = re
	}

	var targetCommit *vcsinfo.LongCommit
	var targetRepo GitRepo
	var ancestorError string
	if ancestor := strings.TrimSpace(q.Get("ancestor")); ancestor != "" {
		c, repo, err := s.resolveCommitOrCL(ctx, ancestor)
		if err != nil {
			ancestorError = err.Error()
		} else {
			targetCommit = c
			targetRepo = repo
		}
	}

	ignoreExternal := q.Get("ignore_external") != "false"

	s.mtx.RLock()
	allEntries := s.cachedEntries
	lastUpdated := s.lastUpdated
	refreshError := s.refreshError
	s.mtx.RUnlock()

	total := len(allEntries)
	filtered := make([]Entry, 0, total)
	if len(filterErrors) == 0 {
		ancestorByCommit := map[string]*bool{}
		for _, entry := range allEntries {
			if ignoreExternal && (!strings.HasPrefix(entry.Image, skiaPublicImagePrefix) || entry.Commit == "") {
				continue
			}
			matched := true
			for col, re := range regexes {
				if !re.MatchString(entry.columnValue(col)) {
					matched = false
					break
				}
			}
			if !matched {
				continue
			}
			if targetCommit != nil && entry.Commit != "" {
				if cached, ok := ancestorByCommit[entry.Commit]; ok {
					entry.IsAncestor = cached
				} else {
					var isAncPtr *bool
					if c, repo, err := findCommit(ctx, s.repos, entry.Commit); err != nil {
						sklog.Warningf("Failed to find commit %s: %s", entry.Commit, err)
					} else if isAnc, err := s.isAncestorOrTransitiveDep(ctx, c, repo, targetCommit, targetRepo, map[string]bool{}); err != nil {
						sklog.Warningf("Failed to determine ancestry for %s: %s", entry.Commit, err)
					} else {
						isAncPtr = &isAnc
					}
					ancestorByCommit[entry.Commit] = isAncPtr
					entry.IsAncestor = isAncPtr
				}
			}
			filtered = append(filtered, entry)
		}
	}

	rv := &EntriesResponse{
		Entries:       filtered,
		Total:         total,
		LastUpdated:   lastUpdated,
		RefreshError:  refreshError,
		AncestorError: ancestorError,
	}
	if len(filterErrors) > 0 {
		rv.FilterErrors = filterErrors
	}
	return rv, nil
}
