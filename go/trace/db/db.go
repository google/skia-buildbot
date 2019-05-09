// Package tracedb provides a datastore for efficiently storing and retrieving traces.
package db

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	traceservice "go.skia.org/infra/go/trace/service"
	"google.golang.org/grpc"
)

// CommitID represents the time of a particular commit, where a commit could either be
// a real commit into the repo, or an event like running a trybot.
type CommitID struct {
	Timestamp int64  `json:"ts"`
	ID        string `json:"id"`     // The git hash
	Source    string `json:"source"` // The branch name, e.g. "master"
}

func (c CommitID) String() string {
	return fmt.Sprintf("%s:%s:%d", c.ID, c.Source, c.Timestamp)
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
	Add(commitID *CommitID, values map[tiling.TraceId]*Entry) error

	// List returns all the CommitID's between begin and end.
	List(begin, end time.Time) ([]*CommitID, error)

	// Create a Tile for the given commit ids. Will build the Tile using the
	// commits in the order they are provided.
	//
	// Note that the Commits in the Tile will only contain the commit id and
	// the timestamp, the Author will not be populated.
	//
	// The Tile's Scale and TileIndex will be set to 0.
	//
	// The md5 hashes for each commitid are also returned.
	TileFromCommits(commitIDs []*CommitID) (*tiling.Tile, []string, error)

	// Close the datastore.
	Close() error
}

const (
	// MAX_ID_CACHED is the size of the LRU cache TsDB.cache.
	MAX_ID_CACHED = 1000000

	// CHUNK_SIZE is the maximum number of values that are added to the datastore
	// in any one gRPC call.
	CHUNK_SIZE = 5000

	// MAX_MESSAGE_SIZE is the maximum grpc message size.
	MAX_MESSAGE_SIZE = 1024 * 1024 * 1024
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

	// mutex protects access to the caches.
	mutex sync.Mutex

	// cache is an LRU cache, records if a given trace has its params stored
	// during a previous Add(), keyed by traceid and maps to a bool
	cache *lru.Cache

	// paramsCache is a cache of params retrieved from tracedb, keyed by traceid.
	paramsCache map[tiling.TraceId]map[string]string

	// id64Cache is a cache of traceids retrieved from tracedb, keyed by trace64id.
	id64Cache map[uint64]tiling.TraceId

	// ctx is the gRPC context.
	ctx context.Context

	// clearMutex is a mutex to protect clearing of the caches. TileFromCommits
	// will get read locks, and a go routine that periodically checks the cache
	// sizes will gain a write lock. That way concurrent TileFromCommits calls
	// will proceed, but none will be running when the caches are potentially
	// cleared.
	clearMutex sync.RWMutex
}

// NewTraceServiceDB creates a new DB that stores the data in the BoltDB backed
// gRPC accessible traceservice.
func NewTraceServiceDB(conn *grpc.ClientConn, traceBuilder tiling.TraceBuilder) (*TsDB, error) {
	ret := &TsDB{
		conn:         conn,
		traceService: traceservice.NewTraceServiceClient(conn),
		traceBuilder: traceBuilder,
		cache:        lru.New(MAX_ID_CACHED),
		paramsCache:  map[tiling.TraceId]map[string]string{},
		id64Cache:    map[uint64]tiling.TraceId{},
		ctx:          context.Background(),
	}

	// This ping causes the client to try and reach the backend. If the backend
	// is down, it will keep trying until it's up.
	if err := ret.ping(); err != nil {
		return nil, err
	}

	// Liveness metric.
	go func() {
		liveness := metrics2.NewLiveness("ping", map[string]string{"module": "tracedb"})
		for range time.Tick(time.Minute) {
			if ret.ping() == nil {
				liveness.Reset()
			}
		}
	}()

	// Keep the caches sizes in check.
	go func() {
		for range time.Tick(15 * time.Minute) {
			ret.clearMutex.Lock()
			if len(ret.paramsCache) > MAX_ID_CACHED {
				ret.paramsCache = map[tiling.TraceId]map[string]string{}
				sklog.Warning("Had to clear paramsCache, this is unexpected. MAX_ID_CACHED too small?")
			}
			if len(ret.id64Cache) > MAX_ID_CACHED {
				ret.id64Cache = map[uint64]tiling.TraceId{}
				sklog.Warning("Had to clear id64Cache, this is unexpected. MAX_ID_CACHED too small?")
			}
			ret.clearMutex.Unlock()
		}
	}()
	return ret, nil
}

func (ts *TsDB) ping() error {
	_, err := ts.traceService.Ping(ts.ctx, &traceservice.Empty{})
	return err
}

