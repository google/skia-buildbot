package dstilestore

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

/*
Store all the digests for the Tile in a single entity.
Write new values batched by test name.
Only process one file at a time.
*/
const (
	TILE_SIZE = 50

	NUM_WORKERS = 1024

	// NUM_SHARDS is the number of shards that the Trace data is sharded over.
	// NUM_SHARDS must never be decreased.
	NUM_SHARDS = 32

	// MAX_ATTEMPTS to retry a transaction that fails on concurrency.
	MAX_ATTEMPTS = 10

	LRU_CACHE_SIZE = 1024 * 1024
)

// Digests is used to store all the digests seen for a given test.
// Key is test_name + "-" + tile_number.
// Kind is ds.DIGESTS_GOLD
type Digests struct {
	Digests []string `datastore:",noindex"`
}

func (d *Digests) Add(digest string) int {
	if index := util.Index(digest, d.Digests); index == -1 {
		d.Digests = append(d.Digests, digest)
		return len(d.Digests) - 1
	} else {
		return index
	}
}

func NewDigests() *Digests {
	return &Digests{
		Digests: []string{types.MISSING_DIGEST},
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

// Trace is used to store a TILE_SIZE commits slice of offsets into the appropriate Digests.
// Key is md5(structured key) + "-" + tile_number
// Kind is ds.TRACE_GOLD
type Trace struct {
	Offsets   []int  `datastore:",noindex"` // Each value is an offset into Digests.Digests.
	TileShard string // tile_number + "-" shard_number.
}

// An individual value, where the key is the tile offset. Has a Trace as its ancestor.
type TraceValue struct {
	Offset int
}

func traceValueKey(tileOffset int, parent *datastore.Key) *datastore.Key {
	ret := ds.NewKey(ds.TRACE_VALUE_GOLD)
	ret.ID = int64(tileOffset + 1) // TODO(jcgregorio) REMEMBER TO SUB 1 WHEN READING.
	ret.Parent = parent
	return ret
}

/*
func (t *Trace) Add(tileOffset, digestOffset int) {
	t.Offsets[tileOffset] = digestOffset
}
*/

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
	ctx               context.Context
	client            *datastore.Client
	jobs              chan *Job
	numDigestsWritten metrics2.Counter
	jobChanLength     metrics2.Int64Metric
	digestCacheSize   metrics2.Int64Metric
	traceCacheSize    metrics2.Int64Metric

	mutex       sync.RWMutex
	digestCache map[string]*Digests

	traceMutex sync.RWMutex
	tracesSeen map[string]bool
}

type Job struct {
	CIndex  int
	Name    string
	Entries []*types.ParsedIngestionEntry
}

func NewJob(name string, cindex int) *Job {
	return &Job{
		CIndex:  cindex,
		Name:    name,
		Entries: []*types.ParsedIngestionEntry{},
	}
}

func NewDSTileStore(ctx context.Context, client *datastore.Client) *DSTileStore {
	ret := &DSTileStore{
		ctx:               ctx,
		client:            client,
		jobs:              make(chan *Job, 2000),
		numDigestsWritten: metrics2.GetCounter("num_digests_written", nil),
		jobChanLength:     metrics2.GetInt64Metric("job_chan_length", nil),
		digestCacheSize:   metrics2.GetInt64Metric("cache_size_digests", nil),
		traceCacheSize:    metrics2.GetInt64Metric("cache_size_traces", nil),

		digestCache: map[string]*Digests{},

		// TODO(jcgregorio) If ingesting pre-populate with values from datastore.
		tracesSeen: map[string]bool{},
	}

	sklog.Info("starting workers")
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
			TileShard: ts,
		}, &Params{
			Keys:      string(k),
			Options:   string(o),
			TraceID:   traceID,
			TileShard: ts,
		}, nil
}

func (d *DSTileStore) worker() {
	sklog.Info("starting worker")
	for e := range d.jobs {
		err := d.addInTx(e)
		if err != nil {
			sklog.Errorf("Failed to add to datastore: %s", err)
		}
	}
}

