package dstilestore

import (
	"context"
	"fmt"
	"sync"

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

// These struct are what is stored in Cloud Datastore.
type Tile struct{}
type TestName struct{}
type Trace struct {
	Digests []string `datastore:"digests,noindex"`
	Trace   []int    `datastore:"trace,noindex"`
	Options paramtools.Params
}

func NewTrace() *Trace {
	return &Trace{
		Digests: []string{},
		Trace:   make([]int, TILE_SIZE),
	}
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

	mutex         sync.Mutex
	seenTile      map[string]bool
	seenTestNames map[string]bool
}

type Job struct {
	CIndex  int
	Name    string
	Entries map[string]*types.ParsedIngestionEntry
}

func NewDSTileStore(ctx context.Context, client *datastore.Client) *DSTileStore {
	ret := &DSTileStore{
		ctx:           ctx,
		client:        client,
		jobs:          make(chan *Job, 1000),
		seenTile:      map[string]bool{},
		seenTestNames: map[string]bool{},
	}

	// Maybe create a worker per TestName?
	for i := 0; i < NUM_WORKERS; i++ {
		go ret.worker()
	}

	return ret
}

func (d *DSTileStore) worker() {
	for e := range d.jobs {
		d.addInTx(e)
	}
}

func (d *DSTileStore) addInTx(job *Job) error {
	tx, err := d.client.NewTransaction(d.ctx)
	if err != nil {
		return err
	}

	keys := make([]*datastore.Key, 0, len(job.Entries))
	for key, _ := range job.Entries {
		keys = append(keys, traceKey(job.CIndex, job.Name, key))
	}
	traces := make([]*Trace, len(keys))
	if err := tx.GetMulti(keys, traces); err != nil {
		// In a GetMulti the ErrNoSuchEntity is hidden within the MultiError.
		if me, ok := err.(datastore.MultiError); !ok {
			if err := tx.Rollback(); err != nil {
				sklog.Errorf("Failed to Rollback GetMulti: %s", err)
			}
			return fmt.Errorf("Failed GetMulti: %s", err)
		} else {
			for i, err := range me {
				if err == datastore.ErrNoSuchEntity {
					traces[i] = NewTrace()
				} else if err != nil {
					return fmt.Errorf("Failed GetMulti: %s", err)
				}
			}
		}
	}
	for i, trace := range traces {
		key := keys[i].Name
		trace.Add(job.Entries[key].Digest, job.CIndex)
	}
	if _, multierr := tx.PutMulti(keys, traces); multierr != nil {
		if err := tx.Rollback(); err != nil {
			sklog.Errorf("Failed to Rollback PutMulti: %s", err)
		}
		return fmt.Errorf("Failed PutMulti: %s", err)
	}

	return nil
}

func (d *DSTileStore) Add(cindex int, entries map[string]*types.ParsedIngestionEntry) error {
	// Create Tile if necessary.
	if err := d.addTileAsNeeded(cindex); err != nil {
		return fmt.Errorf("Unable to add Tile to datastore: %s", err)
	}
	// Create TestName's as necessary.
	names := map[string]bool{}
	for _, entry := range entries {
		names[entry.Name] = true
	}
	if err := d.addTestNamesAsNeeded(cindex, names); err != nil {
		return fmt.Errorf("Unable to add TestNames to datastore: %s", err)
	}

	// Group entries by 'name'.
	byName := map[string]map[string]*types.ParsedIngestionEntry{}
	for key, entry := range entries {
		m := byName[entry.Name]
		m[key] = entry
		byName[entry.Name] = m
	}

	// Pass each group down to a go routine pool to be added.
	// Each routine in the pool, in a tx, loads the Trace, Adds the point, then writes it back.
	for name, m := range byName {
		d.jobs <- &Job{
			Name:    name,
			Entries: m,
			CIndex:  cindex,
		}
	}
	return nil
}

func tileKey(cindex int) *datastore.Key {
	key := ds.NewKey(dsconst.TILE)
	key.Name = fmt.Sprintf("%d", int64(cindex/TILE_SIZE))
	return key
}

func testNameKey(cindex int, testName string) *datastore.Key {
	key := ds.NewKey(dsconst.TEST_NAME)
	key.Name = testName
	key.Parent = tileKey(cindex)
	return key
}

func traceKey(cindex int, testName, traceId string) *datastore.Key {
	key := ds.NewKey(dsconst.TRACE)
	key.Name = traceId
	key.Parent = testNameKey(cindex, testName)
	return key
}

func (d *DSTileStore) addTileAsNeeded(cindex int) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	key := tileKey(cindex)
	if d.seenTile[key.Name] {
		return nil
	}

	var tile Tile
	_, err := d.client.Put(d.ctx, key, &tile)
	if err != nil {
		return err
	}
	d.seenTile[key.Name] = true
	return nil
}

func (d *DSTileStore) addTestNamesAsNeeded(cindex int, names map[string]bool) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	testNamesToAdd := util.StringSet(names).Complement(d.seenTestNames)
	for name, _ := range testNamesToAdd {
		var testName TestName
		_, err := d.client.Put(d.ctx, testNameKey(cindex, name), &testName)
		if err != nil {
			return err
		}
		d.seenTestNames[name] = true
	}
	return nil
}
