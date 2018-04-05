package gtile

import (
	"net/url"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

// Predefined parameter keys
const (
	ClassKey int32 = iota
)

// Predefined parameter values
const (
	PosVal int32 = iota
	NegVal
	UntVal
)

// A query against the tile.
type Query map[int32][]int32

// TODO  Remove ?
type IntParams map[int32]int32
type IntParamSet map[int32][]int32

type GTile struct {
	Traces  []GTrace
	Commits []*tiling.Commit

	// mappings
	paramMap  StrMap // maps param keys and values from string to int32
	digestMap StrMap // maps digests to int32 values
	idMap     StrMap // maps traceids to that strmap
	traceLen  int
	traceBuf  []int32
}

type GTrace struct {
	Values []int32
	Params map[int32]int32
}

type SearchResult struct{}

type SubSearchable struct{}

type Searchable interface {
	Search(q Query) *SearchResult
}

/** API ---------------------------------------- */

func FromTile(tile *tiling.Tile) *GTile {
	traceLen := len(tile.Commits)
	nTraces := len(tile.Traces)

	ret := &GTile{
		Traces:   make([]GTrace, nTraces),
		Commits:  tile.Commits,
		traceLen: traceLen,
		traceBuf: make([]int32, traceLen*nTraces),
	}

	// Initialize the value to int mapping.
	mapSize := 1 << 15
	ret.paramMap.init(mapSize)
	ret.digestMap.init(mapSize)
	ret.idMap.init(mapSize)

	// Add the missing digest first.
	ret.digestMap.toInt(types.MISSING_DIGEST)

	sliceStart := 0
	for traceId, trace := range tile.Traces {
		goldTrace := trace.(*types.GoldenTrace)

		// Note: idx is incremented by one in every iteration since each new
		// traceId is new in idMap.
		idx := ret.idMap.toInt(traceId)
		targetSlice := ret.traceBuf[sliceStart : sliceStart+traceLen]
		ret.Traces[idx].Values = ret.digestMap.intSlice(goldTrace.Values, targetSlice)
		ret.Traces[idx].Params = ret.paramMap.intMap(goldTrace.Params_)
		sliceStart += traceLen
	}

	ret.buildIndices()
	return ret
}

func (g *GTile) ToTile() *tiling.Tile {
	ret := &tiling.Tile{
		Traces:   make(map[string]tiling.Trace, len(g.Traces)),
		ParamSet: paramtools.ParamSet{},
		Commits:  g.Commits,
	}

	allParams := paramtools.ParamSet{}
	for idx := range g.Traces {
		goldTrace := &types.GoldenTrace{
			Params_: g.paramMap.strMap(g.Traces[idx].Params),
			Values:  g.digestMap.strSlice(g.Traces[idx].Values),
		}
		allParams.AddParams(goldTrace.Params_)
		ret.Traces[g.idMap.Vals[idx]] = goldTrace
	}

	ret.ParamSet = allParams
	return ret
}

// PopulateQuery implements
func (g *GTile) PopulateQuery(dest Query, src url.Values, lhsSrc url.Values, match []string) error {
	for key, vals := range src {
		if err := g.addValsToQuery(dest, key, vals); err != nil {
			return err
		}
	}

	for _, matchParam := range match {
		if err := g.addValsToQuery(dest, matchParam, lhsSrc[matchParam]); err != nil {
			return err
		}
	}
	return nil
}

// done
func (g *GTile) addValsToQuery(dest Query, key string, vals []string) error {
	if (key == "") || len(vals) == 0 {
		return nil
	}
	keyIntVal, ok := g.paramMap.ValsMap[key]
	if !ok {
		return sklog.FmtErrorf("Unknown parameter '%s'", key)
	}

	var paramIntVal int32
	intVals := make([]int32, 0, len(vals))
	for _, val := range vals {
		if paramIntVal, ok = g.paramMap.ValsMap[val]; !ok {
			return sklog.FmtErrorf("Unknown parameter value '%s'", val)
		}
		intVals = append(intVals, paramIntVal)
	}

	if len(intVals) > 0 {
		dest[keyIntVal] = intVals
	}
	return nil
}

func (g *GTile) SubSearch(query Query) *SubSearchable {
	return nil
}

func (s *SubSearchable) SearchPrimary(value string) *SubSearchable {
	return nil
}

func (s *SubSearchable) MatchVals(matchVals [][]string) *SubSearchable {
	return s
}

func (s *SubSearchable) Digests() ([]string, []int32) {
	return nil, nil
}

func (s *SubSearchable) Reset() {}

/*        IMPLEMENT  */

func (g *GTile) buildIndices() {}

//
//
//
// /*         UNTESTED AND UNUSED               */
//
//
//
//
//
func (g *GTile) IntParamSet(paramSet paramtools.ParamSet) IntParamSet {
	return nil
}

func (g *GTile) ParamSet(intParamSet IntParamSet) paramtools.ParamSet {
	return nil
}

type interSlice []int

func (i interSlice) intersectx(right interSlice) {

}

func (s *SearchResult) Search(q Query) *SearchResult {
	return nil
}
