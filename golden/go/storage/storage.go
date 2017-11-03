package storage

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/flynn/json5"

	"google.golang.org/api/option"

	gstorage "cloud.google.com/go/storage"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digeststore"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/trybot"
	"go.skia.org/infra/golden/go/types"
)

// Storage is a container struct for the various storage objects we are using.
// It is intended to reduce parameter lists as we pass around storage objects.
type Storage struct {
	DiffStore         diff.DiffStore
	ExpectationsStore expstorage.ExpectationsStore
	IgnoreStore       ignore.IgnoreStore
	MasterTileBuilder tracedb.MasterTileBuilder
	BranchTileBuilder tracedb.BranchTileBuilder
	DigestStore       digeststore.DigestStore
	EventBus          eventbus.EventBus
	TrybotResults     *trybot.TrybotResults
	RietveldAPI       *rietveld.Rietveld
	GerritAPI         *gerrit.Gerrit
	GStorageClient    *GStorageClient

	// NCommits is the number of commits we should consider. If NCommits is
	// 0 or smaller all commits in the last tile will be considered.
	NCommits int

	quiteListQuery paramtools.ParamSet

	// Internal variables used to cache trimmed tiles.
	lastTrimmedTile        *tiling.Tile
	lastTrimmedIgnoredTile *tiling.Tile
	lastIgnoreRev          int64
	mutex                  sync.Mutex
	whiteListQuery         url.Values
}

// LoadWhiteList loads the given JSON5 file that defines that query to
// whitelist traces. If the given path is emtpy or the file cannot be parsed
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

	if err := json5.NewDecoder(f).Decode(&s.whiteListQuery); err != nil {
		return err
	}

	// Make sure the whitelist is not empty.
	empty := true
	for _, values := range s.whiteListQuery {
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
func DrainChangeChannel(ch <-chan map[string]types.TestClassification) {
Loop:
	for {
		select {
		case <-ch:
		default:
			break Loop
		}
	}
}

// GetLastTrimmed returns the last tile as read-only trimmed to contain at
// most NCommits. It caches trimmed tiles as long as the underlying tiles
// do not change.
func (s *Storage) GetLastTileTrimmed() (*types.TilePair, error) {
	// Retieve the most recent tile.
	tile := s.getWhiteListedTile(s.MasterTileBuilder.GetTile())

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
		}, nil
	}

	// Get the tile without the ignored traces.
	retIgnoredTile, err := FilterIgnored(tile, s.IgnoreStore)
	if err != nil {
		return nil, err
	}

	// Cache this tile.
	s.lastIgnoreRev = currentIgnoreRev
	s.lastTrimmedTile = tile
	s.lastTrimmedIgnoredTile = retIgnoredTile

	return &types.TilePair{
		Tile:            s.lastTrimmedIgnoredTile,
		TileWithIgnores: s.lastTrimmedTile,
	}, nil
}

// FilterIgnored returns a copy of the given tile with all traces removed
// that match the ignore rules in the given ignore store.
func FilterIgnored(inputTile *tiling.Tile, ignoreStore ignore.IgnoreStore) (*tiling.Tile, error) {
	ignores, err := ignoreStore.List(false)
	if err != nil {
		return nil, fmt.Errorf("Failed to get ignores to filter tile: %s", err)
	}

	// Now copy the tile by value.
	ret := inputTile.Copy()

	// Then remove traces that should be ignored.
	ignoreQueries, err := ignore.ToQuery(ignores)
	if err != nil {
		return nil, err
	}
	for id, tr := range ret.Traces {
		for _, q := range ignoreQueries {
			if tiling.Matches(tr, q) {
				delete(ret.Traces, id)
				continue
			}
		}
	}
	return ret, nil
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

// GetTileFromTimeRange returns a tile that contains the commits in the given time range.
func (s *Storage) GetTileFromTimeRange(begin, end time.Time) (*tiling.Tile, error) {
	commitIDs, err := s.BranchTileBuilder.ListLong(begin, end, "master")
	if err != nil {
		return nil, fmt.Errorf("Failed retrieving commitIDs in range %s to %s. Got error: %s", begin, end, err)
	}

	tile, err := s.BranchTileBuilder.CachedTileFromCommits(tracedb.ShortFromLong(commitIDs))
	if err != nil {
		return nil, err
	}
	return s.getWhiteListedTile(tile), nil
}

// getWhiteListedTile creates a new tile from the given tile that contains
// only traces that match the whitelist that was loaded earlier.
func (s *Storage) getWhiteListedTile(tile *tiling.Tile) *tiling.Tile {
	if s.whiteListQuery == nil {
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
		if tiling.Matches(trace, s.whiteListQuery) {
			ret.Traces[traceID] = trace
			paramSet.AddParams(trace.Params())
		}
	}
	ret.ParamSet = paramSet
	sklog.Infof("Whitelisted %d of %d traces.", len(ret.Traces), len(tile.Traces))
	return ret
}

// GStorageClient provides read/write to Google storage for one-off
// use cases. Currently only for writing the list of known digests.
type GStorageClient struct {
	storageClient *gstorage.Client
	bucketName    string
	hashFilePath  string
}

// NewGStorageClient creates a new instance of GStorage client. bucket
// contains the name of the bucket where to store the know digests file and
// hashFilePath provices the path from the bucket root.
func NewGStorageClient(client *http.Client, bucketName, hashFilePath string) (*GStorageClient, error) {
	storageClient, err := gstorage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	return &GStorageClient{
		storageClient: storageClient,
		bucketName:    bucketName,
		hashFilePath:  hashFilePath,
	}, nil
}

// WriteKnownDigests writes the given list of digests to GS as newline
// separated strings.
func (g *GStorageClient) WriteKownDigests(digests []string) error {
	if (g.bucketName == "") || (g.hashFilePath == "") {
		return fmt.Errorf("Cannot write known digests file. Either bucket or path missing.")
	}

	ctx := context.Background()
	target := g.storageClient.Bucket(g.bucketName).Object(g.hashFilePath)
	writer := target.NewWriter(ctx)
	writer.ObjectAttrs.ContentType = "text/plain"
	writer.ObjectAttrs.ACL = []gstorage.ACLRule{{Entity: gstorage.AllUsers, Role: gstorage.RoleReader}}
	defer util.Close(writer)

	for _, digest := range digests {
		if _, err := writer.Write([]byte(digest + "\n")); err != nil {
			return err
		}
	}
	sklog.Infof("Known hashes written to %s/%s", g.bucketName, g.hashFilePath)
	return nil
}

// LoadKnownDigests loads the digests that have previously been written
// to GS via WriteKnownDigests.
func (g *GStorageClient) LoadKownDigests() ([]string, error) {
	ctx := context.Background()
	target := g.storageClient.Bucket(g.bucketName).Object(g.hashFilePath)

	// If the item doesn't exist this will return gstorage.ErrObjectNotExist
	_, err := target.Attrs(ctx)
	if err != nil {
		return nil, err
	}

	reader, err := target.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer util.Close(reader)

	scanner := bufio.NewScanner(reader)
	ret := []string{}
	for scanner.Scan() {
		ret = append(ret, scanner.Text())
	}
	return ret, nil
}

// RemoveKownDigests removes the file with know digests. Primarily used
// for testing.
func (g *GStorageClient) RemoveKownDigests() error {
	target := g.storageClient.Bucket(g.bucketName).Object(g.hashFilePath)
	return target.Delete(context.Background())
}
