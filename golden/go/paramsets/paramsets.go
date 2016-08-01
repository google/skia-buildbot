// paramsets keeps a running summary of paramsets per test, digest pair.
package paramsets

import (
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
)

// ParamSummary keep precalculated paramsets for each test, digest pair.
// It is not thread safe. The client of this package needs to make sure there
// are no conflicts.
type ParamSummary struct {
	// map [test:digest] paramset.
	byTrace               map[string]map[string][]string
	byTraceIncludeIgnored map[string]map[string][]string
}

// byTraceForTile calculates all the paramsets from the given tile and tallies.
func byTraceForTile(tile *tiling.Tile, traceTally map[string]tally.Tally) map[string]map[string][]string {
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
func oneStep(tilePair *types.TilePair, tallies *tally.Tallies) (map[string]map[string][]string, map[string]map[string][]string) {
	defer timer.New("paramsets").Stop()
	return byTraceForTile(tilePair.Tile, tallies.ByTrace()), byTraceForTile(tilePair.TileWithIgnores, tallies.ByTrace())
}

// New creates a new ParamSummary.
func New() *ParamSummary {
	return &ParamSummary{}
}

// Calculate sets the values the ParamSummary based on the given tile.
func (s *ParamSummary) Calculate(tilePair *types.TilePair, tallies *tally.Tallies) {
	s.byTrace, s.byTraceIncludeIgnored = oneStep(tilePair, tallies)
}

// Get returns the paramset for the given digest. If 'include' is true
// then the paramset is calculated including ignored traces.
func (s *ParamSummary) Get(test, digest string, include bool) map[string][]string {
	if include {
		return s.byTraceIncludeIgnored[test+":"+digest]
	}
	return s.byTrace[test+":"+digest]
}
