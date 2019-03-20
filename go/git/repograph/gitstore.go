package repograph

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

type gitstoreUpdater struct {
	gs gitstore.GitStore

	// These are stored between calls to Update so that we don't have to
	// Get individual commits as they are requested.
	branches   []*git.Branch
	commits    map[string]*vcsinfo.LongCommit
	lastUpdate time.Time
	mtx        sync.Mutex
}

func (g *gitstoreUpdater) Update(ctx context.Context) error {
	g.mtx.Lock()
	defer g.mtx.Unlock()

	branchPtrs, err := g.gs.GetBranches(ctx)
	if err != nil {
		return err
	}
	branches := make([]*git.Branch, 0, len(branchPtrs))
	for name, ptr := range branchPtrs {
		if name != "" {
			branches = append(branches, &git.Branch{
				Name: name,
				Head: ptr.Head,
			})
		}
	}

	from := g.lastUpdate.Add(-10 * time.Minute)
	now := time.Now()
	to := now.Add(time.Second)
	indexCommits, err := g.gs.RangeByTime(ctx, from, to, "")
	if err != nil {
		return err
	}
	hashes := make([]string, 0, len(indexCommits))
	for _, c := range indexCommits {
		hashes = append(hashes, c.Hash)
	}
	commits, err := g.gs.Get(ctx, hashes)
	if err != nil {
		return err
	}
	commitsMap := make(map[string]*vcsinfo.LongCommit, len(commits))
	for _, c := range commits {
		commitsMap[c.Hash] = c
	}

	g.lastUpdate = now
	g.branches = branches
	g.commits = commitsMap
	return nil
}

func (g *gitstoreUpdater) Get(ctx context.Context, hashes []string) ([]*vcsinfo.LongCommit, error) {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	var needRetrieve []string
	for _, hash := range hashes {
		if _, ok := g.commits[hash]; !ok {
			sklog.Warningf("Commit %q not found in results; performing explicit lookup.", hash)
			needRetrieve = append(needRetrieve, hash)
		}
	}
	if len(needRetrieve) > 0 {
		got, err := g.gs.Get(ctx, needRetrieve)
		if err != nil {
			return nil, err
		}
		for _, c := range got {
			g.commits[c.Hash] = c
		}
	}
	rv := make([]*vcsinfo.LongCommit, 0, len(hashes))
	for _, hash := range hashes {
		c, ok := g.commits[hash]
		if !ok {
			return nil, fmt.Errorf("Missing commit %s but did not retrieve it!", hash)
		}
		rv = append(rv, c)
	}
	return rv, nil
}

func (g *gitstoreUpdater) Branches(ctx context.Context) ([]*git.Branch, error) {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	if g.branches == nil {
		return nil, errors.New("Need to call Update() before Branches()")
	}
	return g.branches, nil
}