// addChunk adds a set of entries to the datastore at the given CommitID.
func (ts *TsDB) addChunk(ctx context.Context, cid *traceservice.CommitID, chunk map[tiling.TraceId]*Entry) error {
	if len(chunk) == 0 {
		return nil
	}
	addReq := &traceservice.AddRequest{
		Commitid: cid,
		Values:   []*traceservice.ValuePair{},
	}
	addParamsRequest := &traceservice.AddParamsRequest{
		Params: []*traceservice.ParamsPair{},
	}
	for traceid, entry := range chunk {
		// Check that all the traceids have their Params.
		ts.mutex.Lock()
		if _, ok := ts.cache.Get(traceid); !ok {
			addParamsRequest.Params = append(addParamsRequest.Params, &traceservice.ParamsPair{
				Key:    string(traceid),
				Params: entry.Params,
			})
			ts.cache.Add(traceid, true)
		}
		ts.mutex.Unlock()
		addReq.Values = append(addReq.Values, &traceservice.ValuePair{
			Key:   string(traceid),
			Value: entry.Value,
		})
	}
	if len(addParamsRequest.Params) > 0 {
		// TODO(stephana): We need to fix the call to AddParams. If it fails the
		// the DB ends up in an inconsistent state and traceService.GetParams
		// for the failing traceID will cause a panic.

		if _, err := ts.traceService.AddParams(ctx, addParamsRequest); err != nil {
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
		Timestamp: c.Timestamp,
		Id:        c.ID,
		Source:    c.Source,
	}
}

// dbCommitID converts a traceservice.CommitID to db.CommitID.
func dbCommitID(c *traceservice.CommitID) *CommitID {
	return &CommitID{
		ID:        c.Id,
		Source:    c.Source,
		Timestamp: c.Timestamp,
	}
}

// Add implements DB.Add().
func (ts *TsDB) Add(commitID *CommitID, values map[tiling.TraceId]*Entry) error {
	ctx := context.Background()
	cid := tsCommitID(commitID)
	// Break the values into chunks of CHUNK_SIZE or less and then process each slice.
	// This will keep the total request size down.
	chunks := []map[tiling.TraceId]*Entry{}
	chunk := map[tiling.TraceId]*Entry{}
	n := 0
	for k, v := range values {
		chunk[k] = v
		if n >= CHUNK_SIZE {
			n = 0
			chunks = append(chunks, chunk)
			chunk = map[tiling.TraceId]*Entry{}
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
func (ts *TsDB) TileFromCommits(commitIDs []*CommitID) (*tiling.Tile, []string, error) {
	ts.clearMutex.RLock()
	ts.clearMutex.RUnlock()
	ctx := context.Background()

	// Build the Tile.
	tile := tiling.NewTile()
	n := len(commitIDs)
	tile.Commits = make([]*tiling.Commit, n, n)
	hash := make([]string, n)

	// Populate the Tile's commits.
	for i, cid := range commitIDs {
		tile.Commits[i] = &tiling.Commit{
			Hash:       cid.ID,
			CommitTime: cid.Timestamp,
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
			getRawValues, err := ts.traceService.GetValuesRaw(ctx, getValuesRequest)
			if err != nil {
				errCh <- fmt.Errorf("Failed to get values for %d %#v: %s", i, *cid, err)
				return
			}
			// Convert raw response into values.
			ci, err := traceservice.NewCommitInfo(getRawValues.Value)
			if err != nil {
				errCh <- fmt.Errorf("Failed to convert values for %d %#v: %s", i, *cid, err)
				return
			}
			// Now make sure we have all the traceids for the trace64ids in ci.
			missingKeys64 := []uint64{}
			ts.mutex.Lock()
			for id64 := range ci.Values {
				if _, ok := ts.id64Cache[id64]; !ok {
					missingKeys64 = append(missingKeys64, id64)
				}
			}
			ts.mutex.Unlock()
			if len(missingKeys64) > 0 {
				traceidsRequest := &traceservice.GetTraceIDsRequest{
					Id: missingKeys64,
				}
				traceids, err := ts.traceService.GetTraceIDs(ctx, traceidsRequest)
				if err != nil {
					errCh <- fmt.Errorf("Failed to get traceids for trace64ids for %d %#v: %s", i, *cid, err)
					return
				}
				ts.mutex.Lock()
				for _, tid := range traceids.Ids {
					ts.id64Cache[tid.Id64] = tiling.TraceId(tid.Id)
				}
				ts.mutex.Unlock()
			}

			ts.mutex.Lock()
			for id64, rawValue := range ci.Values {
				if rawValue == nil {
					sklog.Errorf("Got a nil rawValue in response: %s", err)
					continue
				}
				traceid := ts.id64Cache[id64]
				tileMutex.Lock()
				tr, ok := tile.Traces[traceid]
				if !ok || tr == nil {
					tile.Traces[traceid] = ts.traceBuilder(n)
					tr = tile.Traces[traceid]
				}
				tileMutex.Unlock()
				if tr == nil {
					sklog.Errorf("Trace was still nil for key: %v", traceid)
					continue
				}
				if err := tr.SetAt(i, rawValue); err != nil {
					errCh <- fmt.Errorf("Unable to convert trace value %d %#v: %s", i, *cid, err)
					return
				}
			}
			// Fill in the commits hash.
			hash[i] = getRawValues.Md5
			ts.mutex.Unlock()
		}(i, cid)
	}
	wg.Wait()

	// See if any Go routine generated an error.
	select {
	case err, ok := <-errCh:
		if ok {
			return nil, nil, fmt.Errorf("Failed to load trace data: %s", err)
		}
	default:
	}

	sklog.Infof("Finished loading values. Starting to load Params.")

	// Now load the params for the traces.
	traceids := []tiling.TraceId{}
	ts.mutex.Lock()
	for k := range tile.Traces {
		// Only load params for traces not already in the cache.
		if _, ok := ts.paramsCache[k]; !ok {
			traceids = append(traceids, k)
		}
	}
	ts.mutex.Unlock()

	// Break the loading of params into chunks and make those requests concurrently.
	// The params are just loaded into the paramsCache.
	tracechunks := [][]tiling.TraceId{}
	for len(traceids) > 0 {
		if len(traceids) > CHUNK_SIZE {
			tracechunks = append(tracechunks, traceids[:CHUNK_SIZE])
			traceids = traceids[CHUNK_SIZE:]
		} else {
			tracechunks = append(tracechunks, traceids)
			traceids = []tiling.TraceId{}
		}
	}

	errCh = make(chan error, len(tracechunks))
	for _, chunk := range tracechunks {
		wg.Add(1)
		go func(chunk []tiling.TraceId) {
			defer wg.Done()
			req := &traceservice.GetParamsRequest{
				Traceids: asStrings(chunk),
			}
			resp, err := ts.traceService.GetParams(ctx, req)
			if err != nil {
				errCh <- fmt.Errorf("Failed to load params: %s", err)
				return
			}
			for _, param := range resp.Params {
				ts.mutex.Lock()
				ts.paramsCache[tiling.TraceId(param.Key)] = param.Params
				ts.mutex.Unlock()
			}
		}(chunk)
	}
	wg.Wait()

	// See if any Go routine generated an error.
	select {
	case err, ok := <-errCh:
		if ok {
			return nil, nil, fmt.Errorf("Failed to load params: %s", err)
		}
	default:
	}

	// Add all params from the cache.
	ts.mutex.Lock()
	for k, tr := range tile.Traces {
		p := tr.Params()
		for pk, pv := range ts.paramsCache[k] {
			p[pk] = pv
		}
	}
	ts.mutex.Unlock()

	// Rebuild the ParamSet.
	sklog.Infof("Finished loading params. Starting to rebuild ParamSet.")
	tiling.GetParamSet(tile.Traces, tile.ParamSet)
	return tile, hash, nil
}

func asStrings(xt []tiling.TraceId) []string {
	s := make([]string, 0, len(xt))
	for _, t := range xt {
		s = append(s, string(t))
	}
	return s
}

// Close the underlying connection to the datastore.
func (ts *TsDB) Close() error {
	return ts.conn.Close()
}

// NewTraceServiceDBFromAddress is given the address of the traceService
// implementation and returns an instance of the trace.DB
// (the higher level wrapper on top of trace service).
func NewTraceServiceDBFromAddress(traceServiceAddr string, traceBuilder tiling.TraceBuilder) (DB, error) {
	if traceServiceAddr == "" {
		return nil, fmt.Errorf("Did not get address for trace services.")
	}

	conn, err := grpc.Dial(traceServiceAddr, grpc.WithInsecure(), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MAX_MESSAGE_SIZE), grpc.MaxCallSendMsgSize(MAX_MESSAGE_SIZE)))
	if err != nil {
		return nil, fmt.Errorf("Unable to connnect to trace service at %s. Got error: %s", traceServiceAddr, err)
	}

	return NewTraceServiceDB(conn, traceBuilder)
}
