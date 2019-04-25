package gtracestore

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/gtile"
	"go.skia.org/infra/golden/go/types"
)

type Trace []int32

type IParams map[int32]int32
type IParamSet map[int32][]int32

type DataFrame struct {
	Traces  []Trace
	IDs     []int32
	Params  []IParams
	Commits []*tiling.Commit

	// auxiliary members
	sym *dfSymbols
}

type dfSymbols struct {
	paramsMap  gtile.StrMap
	digestsMap gtile.StrMap
	idsMap     gtile.StrMap
}

func newDFSymbols() *dfSymbols {
	// Initialize the value to int mapping.
	ret := &dfSymbols{}
	mapSize := 1 << 15
	ret.paramsMap.Init(mapSize)
	ret.digestsMap.Init(mapSize)
	ret.idsMap.Init(mapSize)
	return ret
}

func DataFrameFromTile(tile *tiling.Tile) *DataFrame {
	nTraces := len(tile.Traces)

	ret := &DataFrame{
		Traces:  make([]Trace, nTraces),
		IDs:     make([]int32, nTraces),
		Params:  make([]IParams, nTraces),
		Commits: tile.Commits,
	}

	sym := newDFSymbols()

	// Add the missing digest first.
	// TODO: move to init of
	sym.digestsMap.ToInt(types.MISSING_DIGEST)

	for traceId, trace := range tile.Traces {
		goldTrace := trace.(*types.GoldenTrace)

		// Note: idx is incremented by one in every iteration since each new
		// traceId is new in idMap.
		idx := sym.idsMap.ToInt(traceId)
		ret.Traces[idx] = sym.digestsMap.IntSlice(goldTrace.Values, nil)
		ret.IDs[idx] = idx
		ret.Params[idx] = sym.paramsMap.IntMap(goldTrace.Params_)
	}

	ret.sym = sym
	return ret
}

func (d *DataFrame) ToTile() *tiling.Tile {
	ret := &tiling.Tile{
		Traces:   make(map[string]tiling.Trace, len(d.Traces)),
		ParamSet: paramtools.ParamSet{},
		Commits:  d.Commits,
	}

	allParams := paramtools.ParamSet{}
	for idx, trace := range d.Traces {
		tid := d.IDs[idx]
		params := d.Params[idx]
		goldTrace := &types.GoldenTrace{
			Params_: d.sym.paramsMap.StrMap(params),
			Values:  d.sym.digestsMap.StrSlice(trace),
		}
		allParams.AddParams(goldTrace.Params_)
		ret.Traces[d.sym.idsMap.Vals[tid]] = goldTrace
	}

	ret.ParamSet = allParams
	return ret
}
