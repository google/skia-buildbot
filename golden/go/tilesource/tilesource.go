package tilesource

import (
	"context"
	"net/url"
	"sync"
	"time"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/tryjobs"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
)

type DeprecatedTileSource interface {
	// GetTile returns the most recently loaded Tile.
	GetTile() *tiling.Tile
}

type TileSource interface {
	// GetTile returns the most recently loaded Tile.
	GetTile() (types.ComplexTile, error)
}

const (
	// maxNSparseCommits is the maximum number of commits we are considering when condensing a
	// sparse tile into a dense tile by removing commits that contain no data.
	// This should be changed or made a config option when we consider going back more commits makes
	// sense.
	maxNSparseCommits = 3000

	// How long to cache the tile
	tileCacheTime = 3 * time.Minute
)

type CachedTileSourceConfig struct {
	EventBus          eventbus.EventBus
	GerritAPI         gerrit.GerritInterface
	IgnoreStore       ignore.IgnoreStore
	MasterTileBuilder DeprecatedTileSource
	TraceDB           tracedb.DB
	TryjobMonitor     tryjobs.TryjobMonitor
	VCS               vcsinfo.VCS

	// optional. If specified, will only show the params that match this query. This is
	// opt-in, to avoid leaking.
	PubliclyViewableParams paramtools.ParamSet

	// IsSparseTile indicates that new tiles should be condensed by removing commits that have no data.
	IsSparseTile bool

	// NCommits is the number of commits we should consider. If NCommits is
	// 0 or smaller all commits in the last tile will be considered.
	NCommits int
}

type CachedTileSourceImpl struct {
	CachedTileSourceConfig

	lastCpxTile   types.ComplexTile
	lastTimeStamp time.Time
	mutex         sync.Mutex
}

func New(c CachedTileSourceConfig) *CachedTileSourceImpl {
	cti := &CachedTileSourceImpl{
		CachedTileSourceConfig: c,
	}
	return cti
}

// TODO(stephana): Expand the Tile type to make querying faster.
// i.e. add traces as an array so that iteration can be done in parallel and
// add map[hash]Commit to do faster commit lookup (-> Remove tiling.FindCommit).

// GetLastTrimmed returns the last tile as read-only trimmed to contain at
// most NCommits. It caches trimmed tiles as long as the underlying tiles
// do not change.
func (s *CachedTileSourceImpl) GetTile() (types.ComplexTile, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// If the tile was updated within a certain time window just return it without
	// calculating it again.
	if s.lastCpxTile != nil && (time.Since(s.lastTimeStamp) < tileCacheTime) {
		sklog.Infof("short circuiting get tile, because it's still new: %s < %s", time.Since(s.lastTimeStamp), tileCacheTime)
		return s.lastCpxTile, nil
	}

	// Retrieve the most recent tile from the tilestore
	var rawTile *tiling.Tile
	var sparseCommits []*tiling.Commit = nil
	var cardinalities []int = nil
	var err error
	ctx := context.Background()

	// If it's a sparse tile, we build it anew.
	if s.IsSparseTile {
		rawTile, sparseCommits, cardinalities, err = s.getCondensedTile(ctx, s.lastCpxTile)
		if err != nil {
			return nil, skerr.Fmt("Error getting condensed tile: %s", err)
		}
	} else {
		rawTile = s.MasterTileBuilder.GetTile()
	}

	// Get the tile with everything that needs to be whitelisted.
	rawTile = s.filterTile(rawTile)
	if s.NCommits <= 0 {
		cpxTile := types.NewComplexTile(rawTile)
		cpxTile.SetSparse(nil, nil)
		return cpxTile, nil
	}

	// Get the ignore revision and check if the tile has changed at all.
	// Note: This only applies to tiles that are not sparse.
	currentIgnoreRev := s.IgnoreStore.Revision()
	if s.lastCpxTile != nil && s.lastCpxTile.FromSame(rawTile, currentIgnoreRev) {
		return s.lastCpxTile, nil
	}

	// Construct the new complex tile
	cpxTile := types.NewComplexTile(rawTile)
	cpxTile.SetSparse(sparseCommits, cardinalities)

	// Get the tile without the ignored traces and update the complex tile.
	ignores, err := s.IgnoreStore.List()
	if err != nil {
		return nil, skerr.Fmt("could not fetch ignore rules: %s", err)
	}
	retIgnoredTile, ignoreRules, err := ignore.FilterIgnored(rawTile, ignores)
	if err != nil {
		return nil, skerr.Fmt("could not apply ignore rules to tile: %s", err)
	}
	cpxTile.SetIgnoreRules(retIgnoredTile, ignoreRules, currentIgnoreRev)

	// check if all the expectations of all commits have been added to the tile.
	s.checkCommitableIssues(cpxTile)

	// Update the cached tile and return the result.
	s.lastCpxTile = cpxTile
	s.lastTimeStamp = time.Now()
	return cpxTile, nil
}

