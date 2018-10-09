package btts

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"testing"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/btts_testutils"
	"go.skia.org/infra/perf/go/config"
)

var (
	cfg = &config.PerfBigTableConfig{
		TileSize:     256,
		Project:      "test",
		Instance:     "test",
		Table:        "test",
		Topic:        "",
		GitUrl:       "",
		Subscription: "",
		Shards:       8,
	}
)

func TestBasic(t *testing.T) {
	testutils.LargeTest(t)
	ctx := context.Background()
	btts_testutils.CreateTestTable(t)
	defer btts_testutils.CleanUpTestTable(t)

	b, err := NewBigTableTraceStoreFromConfig(ctx, cfg, &btts_testutils.MockTS{}, true)
	assert.NoError(t, err)

	// Create an OPS in a fresh tile.
	tileKey := TileKeyFromOffset(1)
	op, err := b.UpdateOrderedParamSet(tileKey, paramtools.ParamSet{
		"cpu":    []string{"x86", "arm"},
		"config": []string{"8888", "565"},
	})
	assert.NoError(t, err)
	assert.Len(t, op.KeyOrder, 2)

	// Then update that OPS.
	op, err = b.UpdateOrderedParamSet(tileKey, paramtools.ParamSet{
		"os": []string{"linux", "win"},
	})
	assert.NoError(t, err)
	assert.Len(t, op.KeyOrder, 3)

	// Do we calculate LatestTile correctly?
	latest, err := b.GetLatestTile()
	assert.NoError(t, err)
	assert.Equal(t, int32(1), latest.Offset())

	// Add an OPS for a new tile.
	tileKey2 := TileKeyFromOffset(4)
	op, err = b.UpdateOrderedParamSet(tileKey2, paramtools.ParamSet{
		"os": []string{"win", "linux"},
	})

	// Do we calculate LatestTile correctly?
	latest, err = b.GetLatestTile()
	assert.NoError(t, err)
	assert.Equal(t, int32(4), latest.Offset())

	// Create another instance, so it has no cache.
	b2, err := NewBigTableTraceStoreFromConfig(ctx, cfg, &btts_testutils.MockTS{}, false)
	assert.NoError(t, err)

	// OPS for tile 4 should be a no-op since it's already in BT.
	op2, err := b2.UpdateOrderedParamSet(tileKey2, paramtools.ParamSet{
		// Note we reverse "linux", "win" order, but still get the same
		// result as op.
		"os": []string{"linux", "win"},
	})
	assert.Equal(t, op, op2)
}

func encodeParams(t *testing.T, op *paramtools.OrderedParamSet, p paramtools.Params) string {
	key, err := op.EncodeParamsAsString(p)
	assert.NoError(t, err)
	return key
}

func TestTraces(t *testing.T) {
	testutils.LargeTest(t)
	ctx := context.Background()
	btts_testutils.CreateTestTable(t)
	defer btts_testutils.CleanUpTestTable(t)
	now := time.Now()

	b, err := NewBigTableTraceStoreFromConfig(ctx, cfg, &btts_testutils.MockTS{}, true)
	assert.NoError(t, err)

	tileKey := TileKeyFromOffset(1)
	op, err := b.UpdateOrderedParamSet(tileKey, paramtools.ParamSet{
		"cpu": []string{"x86", "arm"},
	})
	assert.NoError(t, err)
	op, err = b.UpdateOrderedParamSet(tileKey, paramtools.ParamSet{
		"config": []string{"8888", "565"},
	})
	assert.NoError(t, err)
	values := map[string]float32{
		encodeParams(t, op, paramtools.Params{"cpu": "x86", "config": "8888"}): 1.0,
		encodeParams(t, op, paramtools.Params{"cpu": "x86", "config": "565"}):  1.1,
		encodeParams(t, op, paramtools.Params{"cpu": "arm", "config": "8888"}): 1.2,
		encodeParams(t, op, paramtools.Params{"cpu": "arm", "config": "565"}):  1.3,
	}
	err = b.WriteTraces(257, values, "gs://some/test/location", now)
	assert.NoError(t, err)

	q, err := query.New(url.Values{"config": []string{"8888"}})
	assert.NoError(t, err)
	r, err := q.Regexp(op)
	assert.NoError(t, err)

	results, err := b.QueryTraces(tileKey, r)
	assert.NoError(t, err)
	vec1 := vec32.New(256)
	vec1[1] = 1.0
	vec2 := vec32.New(256)
	vec2[1] = 1.2
	expected := map[string][]float32{
		",0=0,1=0,": vec1,
		",0=1,1=0,": vec2,
	}
	assert.Equal(t, expected, results)

	// Now overwrite a value.
	values = map[string]float32{
		encodeParams(t, op, paramtools.Params{"cpu": "x86", "config": "8888"}): 2.0,
	}
	err = b.WriteTraces(257, values, "gs://some/other/test/location", now)
	assert.NoError(t, err)

	// Query again to get the updated value.
	results, err = b.QueryTraces(tileKey, r)
	assert.NoError(t, err)
	vec1 = vec32.New(256)
	vec1[1] = 2.0
	vec2 = vec32.New(256)
	vec2[1] = 1.2
	expected = map[string][]float32{
		",0=0,1=0,": vec1,
		",0=1,1=0,": vec2,
	}
	assert.Equal(t, expected, results)

	// Write in the next column.
	values = map[string]float32{
		encodeParams(t, op, paramtools.Params{"cpu": "x86", "config": "8888"}): 3.0,
	}
	err = b.WriteTraces(258, values, "gs://some/other/test/location", now)
	assert.NoError(t, err)

	// Query again to get the updated value.
	results, err = b.QueryTraces(tileKey, r)
	assert.NoError(t, err)
	vec1 = vec32.New(256)
	vec1[1] = 2.0
	vec1[2] = 3.0
	vec2 = vec32.New(256)
	vec2[1] = 1.2
	expected = map[string][]float32{
		",0=0,1=0,": vec1,
		",0=1,1=0,": vec2,
	}
	assert.Equal(t, expected, results)

	// Source
	s, err := b.GetSource(258, encodeParams(t, op, paramtools.Params{"cpu": "x86", "config": "8888"}))
	assert.NoError(t, err)
	assert.Equal(t, "gs://some/other/test/location", s)

	s, err = b.GetSource(259, encodeParams(t, op, paramtools.Params{"cpu": "x86", "config": "8888"}))
	assert.Error(t, err)
	assert.Equal(t, "", s)
}

