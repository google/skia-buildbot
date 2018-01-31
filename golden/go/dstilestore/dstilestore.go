package dstilestore

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"

	"cloud.google.com/go/datastore"
	"github.com/golang/groupcache/lru"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

const (
	TILE_SIZE = 50

	NUM_WORKERS = 256

	// NUM_SHARDS is the number of shards that the Trace data is sharded over.
	// NUM_SHARDS must never be decreased.
	NUM_SHARDS = 32

	// MAX_ATTEMPTS to retry a transaction that fails on concurrency.
	MAX_ATTEMPTS = 10

	LRU_TRACE_CACHE_SIZE = 1024 * 1024
)

// Digests is used to store all the digests seen for a given test.
// Key is test_name + "-" + tile_number.
// Kind is ds.DIGESTS_GOLD
type Digests struct {
	Digests []string `datastore:",noindex"`
}

func (d *Digests) Add(digest string) int {
	if !util.In(digest, d.Digests) {
		d.Digests = append(d.Digests, digest)
	}
	return len(d.Digests) - 1
}

func NewDigests() *Digests {
	return &Digests{
		Digests: []string{types.MISSING_DIGEST},
	}
}

// Trace is used to store a TILE_SIZE commits slice of offsets into the appropriate Digests.
// Key is md5(structured key) + "-" + tile_number
// Kind is ds.TRACE_GOLD
type Trace struct {
	Offsets   []int  `datastore:",noindex"` // Each value is an offset into Digests.Digests.
	TileShard string // tile_number + "-" shard_number.
}

func (t *Trace) Add(tileOffset, digestIndex int) {
	t.Offsets[tileOffset] = digestIndex
}

type TraceCacheEntry struct {
	mutex sync.Mutex
	trace *Trace
}

// Params is used to store the Params for a given trace.
// Key is md5(structured key) + "-" + tile_number
// Kind is ds.PARAMS_GOLD
type Params struct {
	Keys      string `datastore:",noindex"` // JSON encoded paramtools.Params.
	Options   string `datastore:",noindex"` // JSON encoded paramtools.Params.
	TraceID   string `datastore:",noindex"` // The key that Gold expects.
	TileShard string // tile_number + "-" shard_number. Must match the value of the TileShard in Trace for the same key.
}

func commitIndexToTileNumber(cindex int) int {
	return cindex / TILE_SIZE
}

func commitIndexToTileOffset(cindex int) int {
	return cindex % TILE_SIZE
}

func traceDSKey(traceID string, cindex int) *datastore.Key {
	key := ds.NewKey(ds.TRACE_GOLD)
	key.Name = traceKey(traceID, cindex)
	return key
}

func traceKey(traceID string, cindex int) string {
	h := md5.New()
	fmt.Fprintf(h, "%s-%d", traceID, commitIndexToTileNumber(cindex))
	return fmt.Sprintf("%x", h.Sum(nil))
}

type DSTileStore struct {
	ctx                   context.Context
	client                *datastore.Client
	jobs                  chan *Job
	numDigestsWritten     metrics2.Counter
	numDigestsCacheMisses metrics2.Counter
	jobChanLength         metrics2.Int64Metric
	lruCacheSize          metrics2.Int64Metric

	mutex        sync.Mutex          // Protects access to digestsCache.
	digestsCache map[string]*Digests // Map from test name to all Digests seen for that test across all traces.
	traceCache   *lru.Cache          // Stores TraceCacheEntry's.
}

type Job struct {
	CIndex int
	Entry  *types.ParsedIngestionEntry
}

func NewDSTileStore(ctx context.Context, client *datastore.Client) *DSTileStore {
	ret := &DSTileStore{
		ctx:                   ctx,
		client:                client,
		jobs:                  make(chan *Job, 2000),
		numDigestsWritten:     metrics2.GetCounter("num_digests_written", nil),
		numDigestsCacheMisses: metrics2.GetCounter("num_digests_cache_misses", nil),
		jobChanLength:         metrics2.GetInt64Metric("job_chan_length", nil),
		lruCacheSize:          metrics2.GetInt64Metric("lru_cache_size", nil),

		digestsCache: map[string]*Digests{},
		traceCache:   lru.New(LRU_TRACE_CACHE_SIZE),
	}

	// Maybe create a worker per TestName?
	for i := 0; i < NUM_WORKERS; i++ {
		go ret.worker()
	}

	return ret
}

// tileShard returns the TileShard value for a given cindex.
func (d *DSTileStore) tileShard(cindex int) string {
	return fmt.Sprintf("%d-%d", rand.Int63n(NUM_SHARDS), commitIndexToTileNumber(cindex))
}

// Returns a new Trace and Params.
func (d *DSTileStore) newTrace(cindex int, keys, options paramtools.Params, traceID string) (*Trace, *Params, error) {
	k, err := json.Marshal(keys)
	if err != nil {
		return nil, nil, err
	}
	o, err := json.Marshal(options)
	if err != nil {
		return nil, nil, err
	}
	ts := d.tileShard(cindex)
	return &Trace{
			Offsets:   make([]int, TILE_SIZE),
			TileShard: ts,
		}, &Params{
			Keys:      string(k),
			Options:   string(o),
			TraceID:   traceID,
			TileShard: ts,
		}, nil
}

func (d *DSTileStore) worker() {
	for e := range d.jobs {
		err := d.addInTx(e)
		for err == datastore.ErrConcurrentTransaction {
			err = d.addInTx(e)
		}
		if err != nil {
			sklog.Errorf("Failed to add to datastore: %s", err)
		}
	}
}

