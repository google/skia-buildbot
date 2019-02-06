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
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digeststore"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/tryjobs"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
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
	DigestStore          digeststore.DigestStore
	EventBus             eventbus.EventBus
	TryjobStore          tryjobstore.TryjobStore
	TryjobMonitor        *tryjobs.TryjobMonitor
	GerritAPI            gerrit.GerritInterface
	GStorageClient       *GStorageClient
	Baseliner            *Baseliner
	Git                  *gitinfo.GitInfo
	WhiteListQuery       paramtools.ParamSet
	IsAuthoritative      bool
	SiteURL              string

	// IsSparseTile indicates that new tiles should be condensed by removing commits that have no data.
	IsSparseTile bool

	// NCommits is the number of commits we should consider. If NCommits is
	// 0 or smaller all commits in the last tile will be considered.
	NCommits int

	// Internal variables used to cache trimmed tiles.
	lastTrimmedTile        *tiling.Tile
	lastTrimmedIgnoredTile *tiling.Tile
	lastIgnoreRev          int64
	lastIgnoreRules        paramtools.ParamMatcher
	mutex                  sync.Mutex
}

// TODO(stephana): Baseliner will eventually factored into the baseline package and
// InitBaseliner should go away.

// InitBaseliner initializes the Baseliner instance from values already set on the storage instance.
func (s *Storage) InitBaseliner() error {
	var err error
	s.Baseliner, err = NewBaseliner(s.GStorageClient, s.ExpectationsStore, s.IssueExpStoreFactory, s.TryjobStore, s.Git)
	return err
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
// The first tile is send immediately.
// Should the call to read a new tile fail it will send that last
// successfully read tile. Thus it guarantees to send a tile in the provided
// interval, assuming at least one tile could be read.
func (s *Storage) GetTileStreamNow(interval time.Duration) <-chan *types.TilePair {
	retCh := make(chan *types.TilePair)

	go func() {
		var lastTile *types.TilePair = nil

		readOneTile := func() {
			if tilePair, err := s.GetLastTileTrimmed(); err != nil {
				// Log the error and send the best tile we have right now.
				sklog.Errorf("Error reading tile: %s", err)
				if lastTile != nil {
					retCh <- lastTile
				}
			} else {
				lastTile = tilePair
				retCh <- tilePair
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

// TODO(stephana): Expand the Tile and TilePair types to make querying faster.
// i.e. add traces as an array so that iteration can be done in parallel and
// add map[hash]Commit to do faster commit lookup (-> Remove tiling.FindCommit).

// GetLastTrimmed returns the last tile as read-only trimmed to contain at
// most NCommits. It caches trimmed tiles as long as the underlying tiles
// do not change.
func (s *Storage) GetLastTileTrimmed() (*types.TilePair, error) {
	// Retrieve the most recent tile.
	var tile *tiling.Tile
	var err error

	// If it's a sparse tile, we build it anew.
	if s.IsSparseTile {
		tile, err = s.getNewCondensedTile(s.lastTrimmedTile)
		if err != nil {
			return nil, skerr.Fmt("Error getting condensed tile: %s", err)
		}
	} else {
		tile = s.MasterTileBuilder.GetTile()
	}
	tile = s.getWhiteListedTile(tile)

	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.NCommits <= 0 {
		return &types.TilePair{
			Tile:            tile,
			TileWithIgnores: tile,
		}, nil
	}

	currentIgnoreRev := s.IgnoreStore.Revision()

	// Check if the tile hasn't changed and the ignores haven't changed.
	if s.lastTrimmedTile != nil && tile == s.lastTrimmedTile && s.lastTrimmedIgnoredTile != nil && currentIgnoreRev == s.lastIgnoreRev {
		return &types.TilePair{
			Tile:            s.lastTrimmedIgnoredTile,
			TileWithIgnores: s.lastTrimmedTile,
			IgnoreRules:     s.lastIgnoreRules,
		}, nil
	}

	// check if all the expectations of all commits have been added to the tile.
	s.checkCommitableIssues(tile)

	// Get the tile without the ignored traces.
	retIgnoredTile, ignoreRules, err := FilterIgnored(tile, s.IgnoreStore)
	if err != nil {
		return nil, err
	}

	// Cache this tile.
	s.lastIgnoreRev = currentIgnoreRev
	s.lastTrimmedTile = tile
	s.lastTrimmedIgnoredTile = retIgnoredTile
	s.lastIgnoreRules = ignoreRules

	return &types.TilePair{
		Tile:            s.lastTrimmedIgnoredTile,
		TileWithIgnores: s.lastTrimmedTile,
		IgnoreRules:     s.lastIgnoreRules,
	}, nil
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

// GetOrUpdateDigestInfo is a helper function that retrieves the DigestInfo for
// the given test name/digest pair or updates the underlying info if it is not
// in the digest store yet.
func (s *Storage) GetOrUpdateDigestInfo(testName, digest string, commit *tiling.Commit) (*digeststore.DigestInfo, error) {
	digestInfo, ok, err := s.DigestStore.Get(testName, digest)
	if err != nil {
		sklog.Warningf("Error retrieving digest info: %s", err)
		return &digeststore.DigestInfo{Exception: err.Error()}, nil
	}

	if ok {
		return digestInfo, nil
	}
	digestInfo = &digeststore.DigestInfo{
		TestName: testName,
		Digest:   digest,
		First:    commit.CommitTime,
		Last:     commit.CommitTime,
	}
	err = s.DigestStore.Update([]*digeststore.DigestInfo{digestInfo})
	if err != nil {
		return nil, err
	}

	return digestInfo, nil
}

func (s *Storage) GetExpectationsForCommit(parentCommit string) (types.Expectations, error) {
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

// TODO(stephana): Add unit test for getNewCondensedTile.

// getNewCondensedTile returns a tile that contains only commits that have at least one
// nonempty entry. If lastTile is not nil, its first commit is used as a starting point to
// fetch the tiles necessary to build the condensed tile (from several "sparse" tiles.)
func (s *Storage) getNewCondensedTile(lastTile *tiling.Tile) (*tiling.Tile, error) {
	ctx := context.TODO()
	var err error

	// Update the repository.
	if err := s.Git.Update(ctx, true, false); err != nil {
		return nil, err
	}

	// Get enough commits to fill a tile.
	var hashes []string
	if lastTile != nil {
		// Use the first commit of the last tile.
		commitLen := lastTile.LastCommitIndex() + 1
		if commitLen > 0 {
			firstCommitTime := time.Unix(lastTile.Commits[0].CommitTime, 0).Add(-500 * time.Millisecond)
			hashes = s.Git.From(firstCommitTime)
		}
	}

	if len(hashes) == 0 {
		// Get the last 100 windows of interest.
		// TODO: Find a better size. This assumes that there are not many traces in each tile.
		// and is probably overkill.
		tileSize := s.NCommits * 100
		hashes = s.Git.LastN(ctx, tileSize)
	}
	commitIDs := getCommitIDs(hashes, s.Git)

	// Build a Tile from those CommitIDs.
	sparseTile, _, err := s.TraceDB.TileFromCommits(commitIDs)
	if err != nil {
		return nil, skerr.Fmt("Failed to load tile from commitIDs: %s", err)
	}

	targetCommits := make([]string, 0, s.NCommits)
	commitsLen := sparseTile.LastCommitIndex() + 1
	commitIndices := []int{}
	for idx := 0; idx < commitsLen; idx++ {
		for _, trace := range sparseTile.Traces {
			gTrace := trace.(*types.GoldenTrace)
			if gTrace.Values[idx] != types.MISSING_DIGEST {
				targetCommits = append(targetCommits, sparseTile.Commits[idx].Hash)
				commitIndices = append(commitIndices, idx)
				break
			}
		}
	}
	commitIDs = getCommitIDs(targetCommits, s.Git)
	denseTile, _, err := s.TraceDB.TileFromCommits(commitIDs)
	if err != nil {
		return nil, skerr.Fmt("Failed to load dense tile from commitIDs: %s", err)
	}

	if commitLen := denseTile.LastCommitIndex() + 1; commitLen > s.NCommits {
		if denseTile, err = denseTile.Trim(0, s.NCommits); err != nil {
			return nil, skerr.Fmt("Error trimming dense tile: %s", err)
		}
	}

	// Now populate the author for each commit.
	for _, c := range denseTile.Commits {
		details, err := s.Git.Details(ctx, c.Hash, true)
		if err != nil {
			return nil, skerr.Fmt("Couldn't fill in author info in tile for commit %s: %s", c.Hash, err)
		}
		c.Author = details.Author
	}
	return denseTile, nil
}

// getCommitIDs returns instances of tracedb.CommitID from the given hashes that can then be used
// to retrieve data from the tracedb.
func getCommitIDs(hashes []string, git *gitinfo.GitInfo) []*tracedb.CommitID {
	commitIDs := make([]*tracedb.CommitID, 0, len(hashes))
	for _, h := range hashes {
		commitIDs = append(commitIDs, &tracedb.CommitID{
			ID:        h,
			Source:    "master",
			Timestamp: git.Timestamp(h).Unix(),
		})
	}
	return commitIDs
}

// checkCommitableIssues checks all commits of the current tile whether
// the associated expectations have been added to the baseline of the master.
func (s *Storage) checkCommitableIssues(tile *tiling.Tile) {
	go func() {
		var egroup errgroup.Group

		for _, commit := range tile.Commits[:tile.LastCommitIndex()+1] {
			func(commit *tiling.Commit) {
				egroup.Go(func() error {
					longCommit, err := s.Git.Details(context.Background(), commit.Hash, true)
					if err != nil {
						return sklog.FmtErrorf("Error retrieving details for commit %s. Got error: %s", commit.Hash, err)
					}

					issueID, err := s.GerritAPI.ExtractIssueFromCommit(longCommit.Body)
					if err != nil {
						return sklog.FmtErrorf("Unable to extract gerrit issue from commit %s. Got error: %s", commit.Hash, err)
					}

					issueExpStore := s.IssueExpStoreFactory(issueID)
					issueExps, err := issueExpStore.Get()
					if err != nil {
						return sklog.FmtErrorf("Unable to retrieve expecations for issue %d: %s", issueID, err)
					}

					if err := baseline.CommitIssueBaseline(issueID, longCommit.Author, issueExps.TestExp(), s.TryjobStore, s.ExpectationsStore); err != nil {
						return sklog.FmtErrorf("Error retrieving details for commit %s. Got error: %s", commit.Hash, err)
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
