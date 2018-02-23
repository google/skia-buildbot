package gtile

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

type GTile struct {
	Traces  []GTrace
	Commits []*tiling.Commit

	// mappings
	paramMap  StrMap
	digestMap StrMap
	idMap     StrMap
	traceLen  int
	traceBuf  []int32
}

type GTrace struct {
	Values []int32
	Params map[int32]int32
}

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

func (g *GTile) IntParamSet(paramSet paramtools.ParamSet) IntParamSet {
	return nil
}

func (g *GTile) ParamSet(intParamSet IntParamSet) paramtools.ParamSet {
	return nil
}

func (g *GTile) Search(q IntParamSet) *SearchResult {
	// foundTraces := interSlice(make([]int, 0, len(g.Traces)))
	// for key, vals := range q {
	// 	foundTraces.intersect(g.indices[key].getTraces(vals)...)
	// }
	return nil
}

type interSlice []int

func (i interSlice) intersect(right interSlice) {

}

type IntParams map[int32]int32
type IntParamSet map[int32][]int32

type SearchResult struct {
	// traces and parent tile.
}

func (s *SearchResult) Search(q map[int32]int32) *SearchResult {
	return nil
}

func (g *GTile) buildIndices() {

}
