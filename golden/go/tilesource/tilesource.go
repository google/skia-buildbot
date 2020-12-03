package tilesource

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/publicparams"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/tracestore"
)

const (
	emptyCommitsAtHeadMetric = "gold_empty_commits_at_head"
	filledTracesAtHeadMetric = "gold_filled_traces_at_head"
)

type TileSource interface {
	// GetTile returns the most recently loaded Tile.
	GetTile() tiling.ComplexTile
}

type CachedTileSourceConfig struct {
	CLUpdater   code_review.ChangelistLandedUpdater
	IgnoreStore ignore.Store
	TraceStore  tracestore.TraceStore
	VCS         vcsinfo.VCS

	// optional. If specified, will only show the params that match this query. This is
	// opt-in, to avoid leaking.
	PubliclyViewableParams publicparams.Matcher

	// NCommits is the number of commits we should consider. If NCommits is
	// 0 or smaller all commits in the last tile will be considered.
	NCommits int
}

type CachedTileSourceImpl struct {
	CachedTileSourceConfig

	lastCpxTile tiling.ComplexTile
	mutex       sync.RWMutex
}

func New(c CachedTileSourceConfig) *CachedTileSourceImpl {
	cti := &CachedTileSourceImpl{
		CachedTileSourceConfig: c,
	}
	return cti
}

// StartUpdater loads the initial tile and starts a goroutine to update it at
// the specified interval. It returns an error if the initial load fails, but
// will only log errors that happen later.
func (s *CachedTileSourceImpl) StartUpdater(ctx context.Context, interval time.Duration) error {
	if err := s.updateTile(ctx); err != nil {
		return skerr.Wrapf(err, "failed initial tile update")
	}
	go util.RepeatCtx(ctx, interval, func(ctx context.Context) {
		if err := s.updateTile(ctx); err != nil {
			sklog.Errorf("Could not update tile: %s", err)
		}
	})
	return nil
}

// GetTile implements the TileSource interface.
func (s *CachedTileSourceImpl) GetTile() tiling.ComplexTile {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.lastCpxTile
}

