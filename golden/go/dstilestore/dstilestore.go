package dstilestore

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/api/iterator"
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
	key.ID = commitIndexToTileNumber(cindex) + 1 // ID must be > 0.
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
	return dsKey, key, shard
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

	orderedMutex     sync.Mutex
	orderedParamSets map[int]*paramtools.OrderedParamSet

	traceMutex sync.RWMutex
	tracesSeen map[string]bool
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
		Entries: []*types.ParsedIngestionEntry{},
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
		orderedParamSetCache: map[int]paramtools.OrderedParamSet{}, // map from tile # to OrderedParamSet.
		tracesSeen:           map[string]bool{},
	}

	sklog.Info("starting workers")
	// Maybe create a worker per TestName?
	for i := 0; i < NUM_WORKERS; i++ {
		go ret.worker()
	}

	return ret
}

func (d *DSTileStore) PreFillCache(cindex int) {
	sklog.Infof("cindex: %d", cindex)
	var wg sync.WaitGroup
	for i := 0; i < NUM_SHARDS; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			tileShard := fmt.Sprintf("%d-%d", index, commitIndexToTileNumber(cindex))
			sklog.Infof("About to prefill cache: %s", tileShard)
			q := ds.NewQuery(ds.TRACE_GOLD).Filter("TileShard=", tileShard).KeysOnly()
			for t := d.client.Run(d.ctx, q); ; {
				var ii interface{}
				key, err := t.Next(&ii)
				if err == iterator.Done {
					break
				}
				if err != nil {
					sklog.Errorf("Fail reading: %s", err)
					continue
				} else {
					d.mutex.Lock()
					d.tracesSeen[key.Name] = true
					d.mutex.Unlock()
				}
			}
		}(i)
	}
	wg.Wait()
	d.traceCacheSize.Update(int64(len(d.tracesSeen)))
}

// tileShard returns the TileShard value for a given cindex.
func tileShard(cindex int, shard int64) string {
	return fmt.Sprintf("%d-%d", shard, commitIndexToTileNumber(cindex))
}

// Returns a new Trace.
func (d *DSTileStore) newTrace(cindex int, keys, options paramtools.Params, traceID string, shard int64) (*Trace, error) {
	k, err := json.Marshal(keys)
	if err != nil {
		return nil, err
	}
	o, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}
	return &Trace{
		Keys:      string(k),
		Options:   string(o),
		TraceID:   traceID,
		TileShard: tileShard(cindex, shard),
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
		dsTraceKey, traceKey, shard := traceKeys(e.TraceID, job.CIndex)

		d.traceMutex.RLock()
		_, ok := d.tracesSeen[traceKey]
		d.traceMutex.RUnlock()
		if !ok {
			var trace Trace
			if err := d.client.Get(d.ctx, dsTraceKey, &trace); err != nil {
				// If it doesn't exist, then populate it.
				if err == datastore.ErrNoSuchEntity {
					var tr *Trace
					tr, err = d.newTrace(job.CIndex, e.Keys, e.Options, e.TraceID, shard)
					if err != nil {
						sklog.Errorf("Failed to create new trace: %s", err)
						continue
					}
					trace = *tr
					// Write the Params associated with the new Trace.
					if _, err := d.client.Put(d.ctx, dsTraceKey, &trace); err != nil {
						sklog.Errorf("Failed Trace Put: %s", err)
						continue
					}
					d.traceMutex.Lock()
					d.tracesSeen[traceKey] = true
					d.traceCacheSize.Update(int64(len(d.tracesSeen)))
					d.traceMutex.Unlock()
				} else if err != nil {
					sklog.Errorf("Failed Get: %s", err)
					continue
				}
			} else {
				d.traceMutex.Lock()
				d.tracesSeen[traceKey] = true
				d.traceCacheSize.Update(int64(len(d.tracesSeen)))
				d.traceMutex.Unlock()
			}
		}

		valueKeys = append(valueKeys, traceValueKey(commitIndexToTileOffset(job.CIndex), dsTraceKey))
		values = append(values, &TraceValue{
			Offset: digestOffsets[e.Digest],
		})
	}
	// Write traceValue back to datastore.

	// Break this into slices of no more than 25 keys at a time?
	for len(valueKeys) > 0 {
		offset := len(valueKeys)
		if offset > 25 {
			offset = 25
		}
		someValueKeys := valueKeys[:offset]
		valueKeys = valueKeys[offset:]
		someValues := values[:offset]
		values = values[offset:]
		_, err = d.client.RunInTransaction(d.ctx, func(tx *datastore.Transaction) error {
			if _, err := tx.PutMulti(someValueKeys, someValues); err != nil {
				// TODO(jcgregorio) Should handle errors that may happen to individual Put's and retry them
				// if appropriate?
				sklog.Errorf("Error writing TraceValues: %s", err)
				return err
			}
			return nil
		}, datastore.MaxAttempts(MAX_ATTEMPTS))
		if err != nil {
			sklog.Errorf("Failed TX to write TraceValues: %s", err)
		}
	}
	d.numDigestsWritten.Inc(int64(len(job.Entries)))
	d.jobChanLength.Update(int64(len(d.jobs)))
	return err
}

func ingestionEntryToJobEntry(o *paramtools.OrderedParamSet, p *types.ParsedIngestionEntry) (*JobEntry, error) {
	encoded, err := o.EncodeParams(p)
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
	sklog.Infof("got parsed entry to add")
	jobs := map[string]*Job{}
	for _, e := range entries {
		je, err := ingestionEntryToJobEntry(ops, e)
		if err != nil {
			return fmt.Errorf("Failed to convert ParsedIngestionEntry to a JobEntry: %s", err)
		}
		testName := e.Keys["name"]
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

func paramsetDSKey(cindex int) *datastore.Key {
	key := ds.NewKey(ds.PARAMSET_GOLD)
	key.ID = int64(commitIndexToTileNumber(cindex))
	return key
}

func (d *DSTileStore) getOrderedParamSet(cindex int, paramset paramtools.ParamSet) (*paramtools.OrderedParamSet, error) {
	d.orderedMutex.Lock()
	defer d.orderedMutex.Unlock()
	tileNumber := commitIndexToTileNumber(cindex)
	var missing paramtools.ParamSet
	o, ok := d.orderedParamSets[tileNumber]
	if ok {
		missing = o.Check(paramset)
	} else {
		o = paramtools.NewOrderedParamSet()
		missing = paramset
	}
	if len(missing) > 0 {
		key := orderedParamSetDSKey(cindex)
		err := d.client.RunInTransaction(d.ctx, func(tx *datastore.Transaction) error {
			var oDS OrderedParamSet
			if err := tx.Get(key, &oDS); err != nil {
				if err == datastore.ErrNoSuchEntity {
					o = paramtools.NewOrderedParamSet()
				} else {
					return err
				}
			}
			o, err = paramtools.NewOrderedParamSetFromBytes(oDS.Encoded)
			if err != nil {
				return fmt.Errorf("Failed to decode stored OrderedParamSet: %s", err)
			}
			o.Update(paramset)
			oDS.Encoded = o.Encode()
			if err := tx.Put(key, &oDS); err != nil {
				return err
			}
			d.orderedParamSets[tileNumber] = o
		}, datastore.MaxAttempts(MAX_ATTEMPTS))
		if err != nil {
			return nil, fmt.Errorf("Failed to update OrderedParamSet: %s", err)
		}
	}
	// Need to make a deep copy of o.
	ret := paramtools.NewOrderedParamSetFromBytes(o.Encode())
	return ret, nil
}
