package etile

import (
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

type GTile struct {
	Traces    []GTrace
	Params    []string
	ParamVals []string
	Digests   map[int32]string

	// data
	vals     []string
	valsMap  map[string]int32
	traceLen int
	buf      []int32
}

type GTrace struct {
	Values []int32
	Params map[int32]int32
}

func FromTile(tile *tiling.Tile) *GTile {
	traceLen := len(tile.Commits)
	nTraces := len(tile.Traces)
	intTraces := make([]GTrace, len(tile.Traces))
	mapSize := 1 << 15

	ret := &GTile{
		vals:     make([]string, 0, mapSize),
		valsMap:  make(map[string]int32, mapSize),
		traceLen: traceLen,
		buf:      make([]int32, traceLen*nTraces),
	}

	idx := 0
	sliceStart := 0
	for _, trace := range tile.Traces {
		goldTrace := trace.(*types.GoldenTrace)
		it := intTraces[idx]
		it.Values = ret.getValues(goldTrace.Values, sliceStart)
		it.Params = ret.getParams(goldTrace.Params_)
		idx++
		sliceStart += traceLen
	}

	return ret
}

func (g *GTile) getValues(vals []string, sliceStart int) []int32 {
	ret := g.buf[sliceStart : sliceStart+len(vals)]
	for idx, val := range vals {
		if intVal, ok := g.valsMap[val]; ok {
			ret[idx] = intVal
		} else {
			g.valsMap[val] = int32(len(g.vals))
			vals = append(g.vals, val)
		}
	}
	return ret
}

func (g *GTile) getParams(params map[string]string) map[int32]int32 {
	return nil
}
