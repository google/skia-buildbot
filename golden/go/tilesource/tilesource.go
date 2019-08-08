package tilesource

import (
	"context"
	"net/url"
	"sync"
	"time"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/tracestore"
	"go.skia.org/infra/golden/go/tryjobs"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
)

type TileSource interface {
	// GetTile returns the most recently loaded Tile.
	GetTile() (types.ComplexTile, error)
}

type CachedTileSourceConfig struct {
	EventBus      eventbus.EventBus
	GerritAPI     gerrit.GerritInterface
	IgnoreStore   ignore.IgnoreStore
	TraceStore    tracestore.TraceStore
	TryjobMonitor tryjobs.TryjobMonitor
	VCS           vcsinfo.VCS

	// optional. If specified, will only show the params that match this query. This is
	// opt-in, to avoid leaking.
	PubliclyViewableParams paramtools.ParamSet

	// NCommits is the number of commits we should consider. If NCommits is
	// 0 or smaller all commits in the last tile will be considered.
	NCommits int
}

type CachedTileSourceImpl struct {
	CachedTileSourceConfig

	lastCpxTile   types.ComplexTile
	lastTimeStamp time.Time
	mutex         sync.RWMutex
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
	go util.RepeatCtx(interval, ctx, func() {
		if err := s.updateTile(ctx); err != nil {
			sklog.Errorf("Could not update tile: %s", err)
		}
	})
	return nil
}

// TODO(stephana): Expand the Tile type to make querying faster.
// i.e. add traces as an array so that iteration can be done in parallel and
// add map[hash]Commit to do faster commit lookup (-> Remove tiling.FindCommit).

// GetTile implements the TileSource interface.
func (s *CachedTileSourceImpl) GetTile() (types.ComplexTile, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.lastCpxTile, nil
}

// updateTile fetches the latest tile and caches it.
func (s *CachedTileSourceImpl) updateTile(ctx context.Context) error {
	defer metrics2.FuncTimer().Stop()

	if err := s.VCS.Update(ctx, true, false); err != nil {
		return skerr.Wrapf(err, "could not update VCS")
	}

	denseTile, allCommits, err := s.TraceStore.GetDenseTile(ctx, s.NCommits)
	if err != nil {
		return skerr.Wrapf(err, "could not fetch dense tile")
	}

	// Filter down to the publicly viewable ones
	denseTile = s.filterTile(denseTile)

	cpxTile := types.NewComplexTile(denseTile)
	cpxTile.SetSparse(allCommits)

	// Get the tile without the ignored traces and update the complex tile.
	ignores, err := s.IgnoreStore.List()
	if err != nil {
		return skerr.Wrapf(err, "could not fetch ignore rules")
	}
	retIgnoredTile, ignoreRules, err := ignore.FilterIgnored(denseTile, ignores)
	if err != nil {
		return skerr.Wrapf(err, "could not apply ignore rules to tile")
	}
	cpxTile.SetIgnoreRules(retIgnoredTile, ignoreRules)

	// check if all the expectations of all commits have been added to the tile.
	s.checkCommitableIssues(cpxTile)

	s.mutex.Lock()
	defer s.mutex.Unlock()
	// Update the cached tile and return the result.
	s.lastCpxTile = cpxTile
	s.lastTimeStamp = time.Now()
	return nil
}

// filterTile creates a new tile from the given tile that contains
// only traces that match the publicly viewable params.
func (s *CachedTileSourceImpl) filterTile(tile *tiling.Tile) *tiling.Tile {
	if len(s.PubliclyViewableParams) == 0 {
		return tile
	}

	// filter tile.
	ret := &tiling.Tile{
		Traces:  make(map[tiling.TraceId]tiling.Trace, len(tile.Traces)),
		Commits: tile.Commits,
	}

	// Iterate over the tile and copy the publicly viewable traces over.
	// Build the paramset in the process.
	paramSet := paramtools.ParamSet{}
	for traceID, trace := range tile.Traces {
		if tiling.Matches(trace, url.Values(s.PubliclyViewableParams)) {
			ret.Traces[traceID] = trace
			paramSet.AddParams(trace.Params())
		}
	}
	ret.ParamSet = paramSet
	sklog.Infof("After filtering %d original traces, %d are publicly viewable.", len(tile.Traces), len(ret.Traces))
	return ret
}

// checkCommitableIssues checks all commits of the current tile whether
// the associated expectations have been added to the baseline of the master.
// TODO(kjlubick): This should not be here, but likely in tryjobMonitor, named
// something like "CatchUpIssues" or something.
func (s *CachedTileSourceImpl) checkCommitableIssues(cpxTile types.ComplexTile) {
	go func() {
		var egroup errgroup.Group

		for _, commit := range cpxTile.AllCommits() {
			func(commit *tiling.Commit) {
				egroup.Go(func() error {
					// TODO(kjlubick): We probably don't need to run this individually, we could
					// use DetailsMulti instead.
					longCommit, err := s.VCS.Details(context.Background(), commit.Hash, false)
					if err != nil {
						return skerr.Wrapf(err, "retrieving details for commit %s", commit.Hash)
					}

					issueID, err := s.GerritAPI.ExtractIssueFromCommit(longCommit.Body)
					if err != nil {
						return skerr.Wrapf(err, "extracting gerrit issue from commit %s: %s", commit.Hash, longCommit.Body)
					}

					if err := s.TryjobMonitor.CommitIssueBaseline(issueID, longCommit.Author); err != nil {
						return skerr.Wrapf(err, "committing tryjob results for commit %s", commit.Hash)
					}
					return nil
				})
			}(commit)
		}

		if err := egroup.Wait(); err != nil {
			sklog.Errorf("Error trying issue commits: %s", err)
		}
	}()
}
