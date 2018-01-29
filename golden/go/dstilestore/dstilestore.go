package dstilestore

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/dsconst"
	"go.skia.org/infra/golden/go/types"
)

const (
	TILE_SIZE = 50

	NUM_WORKERS = 32
)

type Trace struct {
	Digests []string `datastore:"digests,noindex"`
	Trace   []int    `datastore:"trace,noindex"`
	Params  string   `datastore:"params,noindex"` // JSON encoded.
	Tile    int64
}

func NewTrace(cindex int, params map[string]string) (*Trace, error) {
	b, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return &Trace{
		Digests: []string{},
		Trace:   make([]int, TILE_SIZE),
		Params:  string(b),
		Tile:    int64(cindex / TILE_SIZE),
	}, nil
}

func (t *Trace) Add(digest string, index int) {
	index = index % TILE_SIZE
	var dindex int
	if dindex = util.Index(digest, t.Digests); dindex == -1 {
		t.Digests = append(t.Digests, digest)
		dindex = len(t.Digests) - 1
	}
	if len(t.Trace) == 0 {
		t.Trace = make([]int, TILE_SIZE)
	}
	t.Trace[index] = dindex + 1
}

func (t *Trace) AsGoldenTrace(key string) *types.GoldenTrace {
	ret := &types.GoldenTrace{
		Params_: paramtools.NewParams(key),
		Values:  make([]string, TILE_SIZE),
	}
	for i, dindex := range t.Trace {
		if dindex > 0 {
			ret.Values[i] = t.Digests[dindex-1]
		}
	}
	return ret
}

type DSTileStore struct {
	ctx    context.Context
	client *datastore.Client
	jobs   chan *Job
}

type Job struct {
	CIndex int
	Key    string
	Entry  *types.ParsedIngestionEntry
}

func NewDSTileStore(ctx context.Context, client *datastore.Client) *DSTileStore {
	ret := &DSTileStore{
		ctx:    ctx,
		client: client,
		jobs:   make(chan *Job, 1000),
	}

	// Maybe create a worker per TestName?
	for i := 0; i < NUM_WORKERS; i++ {
		go ret.worker()
	}

	return ret
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

func (d *DSTileStore) addInTx(job *Job) error {
	tx, err := d.client.NewTransaction(d.ctx)
	if err != nil {
		return err
	}

	var trace Trace
	key := traceKey(job.CIndex, job.Key)
	if err := tx.Get(key, &trace); err != nil {
		if err == datastore.ErrNoSuchEntity {
			tr, err := NewTrace(job.CIndex, job.Entry.Params)
			if err != nil {
				return fmt.Errorf("Failed to create new trace: %s", err)
			}
			trace = *tr
		} else if err != nil {
			if err := tx.Rollback(); err != nil {
				sklog.Errorf("Failed to Rollback Get: %s", err)
			}
			return fmt.Errorf("Failed Get: %s", err)
		}
	}
	trace.Add(job.Entry.Digest, job.CIndex)
	if _, err := tx.Put(key, &trace); err != nil {
		if rberr := tx.Rollback(); rberr != nil {
			sklog.Errorf("Failed to Rollback PutMulti: %s", rberr)
		}
		return err
	}
	if _, err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (d *DSTileStore) Add(cindex int, entries map[string]*types.ParsedIngestionEntry) error {
	for key, entry := range entries {
		sklog.Infof("Job: %s - %#v", key, entry)
		d.jobs <- &Job{
			CIndex: cindex,
			Key:    key,
			Entry:  entry,
		}
	}
	return nil
}

func traceKey(cindex int, traceId string) *datastore.Key {
	key := ds.NewKey(dsconst.TRACE)
	key.Name = fmt.Sprintf("%s-%d", traceId, int64(cindex/TILE_SIZE))
	return key
}