// updateTile fetches the latest tile and caches it. updateTile expects to be called from a
// single goroutine (see StartUpdater).
func (s *CachedTileSourceImpl) updateTile(ctx context.Context) error {
	defer metrics2.FuncTimer().Stop()

	if err := s.VCS.Update(ctx, true, false); err != nil {
		return skerr.Wrapf(err, "updating VCS")
	}
	var prevCommit tiling.Commit
	if s.lastCpxTile != nil {
		commits := s.lastCpxTile.AllCommits()
		if len(commits) > 0 {
			prevCommit = commits[len(commits)-1]
		}
	}

	denseTile, allCommits, err := s.TraceStore.GetDenseTile(ctx, s.NCommits)
	if err != nil {
		return skerr.Wrapf(err, "fetching dense tile")
	}

	// Filter down to the publicly viewable ones
	denseTile = s.filterTile(denseTile)
	// Now that we have filtered the public list, compute metrics. We don't care about ignores,
	// since that's something the user is in charge of, whereas the public list is something
	// that's part of the Gold config.
	computeMetricsOnTile(denseTile, allCommits)

	cpxTile := tiling.NewComplexTile(denseTile)
	cpxTile.SetSparse(allCommits)

	// Get the tile without the ignored traces and update the complex tile.
	ignores, err := s.IgnoreStore.List(ctx)
	if err != nil {
		return skerr.Wrapf(err, "fetching ignore rules")
	}
	retIgnoredTile, ignoreRules, err := ignore.FilterIgnored(denseTile, ignores)
	if err != nil {
		return skerr.Wrapf(err, "applying ignore rules to tile")
	}
	cpxTile.SetIgnoreRules(retIgnoredTile, ignoreRules)

	// check if all the expectations of all commits have been added to the tile.
	err = s.checkForLandedChangelists(ctx, prevCommit, allCommits)
	if err != nil {
		return skerr.Wrapf(err, "identifying CLs/CLExpectations that have landed")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	// Update the cached tile and return the result.
	s.lastCpxTile = cpxTile
	return nil
}

// filterTile creates a new tile from the given tile that contains
// only traces that match the publicly viewable params.
func (s *CachedTileSourceImpl) filterTile(tile *tiling.Tile) *tiling.Tile {
	if s.PubliclyViewableParams == nil {
		return tile
	}

	// filter tile.
	ret := &tiling.Tile{
		Traces:  make(map[tiling.TraceID]*tiling.Trace, len(tile.Traces)),
		Commits: tile.Commits,
	}

	// Iterate over the tile and copy the publicly viewable traces over.
	// Build the paramset in the process.
	paramSet := paramtools.ParamSet{}
	for traceID, trace := range tile.Traces {
		ko := trace.KeysAndOptions()
		if s.PubliclyViewableParams.Matches(ko) {
			ret.Traces[traceID] = trace
			paramSet.AddParams(ko)
		}
	}
	paramSet.Normalize()
	ret.ParamSet = paramSet
	sklog.Infof("After filtering %d original traces, %d are publicly viewable.", len(tile.Traces), len(ret.Traces))
	return ret
}

// computeMetricsOnTile calculates a few metrics related to the contents of the tile.
func computeMetricsOnTile(denseTile *tiling.Tile, allCommits []tiling.Commit) {
	tracesWithData := int64(0)
	for _, trace := range denseTile.Traces {
		if !trace.IsMissing(trace.Len() - 1) {
			tracesWithData++
		}
	}
	metrics2.GetInt64Metric(filledTracesAtHeadMetric).Update(tracesWithData)

	emptyCommitsAtHead := 0
	if len(denseTile.Commits) == 0 {
		metrics2.GetInt64Metric(emptyCommitsAtHeadMetric).Update(int64(len(allCommits)))
		return
	}
	lastCommitWithData := denseTile.Commits[len(denseTile.Commits)-1].Hash
	// Start at the end of all of the commits and walk backwards until we hit the last commit
	// with data.
	for ; emptyCommitsAtHead < len(allCommits); emptyCommitsAtHead++ {
		if allCommits[len(allCommits)-1-emptyCommitsAtHead].Hash == lastCommitWithData {
			break
		}
	}
	metrics2.GetInt64Metric(emptyCommitsAtHeadMetric).Update(int64(emptyCommitsAtHead))
}

// checkForLandedChangelists checks all commits of the current tile whether
// the associated expectations have been added to the baseline of the master.
func (s *CachedTileSourceImpl) checkForLandedChangelists(ctx context.Context, prev tiling.Commit, commits []tiling.Commit) error {
	if s.CLUpdater == nil {
		sklog.Infof("Not Updating clstore with landed CLs because no updater configured.")
		return nil
	}
	if len(commits) == 0 {
		sklog.Warningf("No commits in tile?")
		return nil
	}
	if !prev.CommitTime.IsZero() {
		// re-slice commits after prev so as to avoid doing redundant work.
		lastIdx := 0
		for i, c := range commits {
			if prev.Hash == c.Hash {
				lastIdx = i + 1
				break
			}
		}
		commits = commits[lastIdx:]
		if len(commits) == 0 {
			sklog.Infof("No new commits since last cycle")
			return nil
		}
	}

	hashes := make([]string, 0, len(commits))
	for _, c := range commits {
		hashes = append(hashes, c.Hash)
	}

	xc, err := s.VCS.DetailsMulti(ctx, hashes, false)
	if err != nil {
		return skerr.Wrapf(err, "fetching details of %d hashes starting at %s", len(hashes), hashes[0])
	}

	return skerr.Wrap(s.CLUpdater.UpdateChangelistsAsLanded(ctx, xc))

}