// TODO Find the indices for a slice of digests, adding to the datastore
// if not found.
func (d *DSTileStore) digestOffsets(cindex int, testName string, entries []*types.ParsedIngestionEntry) (map[string]int, error) {
	offsets := map[string]int{}
	missingDigests := map[string]bool{}

	dcKey := digestKey(cindex, testName)
	d.mutex.RLock()
	digests, ok := d.digestCache[dcKey]
	d.mutex.RUnlock()
	if !ok {
		// No cache entry, so presume everything is missing.
		for _, e := range entries {
			missingDigests[e.Digest] = true
		}
	} else {
		for _, e := range entries {
			if index := util.Index(e.Digest, digests.Digests); index == -1 {
				missingDigests[e.Digest] = true
			} else {
				offsets[e.Digest] = index
			}
		}
		if len(missingDigests) == 0 {
			return offsets, nil
		}
	}

	var offsetsFromTx map[string]int
	if len(missingDigests) > 0 {
		// Add all missing digests to datastore.
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
			offsetsFromTx = map[string]int{}
			// This TX could fail, so only store the offsets to process after we know
			// the TX has succeeded.
			for d, _ := range missingDigests {
				offsetsFromTx[d] = dig.Add(d)
			}
			// Add the digest to the list.
			if _, err := tx.Put(key, &dig); err != nil {
				return fmt.Errorf("Failed to write Digests: %s", err)
			}
			// Add digests to cache.
			d.mutex.Lock()
			defer d.mutex.Unlock()
			d.digestCache[dcKey] = &dig
			d.digestCacheSize.Update(int64(len(d.digestCache)))
			return nil
		}, datastore.MaxAttempts(MAX_ATTEMPTS))
		if err != nil {
			return nil, fmt.Errorf("Failed to write new digests: %s", err)
		}
	}
	for k, v := range offsetsFromTx {
		offsets[k] = v
	}

	return offsets, nil
}

func (d *DSTileStore) addInTx(job *Job) error {
	if job.Name == "" {
		return fmt.Errorf("Ingested data doesn't contain a valid test name: %s", job.Name)
	}
	// For all the new digests in the job, find out the offset of those digests
	// in the list of all digests seen for a test+tile.
	digestOffsets, err := d.digestOffsets(job.CIndex, job.Name, job.Entries)
	if err != nil {
		return fmt.Errorf("Failed to write new digest for test %s: %s", job.Name, err)
	}

	valueKeys := []*datastore.Key{}
	values := []*TraceValue{}

	for _, e := range job.Entries {
		traceKey := traceDSKey(e.TraceID, job.CIndex)

		d.traceMutex.RLock()
		_, ok := d.tracesSeen[traceKey.Name]
		d.traceMutex.RUnlock()
		if !ok {
			var trace Trace
			if err := d.client.Get(d.ctx, traceKey, &trace); err != nil {
				// If it doesn't exist, then populate it.
				if err == datastore.ErrNoSuchEntity {
					var pm *Params
					var tr *Trace
					tr, pm, err = d.newTrace(job.CIndex, e.Keys, e.Options, e.TraceID)
					if err != nil {
						sklog.Errorf("Failed to create new trace: %s", err)
						continue
					}
					trace = *tr
					// Write the Params associated with the new Trace.
					paramsKey := ds.NewKey(ds.PARAMS_GOLD)
					paramsKey.Name = traceKey.Name
					if _, err := d.client.Put(d.ctx, paramsKey, pm); err != nil {
						sklog.Errorf("Failed Params Put: %s", err)
						continue
					}
					d.traceMutex.Lock()
					d.tracesSeen[traceKey.Name] = true
					d.traceCacheSize.Update(int64(len(d.tracesSeen)))
					d.traceMutex.Unlock()
				} else if err != nil {
					sklog.Errorf("Failed Get: %s", err)
					continue
				}
			} else {
				d.traceMutex.Lock()
				d.tracesSeen[traceKey.Name] = true
				d.traceCacheSize.Update(int64(len(d.tracesSeen)))
				d.traceMutex.Unlock()
			}
		}

		valueKeys = append(valueKeys, traceValueKey(commitIndexToTileOffset(job.CIndex), traceKey))
		values = append(values, &TraceValue{
			Offset: digestOffsets[e.Digest],
		})
	}
	// Write traceValue back to datastore.
	// So do this in a tx?
	if _, err := d.client.PutMulti(d.ctx, valueKeys, values); err != nil {
		// TODO(jcgregorio) Should handle errors that may happen to individual Put's and retry them
		// if appropriate.
		sklog.Errorf("Error writing TraceValues: %s", err)
	}
	d.numDigestsWritten.Inc(int64(len(job.Entries)))
	d.jobChanLength.Update(int64(len(d.jobs)))
	return err
}

func (d *DSTileStore) Add(cindex int, entries []*types.ParsedIngestionEntry) error {
	// Group by test name.
	sklog.Infof("got parsed entry to add")
	jobs := map[string]*Job{}
	for _, e := range entries {
		testName := e.Keys["name"]
		if job, ok := jobs[testName]; !ok {
			job = NewJob(testName, cindex)
			job.Entries = append(job.Entries, e)
			jobs[testName] = job
		} else {
			job.Entries = append(job.Entries, e)
		}
	}
	sklog.Infof("%d entries turned into %d jobs", len(entries), len(jobs))
	for _, job := range jobs {
		d.jobs <- job
	}
	return nil
}
