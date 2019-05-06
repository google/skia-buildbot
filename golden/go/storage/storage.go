package storage

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/flynn/json5"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/tryjobs"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
)

const (
	// maxNSparseCommits is the maximum number of commits we are considering when condensing a
	// sparse tile into a dense tile by removing commits that contain no data.
	// This should be changed or made a config option when we consider going back more commits makes
	// sense.
	maxNSparseCommits = 3000
)

// Storage is a container struct for the various storage objects we are using.
// It is intended to reduce parameter lists as we pass around storage objects.
type Storage struct {
	DiffStore            diff.DiffStore
	ExpectationsStore    expstorage.ExpectationsStore
	IssueExpStoreFactory expstorage.IssueExpStoreFactory
	IgnoreStore          ignore.IgnoreStore
	TraceDB              tracedb.DB
	MasterTileBuilder    tracedb.MasterTileBuilder
	EventBus             eventbus.EventBus
	TryjobStore          tryjobstore.TryjobStore
	TryjobMonitor        *tryjobs.TryjobMonitor
	GerritAPI            gerrit.GerritInterface
	GCSClient            GCSClient
	Baseliner            baseline.Baseliner
	VCS                  vcsinfo.VCS
	WhiteListQuery       paramtools.ParamSet
	IsAuthoritative      bool
	SiteURL              string

	// IsSparseTile indicates that new tiles should be condensed by removing commits that have no data.
	IsSparseTile bool

	// NCommits is the number of commits we should consider. If NCommits is
	// 0 or smaller all commits in the last tile will be considered.
	NCommits int

	// Internal variables used to cache tiles.
	lastCpxTile   *types.ComplexTile
	lastTimeStamp time.Time
	mutex         sync.Mutex
}

// LoadWhiteList loads the given JSON5 file that defines that query to
// whitelist traces. If the given path is empty or the file cannot be parsed
// an error will be returned.
func (s *Storage) LoadWhiteList(fName string) error {
	if fName == "" {
		return fmt.Errorf("No white list file provided.")
	}

	f, err := os.Open(fName)
	if err != nil {
		return fmt.Errorf("Unable open file %s. Got error: %s", fName, err)
	}
	defer util.Close(f)

	if err := json5.NewDecoder(f).Decode(&s.WhiteListQuery); err != nil {
		return err
	}

	// Make sure the whitelist is not empty.
	empty := true
	for _, values := range s.WhiteListQuery {
		if empty = len(values) == 0; !empty {
			break
		}
	}
	if empty {
		return fmt.Errorf("Whitelist in %s cannot be empty.", fName)
	}
	sklog.Infof("Whitelist loaded from %s", fName)
	return nil
}

// GetTileStreamNow is a utility function that reads tiles in the given
// interval and sends them on the returned channel.
// The first tile is sent immediately.
// Should the call to read a new tile fail it will send that last
// successfully read tile. Thus it guarantees to send a tile in the provided
// interval, assuming at least one tile could be read.
// The metricsTag allows for us to monitor how long individual tile streams
// take, in the unlikely event there are multiple failures of the tile in a row.
func (s *Storage) GetTileStreamNow(interval time.Duration, metricsTag string) <-chan *types.ComplexTile {
	retCh := make(chan *types.ComplexTile)
	lastTileStreamed := metrics2.NewLiveness("last_tile_streamed", map[string]string{
		"source": metricsTag,
	})
	go func() {
		var lastTile *types.ComplexTile = nil

		readOneTile := func() {
			if tile, err := s.GetLastTileTrimmed(); err != nil {
				// Log the error and send the best tile we have right now.
				sklog.Errorf("Error reading tile: %s", err)
				if lastTile != nil {
					retCh <- lastTile
				}
			} else {
				lastTile = tile
				lastTileStreamed.Reset()
				retCh <- tile
			}
		}

		readOneTile()
		for range time.Tick(interval) {
			readOneTile()
		}
	}()

	return retCh
}

// DrainChangeChannel removes everything from the channel thats currently
// buffered or ready to be read.
func DrainChangeChannel(ch <-chan types.TestExp) {
Loop:
	for {
		select {
		case <-ch:
		default:
			break Loop
		}
	}
}

var tileCacheTime = 3 * time.Minute

// TODO(stephana): Expand the Tile type to make querying faster.
// i.e. add traces as an array so that iteration can be done in parallel and
// add map[hash]Commit to do faster commit lookup (-> Remove tiling.FindCommit).

