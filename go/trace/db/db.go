// Package tracedb provides a datastore for efficiently storing and retrieving traces.
package db

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	"go.skia.org/infra/go/metrics"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/trace/service"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// CommitID represents the time of a particular commit, where a commit could either be
// a real commit into the repo, or an event like running a trybot.
type CommitID struct {
	Timestamp time.Time
	ID        string // Normally a git hash, but could also be Rietveld patch id.
	Source    string // The branch name, e.g. "master", or the Reitveld issue id.
}

// Entry holds the params and a value for single measurement.
type Entry struct {
	Params map[string]string

	// Value is the value of the measurement.
	//
	// It should be the digest string converted to a []byte, or a float64
	// converted to a little endian []byte. I.e. tiling.Trace.SetAt
	// should be able to consume this value.
	Value []byte
}

// DB represents the interface to any datastore for perf and gold results.
type DB interface {
	// Add new information to the datastore.
	//
	// The values parameter maps a trace id to an Entry.
	//
	// Note that only allowing adding data for a single commit at a time
	// should work well with ingestion while still breaking up writes into
	// shorter actions.
	Add(commitID *CommitID, values map[string]*Entry) error

	// Remove all info for the given commit.
	Remove(commitID *CommitID) error

	// List returns all the CommitID's between begin and end.
	List(begin, end time.Time) ([]*CommitID, error)

	// Create a Tile for the given commit ids. Will build the Tile using the
	// commits in the order they are provided.
	//
	// Note that the Commits in the Tile will only contain the commit id and
	// the timestamp, the Author will not be populated.
	//
	// The Tile's Scale and TileIndex will be set to 0.
	TileFromCommits(commitIDs []*CommitID) (*tiling.Tile, error)

	// Close the datastore.
	Close() error
}

const (
	// MAX_ID_CACHED is the size of the LRU cache TsDB.cache.
	MAX_ID_CACHED = 1000000

	// CHUNK_SIZE is the maximum number of values that are added to the datastore
	// in any one gRPC call.
	CHUNK_SIZE = 100000
)

// TsDB is an implementation of DB that stores traces in traceservice.
type TsDB struct {
	// ts is the client for the traceservice.
	traceService traceservice.TraceServiceClient

	// conn is the underlying connection for ts.
	conn *grpc.ClientConn

	// tb is a TraceBuilder for the type of Tile we're managing, i.e. perf or gold.
	// It is used to build Trace's of the right type and size when building Tiles.
	traceBuilder tiling.TraceBuilder

	// cache is an LRU cache recording is a given trace has its params stored.
	cache *lru.Cache
}

// NewTraceServiceDB creates a new DB that stores the data in the BoltDB backed
// gRPC accessible traceservice.
func NewTraceServiceDB(conn *grpc.ClientConn, traceBuilder tiling.TraceBuilder) (*TsDB, error) {
	ret := &TsDB{
		conn:         conn,
		traceService: traceservice.NewTraceServiceClient(conn),
		traceBuilder: traceBuilder,
		cache:        lru.New(MAX_ID_CACHED),
	}
	go func() {
		ctx := context.Background()
		empty := &traceservice.Empty{}
		liveness := metrics.NewLiveness("tracedb-ping")
		for _ = range time.Tick(time.Minute) {
			if _, err := ret.traceService.Ping(ctx, empty); err == nil {
				liveness.Update()
			}
		}
	}()
	return ret, nil
}

// addChunk adds a set of entries to the datastore at the given CommitID.
func (ts *TsDB) addChunk(ctx context.Context, cid *traceservice.CommitID, chunk map[string]*Entry) error {
	addReq := &traceservice.AddRequest{
		Commitid: cid,
		Entries:  map[string][]byte{},
	}
	addParamsRequest := &traceservice.AddParamsRequest{
		Params: map[string]*traceservice.Params{},
	}
	for traceid, entry := range chunk {
		// Check that all the traceids have their Params.
		if _, ok := ts.cache.Get(traceid); !ok {
			addParamsRequest.Params[traceid] = &traceservice.Params{
				Params: entry.Params,
			}
			ts.cache.Add(traceid, true)
		}
		addReq.Entries[traceid] = entry.Value
	}
	if len(addParamsRequest.Params) > 0 {
		_, err := ts.traceService.AddParams(ctx, addParamsRequest)
		if err != nil {
			return fmt.Errorf("Failed to add params: %s", err)
		}
	}
	if _, err := ts.traceService.Add(ctx, addReq); err != nil {
		return fmt.Errorf("Failed to add values: %s", err)
	}
	return nil
}

// tsCommitID converts a db.CommitID to traceservice.CommitID.
func tsCommitID(c *CommitID) *traceservice.CommitID {
	return &traceservice.CommitID{
		Timestamp: c.Timestamp.Unix(),
		Id:        c.ID,
		Source:    c.Source,
	}
}

// dbCommitID converts a traceservice.CommitID to db.CommitID.
func dbCommitID(c *traceservice.CommitID) *CommitID {
	return &CommitID{
		ID:        c.Id,
		Source:    c.Source,
		Timestamp: time.Unix(c.Timestamp, 0),
	}
}

