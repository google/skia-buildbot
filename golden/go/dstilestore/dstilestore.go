package dstilestore

import (
	"context"
	"fmt"
	"sync"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/paramtools"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/dsconst"
	"go.skia.org/infra/golden/go/types"
)

const (
	TILE_SIZE = 50
)

type DSTileStore struct {
	ctx           context.Context
	mutex         sync.Mutex
	seenTile      map[int64]bool
	seenTestNames map[string]bool
}

type Tile struct{}
type TestName struct{}

type Trace struct {
	Digests []string `datastore:"digests,noindex"`
	Trace   []int    `datastore:"trace,noindex"`
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

func NewDSTileStore(ctx context.Context) *DSTileStore {
	return &DSTileStore{
		ctx:           ctx,
		seenTile:      map[int64]bool{},
		seenTestNames: map[string]bool{},
	}
}

func (d *DSTileStore) Add(cindex int, entries map[string]*tracedb.Entry) error {
	// Create Tile if necessary.
	if err := d.addTileAsNeeded(cindex); err != nil {
		return fmt.Errorf("Unable to add Tile to datastore: %s", err)
	}
	// Create TestName's as necessary.
	// Group entries by 'name'.
	// Pass each group down to a go routine pool to be added.
	// Each routine in the pool, in a tx, loads the Trace, Adds the point, then writes it back.
	return nil
}

func (d *DSTileStore) addTileAsNeeded(cindex int) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	tileIndex := int64(cindex / TILE_SIZE)

	if d.seenTile[tileIndex] {
		return nil
	}

	var tile Tile
	key := ds.NewKey(dsconst.TILE)
	key.ID = tileIndex
	_, err := ds.DS.Put(d.ctx, key, &tile)
	if err != nil {
		return err
	}
	d.seenTile[tileIndex] = true
	return nil
}