// GetLastTrimmed returns the last tile as read-only trimmed to contain at
// most NCommits. It caches trimmed tiles as long as the underlying tiles
// do not change.
func (s *Storage) GetLastTileTrimmed() (*types.ComplexTile, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// If the tile was updated within a certain time window just return it without
	// calculating it again.
	if s.lastCpxTile != nil && (time.Now().Sub(s.lastTimeStamp) < tileCacheTime) {
		sklog.Infof("short circuiting get tile, because it's still new: %s < %s", time.Now().Sub(s.lastTimeStamp), tileCacheTime)
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
	rawTile = s.getWhiteListedTile(rawTile)
	if s.NCommits <= 0 {
		return types.NewComplexTile(rawTile).SetSparse(nil, nil), nil
	}

	// Get the ignore revision and check if the tile has changed at all.
	// Note: This only applies to tiles that are not sparse.
	currentIgnoreRev := s.IgnoreStore.Revision()
	if s.lastCpxTile.FromSame(rawTile, currentIgnoreRev) {
		return s.lastCpxTile, nil
	}

	// Construct the new complex tile
	cpxTile := types.NewComplexTile(rawTile).SetSparse(sparseCommits, cardinalities)

	// Get the tile without the ignored traces and update the complex tile.
	retIgnoredTile, ignoreRules, err := FilterIgnored(rawTile, s.IgnoreStore)
	if err != nil {
		return nil, err
	}
	cpxTile.SetIgnoreRules(retIgnoredTile, ignoreRules, currentIgnoreRev)

	// check if all the expectations of all commits have been added to the tile.
	s.checkCommitableIssues(cpxTile)

	// Update the cached tile and return the result.
	s.lastCpxTile = cpxTile
	s.lastTimeStamp = time.Now()
	return cpxTile, nil
}

// FilterIgnored returns a copy of the given tile with all traces removed
// that match the ignore rules in the given ignore store. It also returns the
// ignore rules for later matching.
func FilterIgnored(inputTile *tiling.Tile, ignoreStore ignore.IgnoreStore) (*tiling.Tile, paramtools.ParamMatcher, error) {
	ignores, err := ignoreStore.List(false)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to get ignores to filter tile: %s", err)
	}

	// Now copy the tile by value.
	ret := inputTile.Copy()

	// Then remove traces that should be ignored.
	ignoreQueries, err := ignore.ToQuery(ignores)
	if err != nil {
		return nil, nil, err
	}
	for id, tr := range ret.Traces {
		for _, q := range ignoreQueries {
			if tiling.Matches(tr, q) {
				delete(ret.Traces, id)
				continue
			}
		}
	}

	ignoreRules := make([]paramtools.ParamSet, len(ignoreQueries), len(ignoreQueries))
	for idx, q := range ignoreQueries {
		ignoreRules[idx] = paramtools.ParamSet(q)
	}
	return ret, ignoreRules, nil
}

func (s *Storage) GetExpectationsForCommit(parentCommit string) (types.TestExpBuilder, error) {
	return nil, sklog.FmtErrorf("Not implemented yet !")
}

// getWhiteListedTile creates a new tile from the given tile that contains
// only traces that match the whitelist that was loaded earlier.
func (s *Storage) getWhiteListedTile(tile *tiling.Tile) *tiling.Tile {
	if s.WhiteListQuery == nil {
		return tile
	}

	// filter tile.
	ret := &tiling.Tile{
		Traces:  make(map[string]tiling.Trace, len(tile.Traces)),
		Commits: tile.Commits,
	}

	// Iterate over the tile and copy the whitelisted traces over.
	// Build the paramset in the process.
	paramSet := paramtools.ParamSet{}
	for traceID, trace := range tile.Traces {
		if tiling.Matches(trace, url.Values(s.WhiteListQuery)) {
			ret.Traces[traceID] = trace
			paramSet.AddParams(trace.Params())
		}
	}
	ret.ParamSet = paramSet
	sklog.Infof("Whitelisted %d of %d traces.", len(ret.Traces), len(tile.Traces))
	return ret
}

// getCondensedTile returns a tile that contains only commits that have at least one
// nonempty entry. If lastTile is not nil, its first commit is used as a starting point to
// fetch the tiles necessary to build the condensed tile (from several "sparse" tiles.)
func (s *Storage) getCondensedTile(ctx context.Context, lastCpxTile *types.ComplexTile) (*tiling.Tile, []*tiling.Commit, []int, error) {
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

// checkCommitableIssues checks all commits of the current tile whether
// the associated expectations have been added to the baseline of the master.
func (s *Storage) checkCommitableIssues(cpxTile *types.ComplexTile) {
	go func() {
		var egroup errgroup.Group

		for _, commit := range cpxTile.AllCommits() {
			func(commit *tiling.Commit) {
				egroup.Go(func() error {
					longCommit, err := s.VCS.Details(context.Background(), commit.Hash, false)
					if err != nil {
						return sklog.FmtErrorf("Error retrieving details for commit %s. Got error: %s", commit.Hash, err)
					}

					issueID, err := s.GerritAPI.ExtractIssueFromCommit(longCommit.Body)
					if err != nil {
						return sklog.FmtErrorf("Unable to extract gerrit issue from commit %s. Got error: %s", commit.Hash, err)
					}

					if err := s.TryjobMonitor.CommitIssueBaseline(issueID, longCommit.Author); err != nil {
						return sklog.FmtErrorf("Error commiting tryjob results for commit %s. Got error: %s", commit.Hash, err)
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