func digestKey(cindex int, testName string) string {
	return fmt.Sprintf("%s-%d", testName, commitIndexToTileNumber(cindex))
}

func digestDSKey(cindex int, testName string) *datastore.Key {
	key := ds.NewKey(ds.DIGESTS_GOLD)
	key.Name = digestKey(cindex, testName)
	return key
}

func (d *DSTileStore) digestIndex(cindex int, testName, digest string) int {
	var offset int = -1
	d.mutex.Lock()
	defer d.mutex.Unlock()
	cached := d.digestsCache[digestKey(cindex, testName)]
	if cached != nil {
		offset = util.Index(digest, cached.Digests)
	}
	return offset
}

func (d *DSTileStore) addDigest(cindex int, digest, testName string) (int, error) {
	index := -1
	_, err := d.client.RunInTransaction(d.ctx, func(tx *datastore.Transaction) error {
		// Load the Digests
		var dig Digests
		key := digestDSKey(cindex, testName)
		if err := tx.Get(key, &dig); err != nil {
			if err == datastore.ErrNoSuchEntity {
				// If none exists, then create a new one.
				dig = *NewDigests()
			} else {
				return err
			}
		}
		// Add the digest to the list.
		index = dig.Add(digest)
		if _, err := tx.Put(key, &dig); err == nil {
			d.mutex.Lock()
			defer d.mutex.Unlock()
			d.digestsCache[key.Name] = &dig
		}
		return nil
	}, datastore.MaxAttempts(MAX_ATTEMPTS))
	return index, err
}

func (d *DSTileStore) addInTx(job *Job) error {
	testName := job.Entry.Keys["name"]
	if testName == "" {
		return fmt.Errorf("Ingested data doesn't contain a valid test name: %s", job.Entry.TraceID)
	}
	// Does the Digest appear in the cache of Digests?
	// If not then add the new Digest in a TX.
	digestIndex := d.digestIndex(job.CIndex, testName, job.Entry.Digest)
	if digestIndex == -1 {
		d.numDigestsCacheMisses.Inc(1)
		var err error
		if digestIndex, err = d.addDigest(job.CIndex, job.Entry.Digest, testName); err != nil {
			return fmt.Errorf("Failed to write new digest for test %s %s: %s", job.Entry.Digest, testName, err)
		}
	}

	_, err := d.client.RunInTransaction(d.ctx, func(tx *datastore.Transaction) error {
		// Load the Trace
		key := traceDSKey(job.Entry.TraceID, job.CIndex)

		tce, ok := d.traceFromCache(key.Name)
		defer tce.mutex.Unlock()
		if !ok {
			var trace Trace
			if err := tx.Get(key, &trace); err != nil {
				// If it doesn't exist, then populate it.
				if err == datastore.ErrNoSuchEntity {
					tr, pm, err := d.newTrace(job.CIndex, job.Entry.Keys, job.Entry.Options, job.Entry.TraceID)
					if err != nil {
						return fmt.Errorf("Failed to create new trace: %s", err)
					}
					tce.trace = tr
					// Write the Params associated with the new Trace.
					paramsKey := ds.NewKey(ds.PARAMS_GOLD)
					paramsKey.Name = key.Name
					if _, err := tx.Put(paramsKey, pm); err != nil {
						if err := tx.Rollback(); err != nil {
							sklog.Errorf("Failed to Rollback Put of Params: %s", err)
						}
						return fmt.Errorf("Failed Params Put: %s", err)
					}
				} else if err != nil {
					if err := tx.Rollback(); err != nil {
						sklog.Errorf("Failed to Rollback Get: %s", err)
					}
					return fmt.Errorf("Failed Get: %s", err)
				}
			} else {
				tce.trace = &trace
			}
		}
		// Add new value to trace.
		tce.trace.Add(commitIndexToTileOffset(job.CIndex), digestIndex)
		// Write trace back to datastore.
		if _, err := tx.Put(key, tce.trace); err != nil {
			if rberr := tx.Rollback(); rberr != nil {
				sklog.Errorf("Failed to Rollback PutMulti: %s", rberr)
			}
			return err
		}
		d.numDigestsWritten.Inc(1)
		d.traceToCache(key.Name, tce)
		d.jobChanLength.Update(int64(len(d.jobs)))
		d.lruCacheSize.Update(int64(d.traceCache.Len()))
		return nil
	}, datastore.MaxAttempts(MAX_ATTEMPTS))
	return err
}

func (d *DSTileStore) traceToCache(key string, tce *TraceCacheEntry) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.traceCache.Add(key, tce)
}

func (d *DSTileStore) traceFromCache(key string) (*TraceCacheEntry, bool) {
	// Once we get a trace from the lruCache we need to lock it so that no-one else
	// can modify it while we do an update.

	// But, we need to release it if our write to the datastore fails.
	d.mutex.Lock()
	defer d.mutex.Unlock()
	var ok bool
	var ret *TraceCacheEntry
	var tceIntf interface{}
	tceIntf, ok = d.traceCache.Get(key)
	if !ok {
		ret = &TraceCacheEntry{}
		d.traceCache.Add(key, ret)
	} else {
		ret, ok = tceIntf.(*TraceCacheEntry)
		if !ok {
			sklog.Errorf("Found wrong type in LRU cache?: %s", key)
			ret = &TraceCacheEntry{}
			d.traceCache.Add(key, ret)
		}
	}
	ret.mutex.Lock()
	return ret, ok
}

func (d *DSTileStore) Add(cindex int, entries []*types.ParsedIngestionEntry) error {
	for _, entry := range entries {
		d.jobs <- &Job{
			CIndex: cindex,
			Entry:  entry,
		}
	}
	return nil
}
