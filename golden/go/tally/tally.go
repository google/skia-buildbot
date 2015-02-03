// tally returns counts of digests for various views on a Tile.
package tally

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"skia.googlesource.com/buildbot.git/go/timer"
	gtypes "skia.googlesource.com/buildbot.git/golden/go/types"
	"skia.googlesource.com/buildbot.git/perf/go/types"
)

type OnChangeCallback func()

// Tally maps a digest to a count.
type Tally map[string]int

// TraceTally maps each trace id to its Tally.
type TraceTally map[string]*Tally

// TestTally maps each test name to its Tally.
type TestTally map[string]*Tally

// Tallies allows querying for digest counts in different ways.
type Tallies struct {
	mutex      sync.Mutex
	tileStore  types.TileStore
	traceTally TraceTally
	testTally  TestTally
	callbacks  []OnChangeCallback
}

// New creates a new Tallies for the given TileStore.
func New(tileStore types.TileStore) (*Tallies, error) {
	tile, err := tileStore.Get(0, -1)
	if err != nil {
		return nil, fmt.Errorf("Couldn't retrieve tile: %s", err)
	}
	trace, test := tallyTile(tile)
	t := &Tallies{
		traceTally: trace,
		testTally:  test,
		tileStore:  tileStore,
		callbacks:  []OnChangeCallback{},
	}
	go func() {
		for _ = range time.Tick(2 * time.Minute) {
			trace, test := tallyTile(tile)
			t.mutex.Lock()
			t.traceTally = trace
			t.testTally = test
			t.mutex.Unlock()
			for _, cb := range t.callbacks {
				cb()
			}
		}
	}()
	return t, nil
}

func (t *Tallies) OnChange(f OnChangeCallback) {
	t.callbacks = append(t.callbacks, f)
}

func (t *Tallies) ByTest() TestTally {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.testTally
}

func (t *Tallies) ByTrace() TraceTally {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.traceTally
}

// ByQuery returns a Tally of all the digests that match the given query.
func (t *Tallies) ByQuery(query url.Values) (Tally, error) {
	tile, err := t.tileStore.Get(0, -1)
	if err != nil {
		return nil, fmt.Errorf("Couldn't retrieve tile: %s", err)
	}
	return tallyBy(tile, t.traceTally, query), nil

}

// tallyBy does the actual work of ByQuery.
func tallyBy(tile *types.Tile, traceTally TraceTally, query url.Values) Tally {
	ret := Tally{}
	for k, tr := range tile.Traces {
		if types.Matches(tr, query) {
			for digest, n := range *traceTally[k] {
				if _, ok := ret[digest]; ok {
					ret[digest] += n
				} else {
					ret[digest] = n
				}
			}
		}
	}
	return ret
}

// tallyTile computes a TraceTally and TestTally from the given Tile.
func tallyTile(tile *types.Tile) (TraceTally, TestTally) {
	defer timer.New("tally").Stop()
	traceTally := TraceTally{}
	testTally := TestTally{}
	for k, tr := range tile.Traces {
		gtr := tr.(*types.GoldenTrace)
		tally := Tally{}
		for _, s := range gtr.Values {
			if s == types.MISSING_DIGEST {
				continue
			}
			if n, ok := tally[s]; ok {
				tally[s] = n + 1
			} else {
				tally[s] = 1
			}
		}
		traceTally[k] = &tally
		testName := tr.Params()[gtypes.PRIMARY_KEY_FIELD]
		if t, ok := testTally[testName]; ok {
			for digest, n := range tally {
				if _, ok := (*t)[digest]; ok {
					(*t)[digest] += n
				} else {
					(*t)[digest] = n
				}
			}
		} else {
			cp := Tally{}
			for k, v := range tally {
				cp[k] = v
			}
			testTally[testName] = &cp
		}
	}
	return traceTally, testTally
}
