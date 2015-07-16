// paramsets keeps a running summary of paramsets per test, digest pair.
package paramsets

import (
	"fmt"
	"sync"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
	ptypes "go.skia.org/infra/perf/go/types"
)

// Summary keep precalculated paramsets for each test, digest pair.
type Summary struct {
	mutex sync.RWMutex

	// map [test:digest] paramset.
	byTrace               map[string]map[string][]string
	byTraceIncludeIgnored map[string]map[string][]string
}

// byTraceForTile calculates all the paramsets from the given tile and tallies.
func byTraceForTile(tile *ptypes.Tile, traceTally map[string]tally.Tally) map[string]map[string][]string {
	ret := map[string]map[string][]string{}

	for id, t := range traceTally {
		if tr, ok := tile.Traces[id]; ok {
			test := tr.Params()[types.PRIMARY_KEY_FIELD]
			for digest, _ := range t {
				key := test + ":" + digest
				if _, ok := ret[key]; !ok {
					ret[key] = map[string][]string{}
				}
				util.AddParamsToParamSet(ret[key], tr.Params())
			}
		}
	}

	return ret
}

// oneStep does a single step, calculating all the paramsets from the latest tile and tallies.
//
// Returns the paramsets for both the tile with and without ignored traces included.
func oneStep(tallies *tally.Tallies, storages *storage.Storage) (map[string]map[string][]string, map[string]map[string][]string, error) {
	defer timer.New("paramsets").Stop()

	tile, err := storages.GetLastTileTrimmed(false)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to get tile: %s", err)
	}
	byTrace := byTraceForTile(tile, tallies.ByTrace())

	tile, err = storages.GetLastTileTrimmed(true)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to get tile: %s", err)
	}
	byTraceIncludeIgnored := byTraceForTile(tile, tallies.ByTrace())

	return byTrace, byTraceIncludeIgnored, nil
}

// New creates a new Summary.
//
// The Summary listens for change events from the tallies object
// and calculates new paramsets on every event.
func New(tallies *tally.Tallies, storages *storage.Storage) *Summary {
	byTrace, byTraceIncludeIgnored, err := oneStep(tallies, storages)
	if err != nil {
		glog.Fatalf("Failed to calculate first step of paramsets: %s", err)
	}

	s := &Summary{
		byTrace:               byTrace,
		byTraceIncludeIgnored: byTraceIncludeIgnored,
	}

	tallies.OnChange(func() {
		byTrace, byTraceIncludeIgnored, err := oneStep(tallies, storages)
		if err != nil {
			glog.Errorf("Failed to calculate new paramsets, keeping old paramsets: %s", err)
			return
		}
		s.mutex.Lock()
		defer s.mutex.Unlock()
		s.byTrace = byTrace
		s.byTraceIncludeIgnored = byTraceIncludeIgnored
	})

	return s
}

// Get returns the paramset for the given digest. If 'include' is true
// then the paramset is calculated including ignored traces.
func (s *Summary) Get(test, digest string, include bool) map[string][]string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if include {
		return s.byTraceIncludeIgnored[test+":"+digest]
	} else {
		return s.byTrace[test+":"+digest]
	}
}