func TestTileKey(t *testing.T) {
	testutils.SmallTest(t)

	numShards := int32(3)
	tileKey := TileKeyFromOffset(0)
	assert.Equal(t, int32(math.MaxInt32), int32(tileKey))
	assert.Equal(t, int32(0), tileKey.Offset())
	assert.Equal(t, "@2147483647", tileKey.OpsRowName())
	assert.Equal(t, "2:2147483647:", tileKey.TraceRowPrefix(2))
	assert.Equal(t, "1:2147483647:,0=1,", tileKey.TraceRowName(",0=1,", numShards))

	tileKey = TileKeyFromOffset(1)
	assert.Equal(t, int32(math.MaxInt32-1), int32(tileKey))
	assert.Equal(t, "@2147483646", tileKey.OpsRowName())
	assert.Equal(t, "3:2147483646:", tileKey.TraceRowPrefix(3))
	assert.Equal(t, "1:2147483646:,0=1,", tileKey.TraceRowName(",0=1,", numShards))

	tileKey = TileKeyFromOffset(-1)
	assert.Equal(t, BadTileKey, tileKey)

	var err error
	tileKey, err = TileKeyFromOpsRowName("2147483646")
	assert.Error(t, err)
	assert.Equal(t, BadTileKey, tileKey)

	tileKey, err = TileKeyFromOpsRowName("@2147483637")
	assert.NoError(t, err)
	assert.Equal(t, "@2147483637", tileKey.OpsRowName())
}

const rowJson = `{
  "D": [
    {
      "Row": "@2147483643",
      "Column": "D:H",
      "Timestamp": 1536145696388000,
      "Value": "NWY2MDQ5ZTk3ODdiMDcxMGFhY2U2MTYzNDU3NzRiNTk="
    },
    {
      "Row": "@2147483643",
      "Column": "D:OPS",
      "Timestamp": 1536145696388000,
      "Value": "Of+BAwEBD09yZGVyZWRQYXJhbVNldAH/ggABAgEIS2V5T3JkZXIB/4QAAQhQYXJhbVNldAH/hgAAABb/gwIBAQhbXXN0cmluZwH/hAABDAAAGf+FBAEBCFBhcmFtU2V0Af+GAAEMAf+EAAAY/4IBAQJvcwEBAm9zAgN3aW4FbGludXgA"
    }
  ]
}`

func TestOpsCacheEntry(t *testing.T) {
	testutils.SmallTest(t)
	// Entry for an empty OPS.
	o, err := NewOpsCacheEntry()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(o.ops.KeyOrder))
	assert.Equal(t, "c011636276b346664a4d3a473ff07fc5", o.hash)

	// Entry for an OPS with just one key-value pair:
	ops := o.ops.Copy()
	ops.Update(paramtools.ParamSet{"config": []string{"8888"}})
	o2, err := opsCacheEntryFromOPS(ops)
	assert.NoError(t, err)
	assert.Equal(t, []string{"config"}, o2.ops.KeyOrder)
	assert.Equal(t, "7a59e4600a8f20900c933037d8e0011a", o2.hash)

	// From a BigTable row.
	goodRow := bigtable.Row{}
	err = json.Unmarshal([]byte(rowJson), &goodRow)
	assert.NoError(t, err)

	o3, err := NewOpsCacheEntryFromRow(goodRow)
	assert.NoError(t, err)
	assert.Equal(t, []string{"os"}, o3.ops.KeyOrder)
	assert.Equal(t, "5f6049e9787b0710aace616345774b59", o3.hash)

	// Empty BT row.
	row := bigtable.Row{}
	_, err = NewOpsCacheEntryFromRow(row)
	assert.Error(t, err)

	// Nothing missing.
	row = bigtable.Row{
		"D": []bigtable.ReadItem{
			goodRow["D"][0],
			goodRow["D"][1],
		},
	}
	_, err = NewOpsCacheEntryFromRow(row)
	assert.NoError(t, err)

	// Missing H.
	row = bigtable.Row{
		"D": []bigtable.ReadItem{
			goodRow["D"][0],
		},
	}
	_, err = NewOpsCacheEntryFromRow(row)
	assert.Error(t, err)

	// Missing OPS.
	row = bigtable.Row{
		"D": []bigtable.ReadItem{
			goodRow["D"][1],
		},
	}
	_, err = NewOpsCacheEntryFromRow(row)
	assert.Error(t, err)
}