// getCondensedTile returns a tile that contains only commits that have at least one
// nonempty entry. If lastTile is not nil, its first commit is used as a starting point to
// fetch the tiles necessary to build the condensed tile (from several "sparse" tiles.)
// TODO(kjlubick): delete this when we have the BT-based tracestore, which will compute this.
func (s *CachedTileSourceImpl) getCondensedTile(ctx context.Context, lastCpxTile types.ComplexTile) (*tiling.Tile, []*tiling.Commit, []int, error) {
	if s.NCommits <= 0 {
		ret := tiling.NewTile()
		ret.Commits = ret.Commits[:0]
		return ret, nil, nil, nil
	}

	var err error

	// Determine the starting value of commits to fetch.
	lastNCommits := 10 * s.NCommits
	if lastCpxTile != nil {
		lastNCommits = len(lastCpxTile.AllCommits())
	}

	// Find all commit IDs we are interested in.
	var sparseCommitIDs []*tracedb.CommitID
	var sparseCommits []*tiling.Commit
	var cardinalities []int
	var targetHashes util.StringSet

	// Repeat until we get the desired number of commits.
	var idxCommits, prevIdxCommits []*vcsinfo.IndexCommit
	for len(targetHashes) < s.NCommits {
		idxCommits = s.VCS.LastNIndex(lastNCommits)
		if len(idxCommits) <= len(prevIdxCommits) {
			break
		}
		prevIdxCommits = idxCommits

		// Build a candidate Tile from the found commits
		sparseCommitIDs = getCommitIDs(idxCommits)
		sparseTile, _, err := s.TraceDB.TileFromCommits(sparseCommitIDs)
		if err != nil {
			return nil, nil, nil, skerr.Fmt("Failed to load tile from commitIDs: %s", err)
		}

		// Find which commits are non-empty
		targetHashes = make(util.StringSet, len(sparseCommitIDs))
		tileLen := sparseTile.LastCommitIndex() + 1
		sklog.Infof("Sparse tile len: %d", tileLen)
		sparseCommits = sparseTile.Commits[:tileLen]
		sklog.Infof("Sparse tile commits len: %d", len(sparseCommits))
		cardinalities = make([]int, tileLen)

		for idx := 0; idx < tileLen; idx++ {
			hash := sparseCommits[idx].Hash
			for _, trace := range sparseTile.Traces {
				gTrace := trace.(*types.GoldenTrace)
				if gTrace.Digests[idx] != types.MISSING_DIGEST {
					targetHashes[hash] = true
					cardinalities[idx]++
				}
			}
		}

		// double the number of commits we consider for the target tile to reach our goal fast.
		// If we are above a maximum number of commits don't go any further and just use what we have.
		lastNCommits *= 2
		if lastNCommits > maxNSparseCommits && len(targetHashes) > 0 {
			sklog.Infof("Reached limit of %d commits to consider in sparse tile. Using %d commits.", maxNSparseCommits, len(targetHashes))
			break
		}
	}
	sklog.Infof("Found %d target commits within %d sparse commits", len(targetHashes), len(sparseCommits))

	detailsHashes := make([]string, 0, len(sparseCommits))
	denseCommitIDs := make([]*tracedb.CommitID, 0, len(targetHashes))
	remainingCommits := len(targetHashes)
	sparseStart := -1
	sklog.Infof("Starting to add commit details")
	for idx, commitID := range sparseCommitIDs {
		if targetHashes[commitID.ID] {
			if remainingCommits <= s.NCommits {
				if sparseStart == -1 {
					sparseStart = idx
					sklog.Infof("Sparse start: %d", sparseStart)
				}
				denseCommitIDs = append(denseCommitIDs, commitID)
			}
			remainingCommits--
		}

		// If we have found the first commit we consider, then we add the details data.
		if sparseStart >= 0 {
			detailsHashes = append(detailsHashes, commitID.ID)
		}
	}

	// Trim the prefix of the sparse commits
	sparseCommits = sparseCommits[sparseStart:]
	cardinalities = cardinalities[sparseStart:]

	longCommits, err := s.VCS.DetailsMulti(ctx, detailsHashes, false)
	if err != nil {
		return nil, nil, nil, skerr.Fmt("Error retrieving details for: %s", err)
	}
	sklog.Infof("Retrieved %d details", len(longCommits))

	// Load the dense tile.
	denseTile, _, err := s.TraceDB.TileFromCommits(denseCommitIDs)
	if err != nil {
		return nil, nil, nil, err
	}

	sklog.Infof("ncommits: %d", s.NCommits)
	sklog.Infof("dense: %d", len(denseTile.Commits))
	denseIdx := 0
	for idx, commit := range sparseCommits {
		commit.Author = longCommits[idx].Author
		if denseIdx < len(denseTile.Commits) && denseTile.Commits[denseIdx].Hash == commit.Hash {
			denseTile.Commits[denseIdx].Author = longCommits[idx].Author
			denseIdx++
		}
	}

	if len(sparseCommits) > 0 && len(denseTile.Commits) > 0 {
		sklog.Infof("Found %d sparse commits and %d dense commits with starting hashes: %s == %s", len(sparseCommits), len(denseTile.Commits), denseTile.Commits[0].Hash, sparseCommits[0].Hash)
	}

	return denseTile, sparseCommits, cardinalities, nil
}

