package dstilestore

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
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

func commitIndexToTileNumber(cindex int) int {
	return cindex / TILE_SIZE
}

func commitIndexToTileOffset(cindex int) int {
	return cindex % TILE_SIZE
}

type OrderedParamSet struct {
	Encoded []byte `datastore:",noindex"`
}

func orderedParamSetDSKey(cindex int) *datastore.Key {
	key := ds.NewKey(ds.ORDERED_PARAMSET_GOLD)
	key.ID = int64(commitIndexToTileNumber(cindex) + 1) // ID must be > 0.
	return key
}

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
		Digests: []string{types.MISSING_DIGEST}, // 0 is the missing digest.
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
	Params    []byte `datastore:"p,noindex"`  // OrderedParamSet encoded paramtools.Params.
	TraceID   string `datastore:"id,noindex"` // The key that Gold expects.
	TileShard string `datastore:"ts"`         // shard_number + "-" + tile_number.
	Values    []byte `datastore:"v,noindex"`  // binary.Varint written (tileOffset, digestOffset) pairs, making this append only.
}

func traceKeys(traceID string, cindex int) (*datastore.Key, string, int64) {
	h := md5.New()
	fmt.Fprintf(h, "%s-%d", traceID, commitIndexToTileNumber(cindex))
	b := h.Sum(nil)
	key := fmt.Sprintf("%x", b)
	shard := binary.LittleEndian.Uint64(b) % NUM_SHARDS
	dsKey := ds.NewKey(ds.TRACE_GOLD)
	dsKey.Name = key
	return dsKey, key, int64(shard)
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

	orderedMutex         sync.Mutex
	orderedParamSetCache map[int]*paramtools.OrderedParamSet
}

type JobEntry struct {
	TraceID string
	Params  []byte
	Digest  string
}

type Job struct {
	CIndex  int
	Name    string
	Entries []*JobEntry
}

func NewJob(name string, cindex int) *Job {
	return &Job{
		CIndex:  cindex,
		Name:    name,
		Entries: []*JobEntry{},
	}
}

func NewDSTileStore(ctx context.Context, client *datastore.Client) *DSTileStore {
	ret := &DSTileStore{
		ctx:                  ctx,
		client:               client,
		jobs:                 make(chan *Job, 2000),
		numDigestsWritten:    metrics2.GetCounter("num_digests_written", nil),
		jobChanLength:        metrics2.GetInt64Metric("job_chan_length", nil),
		digestCacheSize:      metrics2.GetInt64Metric("cache_size_digests", nil),
		traceCacheSize:       metrics2.GetInt64Metric("cache_size_traces", nil),
		digestCache:          map[string]*Digests{},
		orderedParamSetCache: map[int]*paramtools.OrderedParamSet{}, // map from tile # to OrderedParamSet.
	}

	sklog.Info("starting workers")
	// Maybe create a worker per TestName?
	for i := 0; i < NUM_WORKERS; i++ {
		go ret.worker()
	}

	return ret
}

// tileShard returns the TileShard value for a given cindex.
func tileShard(cindex int, shard int64) string {
	return fmt.Sprintf("%d-%d", shard, commitIndexToTileNumber(cindex))
}

