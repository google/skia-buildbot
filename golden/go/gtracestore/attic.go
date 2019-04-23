package gtracestore

import (
	"fmt"
	"hash/crc32"
	"math"
	"strconv"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/gtile"
	"go.skia.org/infra/golden/go/types"
)

// TileKey is the identifier for each tile held in BigTable.
//
// Note that tile keys are in the opposite order of tile offset, that is, the first
// tile for a repo would be 0, and then 1, etc. Those are the offsets, and the most
// recent tile has the largest offset. To make it easy to find the most recent
// tile we calculate tilekey as math.MaxInt32 - tileoffset, so that more recent
// tiles come first in sort order.
type TileKey int32

// BadTileKey is returned in error conditions.
const BadTileKey = TileKey(-1)

// TileKeyFromOffset returns a TileKey from the tile offset.
func TileKeyFromOffset(tileOffset int32) TileKey {
	if tileOffset < 0 {
		return BadTileKey
	}
	return TileKey(math.MaxInt32 - tileOffset)
}

func (t TileKey) PrevTile() TileKey {
	return TileKeyFromOffset(t.Offset() - 1)
}

// OpsRowName returns the name of the BigTable row that the OrderedParamSet for this tile is stored at.
func (t TileKey) OpsRowName() string {
	return fmt.Sprintf("@%07d", t)
}

// TraceRowPrefix returns the prefix of a BigTable row name for any Trace in this tile.
func (t TileKey) TraceRowPrefix(shard int32) string {
	return fmt.Sprintf("%d:%07d:", shard, t)
}

// TraceRowPrefix returns the BigTable row name for the given trace, for the given number of shards.
// TraceRowName(",0=1,", 3) -> 2:2147483647:,0=1,
func (t TileKey) TraceRowName(traceId string, shards int32) string {
	return fmt.Sprintf("%d:%07d:%s", crc32.ChecksumIEEE([]byte(traceId))%uint32(shards), t, traceId)
}

// Offset returns the tile offset, i.e. the not-reversed number.
func (t TileKey) Offset() int32 {
	return math.MaxInt32 - int32(t)
}

func TileKeyFromOpsRowName(s string) (TileKey, error) {
	if s[:1] != "@" {
		return BadTileKey, fmt.Errorf("TileKey strings must beginw with @: Got %q", s)
	}
	i, err := strconv.ParseInt(s[1:], 10, 32)
	if err != nil {
		return BadTileKey, err
	}
	return TileKey(i), nil
}

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
