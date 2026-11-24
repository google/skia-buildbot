// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package deployment_status

import (
	"context"
	"errors"
	"os"
	"strings"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

var errAmbiguousCommitPrefix = errors.New("ambiguous commit prefix")

// GitRepo is the subset of gitiles.GitilesRepo needed for resolving commits,
// commit logs, ancestry, and DEPS files. *gitiles.Repo implements GitRepo
// directly.
type GitRepo interface {
	Details(ctx context.Context, ref string) (*vcsinfo.LongCommit, error)
	LogFirstParent(ctx context.Context, from, to string, opts ...gitiles.LogOption) ([]*vcsinfo.LongCommit, error)
	IsAncestor(ctx context.Context, a, b string) (bool, error)
	ReadFileAtRef(ctx context.Context, srcPath, ref string) ([]byte, error)
	ListDirAtRef(ctx context.Context, dir, ref string) ([]os.FileInfo, error)
	URL() string
}

// RepographRepo implements GitRepo using an in-memory repograph.Graph for
// commit details, logs, and ancestry checks, and an embedded *gitiles.Repo for
// reading file contents (such as DEPS).
type RepographRepo struct {
	*gitiles.Repo
	graph *repograph.Graph
}

// NewRepographRepo returns a RepographRepo wrapping the given gitiles.Repo and
// repograph.Graph.
func NewRepographRepo(gr *gitiles.Repo, graph *repograph.Graph) *RepographRepo {
	return &RepographRepo{
		Repo:  gr,
		graph: graph,
	}
}

// Details implements GitRepo.
func (r *RepographRepo) Details(ctx context.Context, ref string) (*vcsinfo.LongCommit, error) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if c := r.graph.Get(ref); c != nil {
		return c.LongCommit, nil
	}
	if !git.IsCommitHash(ref) || git.IsFullCommitHash(ref) {
		return nil, skerr.Fmt("unknown commit %q", ref)
	}
	var matched *repograph.Commit
	for hash, c := range r.graph.GetAll() {
		if strings.HasPrefix(hash, ref) {
			if matched != nil && matched.Hash != c.Hash {
				return nil, skerr.Wrapf(errAmbiguousCommitPrefix, "%q", ref)
			}
			matched = c
		}
	}
	if matched == nil {
		return nil, skerr.Fmt("unknown commit %q", ref)
	}
	return matched.LongCommit, nil
}

// LogFirstParent implements GitRepo.
func (r *RepographRepo) LogFirstParent(ctx context.Context, from, to string, opts ...gitiles.LogOption) ([]*vcsinfo.LongCommit, error) {
	commits, err := r.graph.LogLinear(from, to)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv := make([]*vcsinfo.LongCommit, 0, len(commits))
	for _, c := range commits {
		rv = append(rv, c.LongCommit)
	}
	return rv, nil
}

// IsAncestor implements GitRepo.
func (r *RepographRepo) IsAncestor(ctx context.Context, a, b string) (bool, error) {
	return r.graph.IsAncestor(a, b)
}

var _ GitRepo = (*gitiles.Repo)(nil)
var _ GitRepo = (*RepographRepo)(nil)

// findCommit resolves a full or shortened commit hash across repos.
func findCommit(ctx context.Context, repos []GitRepo, ref string) (*vcsinfo.LongCommit, GitRepo, error) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if !git.IsCommitHash(ref) {
		return nil, nil, skerr.Fmt("invalid commit hash %q", ref)
	}
	var lastErr error
	for _, repo := range repos {
		c, err := repo.Details(ctx, ref)
		if err == nil {
			return c, repo, nil
		}
		if errors.Is(err, errAmbiguousCommitPrefix) {
			return nil, nil, skerr.Wrap(err)
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, nil, skerr.Wrapf(lastErr, "unknown commit %q", ref)
	}
	return nil, nil, skerr.Fmt("unknown commit %q", ref)
}

// getMostRecentCommit returns the most recent Commit from the given slice of
// commit hashes by logging from an arbitrary commit in the slice to main and
// checking whether any other commits in the slice appear in that range.
func getMostRecentCommit(ctx context.Context, repos []GitRepo, commitHashes []string) *vcsinfo.LongCommit {
	if len(commitHashes) == 0 {
		return nil
	}
	var startCommit *vcsinfo.LongCommit
	var matchedRepo GitRepo
	for _, hash := range commitHashes {
		c, repo, err := findCommit(ctx, repos, hash)
		if err == nil {
			startCommit = c
			matchedRepo = repo
			break
		}
		sklog.Warningf("Failed to find commit %s in any repo: %s", hash, err)
	}
	if startCommit == nil {
		return nil
	}
	if len(commitHashes) == 1 {
		return startCommit
	}
	hashesSet := make(map[string]bool, len(commitHashes))
	for _, h := range commitHashes {
		hashesSet[h] = true
	}
	commits, err := matchedRepo.LogFirstParent(ctx, startCommit.Hash, git.MainBranch)
	if err != nil {
		sklog.Warningf("Failed to log %s..%s in %s: %s", startCommit.Hash, git.MainBranch, matchedRepo.URL(), err)
		return startCommit
	}
	for _, c := range commits {
		if hashesSet[c.Hash] {
			return c
		}
	}
	return startCommit
}