// Add implements DB.Add().
func (ts *TsDB) Add(commitID *CommitID, values map[string]*Entry) error {
	ctx := context.Background()
	cid := tsCommitID(commitID)
	// Break the values into chunks of CHUNK_SIZE or less and then process each slice.
	// This will keep the total request size down.
	chunks := []map[string]*Entry{}
	chunk := map[string]*Entry{}
	n := 0
	for k, v := range values {
		chunk[k] = v
		if n >= CHUNK_SIZE {
			n = 0
			chunks = append(chunks, chunk)
			chunk = map[string]*Entry{}
		}
		n++
	}
	chunks = append(chunks, chunk)

	for i, chunk := range chunks {
		if err := ts.addChunk(ctx, cid, chunk); err != nil {
			return fmt.Errorf("Failed to add chunk %d: %s", i, err)
		}
	}

	return nil
}

// Remove implements DB.Remove().
func (ts *TsDB) Remove(commitID *CommitID) error {
	removeRequest := &traceservice.RemoveRequest{
		Commitid: tsCommitID(commitID),
	}
	_, err := ts.traceService.Remove(context.Background(), removeRequest)
	return err
}

// List implements DB.List().
func (ts *TsDB) List(begin, end time.Time) ([]*CommitID, error) {
	listReq := &traceservice.ListRequest{
		Begin: begin.Unix(),
		End:   end.Unix(),
	}
	listResp, err := ts.traceService.List(context.Background(), listReq)
	if err != nil {
		return nil, fmt.Errorf("List request failed: %s", err)
	}
	// Copy the data from the ListResponse to a slice of CommitIDs.
	ret := []*CommitID{}
	for _, c := range listResp.Commitids {
		ret = append(ret, dbCommitID(c))
	}
	return ret, nil
}

// TileFromCommits implements DB.TileFromCommits().
func (ts *TsDB) TileFromCommits(commitIDs []*CommitID) (*tiling.Tile, error) {
	ctx := context.Background()

	// Build the Tile.
	tile := tiling.NewTile()
	n := len(commitIDs)
	tile.Commits = make([]*tiling.Commit, n, n)

	// Populate the Tile's commits.
	for i, cid := range commitIDs {
		tile.Commits[i] = &tiling.Commit{
			Hash:       cid.ID,
			CommitTime: cid.Timestamp.Unix(),
		}
	}

	// tileMutex protects access to the Tile. Note that this only means the Tile,
	// while writing values into a Trace that already exists and is the right
	// size is Go routine safe.
	var tileMutex sync.Mutex

	errCh := make(chan error, len(commitIDs))

	// Fill in the data for each commit in it's own Go routine.
	var wg sync.WaitGroup
	for i, cid := range commitIDs {
		wg.Add(1)
		go func(i int, cid *CommitID) {
			defer wg.Done()
			// Load the values for the commit.
			getValuesRequest := &traceservice.GetValuesRequest{
				Commitid: tsCommitID(cid),
			}
			getValuesResponse, err := ts.traceService.GetValues(ctx, getValuesRequest)
			if err != nil {
				errCh <- fmt.Errorf("Failed to get values for %d %#v: %s", i, *cid, err)
				return
			}
			for k, v := range getValuesResponse.Values {
				tr, ok := tile.Traces[k]
				if !ok {
					tileMutex.Lock()
					tile.Traces[k] = ts.traceBuilder(n)
					tileMutex.Unlock()
					tr = tile.Traces[k]
				}
				if err := tr.SetAt(i, v); err != nil {
					errCh <- fmt.Errorf("Unable to convert trace value %d %#v: %s", i, *cid, err)
					return
				}
			}
		}(i, cid)
	}
	wg.Wait()

	// See if any Go routine generated an error.
	select {
	case err, ok := <-errCh:
		if ok {
			return nil, fmt.Errorf("Failed to load trace data: %s", err)
		}
	default:
	}

	// Now load the params for the traces.
	traceids := []string{}
	for k, _ := range tile.Traces {
		traceids = append(traceids, k)
	}
	// Request the params in CHUNK_SIZE slices of traceids.
	for len(traceids) > 0 {
		req := &traceservice.GetParamsRequest{}
		if len(traceids) > CHUNK_SIZE {
			req.Traceids = traceids[:CHUNK_SIZE]
			traceids = traceids[CHUNK_SIZE:]
		} else {
			req.Traceids = traceids
			traceids = []string{}
		}
		resp, err := ts.traceService.GetParams(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("Failed to load params: %s", err)
		}
		for traceid, params := range resp.Params {
			dst := tile.Traces[traceid].Params()
			for k, v := range params.Params {
				dst[k] = v
			}
		}
	}

	// Rebuild the ParamSet.
	tiling.GetParamSet(tile.Traces, tile.ParamSet)
	return tile, nil
}

// Close the underlying connection to the datastore.
func (ts *TsDB) Close() error {
	return ts.conn.Close()
}