// getCommitIDs returns instances of tracedb.CommitID from the given hashes that can then be used
// to retrieve data from the tracedb.
func getCommitIDs(indexCommits []*vcsinfo.IndexCommit) []*tracedb.CommitID {
	commitIDs := make([]*tracedb.CommitID, 0, len(indexCommits))
	for _, c := range indexCommits {
		commitIDs = append(commitIDs, &tracedb.CommitID{
			ID:        c.Hash,
			Source:    "master",
			Timestamp: c.Timestamp.Unix(),
		})
	}
	return commitIDs
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

	// Iterate over the tile and copy the whitelisted traces over.
	// Build the paramset in the process.
	paramSet := paramtools.ParamSet{}
	for traceID, trace := range tile.Traces {
		if tiling.Matches(trace, url.Values(s.PubliclyViewableParams)) {
			ret.Traces[traceID] = trace
			paramSet.AddParams(trace.Params())
		}
	}
	ret.ParamSet = paramSet
	sklog.Infof("Whitelisted %d of %d traces.", len(ret.Traces), len(tile.Traces))
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
						return skerr.Fmt("Error retrieving details for commit %s. Got error: %s", commit.Hash, err)
					}

					issueID, err := s.GerritAPI.ExtractIssueFromCommit(longCommit.Body)
					if err != nil {
						return skerr.Fmt("Unable to extract gerrit issue from commit %s. Got error: %s", commit.Hash, err)
					}

					if err := s.TryjobMonitor.CommitIssueBaseline(issueID, longCommit.Author); err != nil {
						return skerr.Fmt("Error commiting tryjob results for commit %s. Got error: %s", commit.Hash, err)
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
