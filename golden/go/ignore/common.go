package ignore

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
)

// FilterIgnored returns a copy of the given tile with all traces removed
// that match the ignore rules in the given ignore store. It also returns the
// ignore rules for later matching.
func FilterIgnored(inputTile *tiling.Tile, ignores []*Rule) (*tiling.Tile, paramtools.ParamMatcher, error) {
	// Make a shallow copy with a new Traces map
	ret := &tiling.Tile{
		Traces:   map[tiling.TraceID]tiling.Trace{},
		ParamSet: inputTile.ParamSet,
		Commits:  inputTile.Commits,

		Scale:     inputTile.Scale,
		TileIndex: inputTile.TileIndex,
	}

	// Then, add any traces that don't match any ignore rules
	ignoreQueries, err := toQuery(ignores)
	if err != nil {
		return nil, nil, err
	}
nextTrace:
	for id, tr := range inputTile.Traces {
		for _, q := range ignoreQueries {
			if tiling.Matches(tr, q) {
				continue nextTrace
			}
		}
		ret.Traces[id] = tr
	}

	ignoreRules := make([]paramtools.ParamSet, len(ignoreQueries))
	for idx, q := range ignoreQueries {
		ignoreRules[idx] = paramtools.ParamSet(q)
	}
	return ret, ignoreRules, nil
}