// Returns a new Trace.
func (d *DSTileStore) newTrace(cindex int, jobEntry *JobEntry, shard int64) *Trace {
	return &Trace{
		Params: jobEntry.Params,
		//TraceID:   jobEntry.TraceID,
		TileShard: tileShard(cindex, shard),
	}
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

func (d *DSTileStore) digestOffsets(cindex int, testName string, entries []*JobEntry) (map[string]int, error) {
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
	numEntries := len(job.Entries)

	for len(job.Entries) > 0 {
		offset := len(job.Entries)
		if offset > 25 {
			offset = 25
		}
		entriesSlice := job.Entries[:offset]
		job.Entries = job.Entries[offset:]
		allKeys := make([]string, 0, len(entriesSlice))
		allDsKeys := make([]*datastore.Key, 0, len(entriesSlice))
		allShards := make([]int64, 0, len(entriesSlice))
		for _, e := range entriesSlice {
			dsTraceKey, traceKey, shard := traceKeys(e.TraceID, job.CIndex)
			allKeys = append(allKeys, traceKey)
			allDsKeys = append(allDsKeys, dsTraceKey)
			allShards = append(allShards, shard)
		}

		d.client.RunInTransaction(d.ctx, func(tx *datastore.Transaction) error {
			allTraces := make([]*Trace, len(entriesSlice))
			err := tx.GetMulti(allDsKeys, allTraces)
			if err != nil {
				multiError, ok := err.(datastore.MultiError)
				if !ok {
					return fmt.Errorf("Error in GetMulti but not MutilError: %s", err)
				}
				for i, err := range multiError {
					if err == datastore.ErrNoSuchEntity {
						allTraces[i] = d.newTrace(job.CIndex, entriesSlice[i], allShards[i])
					} else if err == nil {
						// No error.
					} else {
						// Fail on any other error.
						return err
					}
					// Look up Digest offset outside of TX.
					allTraces[i].Values = append(allTraces[i].Values, intsVarEncoded(commitIndexToTileOffset(job.CIndex), digestOffsets[entriesSlice[i].Digest])...)
				}
			} else {
				for i, trace := range allTraces {
					// Look up Digest offset outside of TX.
					trace.Values = append(trace.Values, intsVarEncoded(commitIndexToTileOffset(job.CIndex), digestOffsets[entriesSlice[i].Digest])...)
				}
			}
			if _, err := tx.PutMulti(allDsKeys, allTraces); err != nil {
				return fmt.Errorf("Failed to write traces: %s", err)
			}
			return nil
		}, datastore.MaxAttempts(MAX_ATTEMPTS))
	}
	d.numDigestsWritten.Inc(int64(numEntries))
	d.jobChanLength.Update(int64(len(d.jobs)))
	return err
}

func intsVarEncoded(ints ...int) []byte {
	ret := []byte{}
	buf := make([]byte, binary.MaxVarintLen64)
	for _, x := range ints {
		n := binary.PutVarint(buf, int64(x))
		ret = append(ret, buf[:n]...)
	}
	return ret
}

func ingestionEntryToJobEntry(o *paramtools.OrderedParamSet, p *types.ParsedIngestionEntry) (*JobEntry, error) {
	encoded, err := o.EncodeParams(p.Params)
	if err != nil {
		return nil, err
	}
	return &JobEntry{
		TraceID: p.TraceID,
		Digest:  p.Digest,
		Params:  encoded,
	}, nil
}

func (d *DSTileStore) Add(cindex int, entries []*types.ParsedIngestionEntry) error {
	paramset := paramtools.ParamSet{}
	for _, e := range entries {
		paramset.AddParams(e.Params)
	}

	ops, err := d.getOrderedParamSet(cindex, paramset)
	if err != nil {
		return fmt.Errorf("Failed to get ParamSet: %s", err)
	}

	// Convert each entry into one that uses encoded params.
	// Group by test name.
	jobs := map[string]*Job{}
	for _, e := range entries {
		je, err := ingestionEntryToJobEntry(ops, e)
		if err != nil {
			return fmt.Errorf("Failed to convert ParsedIngestionEntry to a JobEntry: %s", err)
		}
		testName := e.Params["name"]
		if job, ok := jobs[testName]; !ok {
			job = NewJob(testName, cindex)
			job.Entries = append(job.Entries, je)
			jobs[testName] = job
		} else {
			job.Entries = append(job.Entries, je)
		}
	}
	sklog.Infof("%d entries turned into %d jobs", len(entries), len(jobs))
	for _, job := range jobs {
		d.jobs <- job
	}
	return nil
}

func (d *DSTileStore) getOrderedParamSet(cindex int, paramset paramtools.ParamSet) (*paramtools.OrderedParamSet, error) {
	d.orderedMutex.Lock()
	defer d.orderedMutex.Unlock()
	tileNumber := commitIndexToTileNumber(cindex)
	var missing paramtools.ParamSet
	o, ok := d.orderedParamSetCache[tileNumber]
	if ok {
		missing = o.Check(paramset)
	} else {
		o = paramtools.NewOrderedParamSet()
		missing = paramset
	}
	if len(missing) > 0 {
		key := orderedParamSetDSKey(cindex)
		_, err := d.client.RunInTransaction(d.ctx, func(tx *datastore.Transaction) error {
			var oDS OrderedParamSet
			if err := tx.Get(key, &oDS); err != nil {
				if err == datastore.ErrNoSuchEntity {
					o = paramtools.NewOrderedParamSet()
				} else {
					return err
				}
			} else {
				o, err = paramtools.NewOrderedParamSetFromBytes(oDS.Encoded)
				if err != nil {
					return fmt.Errorf("Failed to decode stored OrderedParamSet: %s", err)
				}
			}
			o.Update(paramset)
			var err error
			oDS.Encoded, err = o.Encode()
			if err != nil {
				return err
			}
			if _, err := tx.Put(key, &oDS); err != nil {
				return err
			}
			d.orderedParamSetCache[tileNumber] = o
			return nil
		}, datastore.MaxAttempts(MAX_ATTEMPTS))
		if err != nil {
			return nil, fmt.Errorf("Failed to update OrderedParamSet: %s", err)
		}
	}
	// Need to make a deep copy of o.
	b, err := o.Encode()
	if err != nil {
		return nil, fmt.Errorf("Can't encode OrderedParamSet: %s", err)
	}
	return paramtools.NewOrderedParamSetFromBytes(b)
}
