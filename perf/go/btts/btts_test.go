package btts

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/btts_testutils"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/types"
)

var (
	cfg = &config.PerfBigTableConfig{
		TileSize: 256,
		Project:  "test",
		Instance: "test",
		Table:    "test",
		Topic:    "",
		GitUrl:   "",
		Shards:   8,
	}
)

func TestBasic(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	ctx := context.Background()
	btts_testutils.CreateTestTable(t)
	defer btts_testutils.CleanUpTestTable(t)

	b, err := NewBigTableTraceStoreFromConfig(ctx, cfg, &btts_testutils.MockTS{}, true)
	assert.NoError(t, err)

	// Create an OPS in a fresh tile.
	tileKey := tileKeyFromOffset(1)
	op, err := b.updateOrderedParamSet(tileKey, paramtools.ParamSet{
		"cpu":    []string{"x86", "arm"},
		"config": []string{"8888", "565"},
	})
	assert.NoError(t, err)
	assert.Len(t, op.KeyOrder, 2)

	// Then update that OPS.
	op, err = b.updateOrderedParamSet(tileKey, paramtools.ParamSet{
		"os": []string{"linux", "win"},
	})
	assert.NoError(t, err)
	assert.Len(t, op.KeyOrder, 3)

	// Do we calculate LatestTile correctly?
	latest, err := b.GetLatestTile()
	assert.NoError(t, err)
	assert.Equal(t, int32(1), latest.Offset())

	// Add an OPS for a new tile.
	tileKey2 := tileKeyFromOffset(4)
	op, err = b.updateOrderedParamSet(tileKey2, paramtools.ParamSet{
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
	op2, err := b2.updateOrderedParamSet(tileKey2, paramtools.ParamSet{
		// Note we reverse "linux", "win" order, but still get the same
		// result as op.
		"os": []string{"linux", "win"},
	})
	assert.Equal(t, op, op2)
}

// assertIndices asserts that the indices in the table match expectedKeys and expectedColumns.
//
// Returns the number of rows actually retrieved.
func assertIndices(t *testing.T, ops *paramtools.OrderedParamSet, b *BigTableTraceStore, expectedKeys []string, expectedColumns map[string][]string, msg string) int {
	var count int
	err := b.getTable().ReadRows(context.Background(), bigtable.PrefixRange(INDEX_PREFIX), func(row bigtable.Row) bool {
		count += 1
		assert.Contains(t, expectedKeys, row.Key())
		for _, col := range row[INDEX_FAMILY] {
			// Strip off the family name which is prefixed.
			rowKey := strings.Split(col.Column, ":")[3]
			p, err := ops.DecodeParamsFromString(rowKey)
			assert.NoError(t, err)
			decodedKey, err := query.MakeKeyFast(p)
			assert.NoError(t, err)
			assert.Contains(t, expectedColumns[col.Row], decodedKey, fmt.Sprintf("For row: %q", row.Key()))
		}
		return true
	}, bigtable.RowFilter(
		bigtable.ChainFilters(
			bigtable.LatestNFilter(1),
			bigtable.FamilyFilter(INDEX_FAMILY),
		),
	),
	)
	assert.NoError(t, err)
	assert.Equal(t, len(expectedKeys), b.indexed.Len())
	return count
}

func TestTraces(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	ctx := context.Background()
	btts_testutils.CreateTestTable(t)
	defer btts_testutils.CleanUpTestTable(t)
	now := time.Now()

	b, err := NewBigTableTraceStoreFromConfig(ctx, cfg, &btts_testutils.MockTS{}, true)
	assert.NoError(t, err)

	tileKey := tileKeyFromOffset(1)
	ops, err := b.GetOrderedParamSet(ctx, tileKey)
	assert.NoError(t, err)
	assertIndices(t, ops, b, nil, nil, "Start empty")

	paramset := paramtools.ParamSet{
		"config": []string{"8888", "565"},
		"cpu":    []string{"x86", "arm"},
	}
	assert.NoError(t, err)
	params := []paramtools.Params{
		{"cpu": "x86", "config": "8888"},
		{"cpu": "x86", "config": "565"},
		{"cpu": "arm", "config": "8888"},
		{"cpu": "arm", "config": "565"},
	}
	values := []float32{
		1.0,
		1.1,
		1.2,
		1.3,
	}
	err = b.WriteTraces(257, params, values, paramset, "gs://some/test/location", now)
	assert.NoError(t, err)

	// Confirm that indices were written correctly.
	expectedKeys := []string{
		"i2147483646:config:565",
		"i2147483646:config:8888",
		"i2147483646:cpu:arm",
		"i2147483646:cpu:x86",
	}
	expectedColumns := map[string][]string{
		"i2147483646:config:565":  {",config=565,cpu=x86,", ",config=565,cpu=arm,"},
		"i2147483646:config:8888": {",config=8888,cpu=x86,", ",config=8888,cpu=arm,"},
		"i2147483646:cpu:arm":     {",config=565,cpu=arm,", ",config=8888,cpu=arm,"},
		"i2147483646:cpu:x86":     {",config=565,cpu=x86,", ",config=8888,cpu=x86,"},
	}
	ops, err = b.GetOrderedParamSet(ctx, tileKey)
	assert.NoError(t, err)
	count := assertIndices(t, ops, b, expectedKeys, expectedColumns, "First write")
	assert.Equal(t, 4, count)

	q, err := query.New(url.Values{"config": []string{"8888"}})
	assert.NoError(t, err)

	results, err := b.QueryTraces(ctx, tileKey, q)
	assert.NoError(t, err)
	vec1 := vec32.New(256)
	vec1[1] = 1.0
	vec2 := vec32.New(256)
	vec2[1] = 1.2
	expected := types.TraceSet{
		",config=8888,cpu=x86,": vec1,
		",config=8888,cpu=arm,": vec2,
	}
	assert.Equal(t, expected, results)

	keys, err := b.TileKeys(ctx, tileKey)
	assert.NoError(t, err)
	sort.Strings(keys)
	assert.Equal(t, []string{",config=565,cpu=arm,", ",config=565,cpu=x86,", ",config=8888,cpu=arm,", ",config=8888,cpu=x86,"}, keys)

	// Now overwrite a value.
	params = []paramtools.Params{
		{"cpu": "x86", "config": "8888"},
	}
	values = []float32{
		2.0,
	}
	err = b.WriteTraces(257, params, values, paramset, "gs://some/other/test/location", now)
	assert.NoError(t, err)
	ops, err = b.GetOrderedParamSet(ctx, tileKey)
	assert.NoError(t, err)
	count = assertIndices(t, ops, b, expectedKeys, expectedColumns, "Overwrite")
	assert.Equal(t, 4, count)

	// Query again to get the updated value.
	results, err = b.QueryTraces(context.Background(), tileKey, q)
	assert.NoError(t, err)
	vec1 = vec32.New(256)
	vec1[1] = 2.0
	vec2 = vec32.New(256)
	vec2[1] = 1.2
	expected = types.TraceSet{
		",config=8888,cpu=x86,": vec1,
		",config=8888,cpu=arm,": vec2,
	}
	assert.Equal(t, expected, results)

	// Write in the next column.
	params = []paramtools.Params{
		{"cpu": "x86", "config": "8888"},
	}
	values = []float32{
		3.0,
	}
	err = b.WriteTraces(258, params, values, paramset, "gs://some/other/test/location", now)
	assert.NoError(t, err)

	// Query again to get the updated value.
	results, err = b.QueryTraces(context.Background(), tileKey, q)
	assert.NoError(t, err)
	vec1 = vec32.New(256)
	vec1[1] = 2.0
	vec1[2] = 3.0
	vec2 = vec32.New(256)
	vec2[1] = 1.2
	expected = types.TraceSet{
		",config=8888,cpu=x86,": vec1,
		",config=8888,cpu=arm,": vec2,
	}
	assert.Equal(t, expected, results)
	count = assertIndices(t, ops, b, expectedKeys, expectedColumns, "Write new value.")
	assert.Equal(t, 4, count)

	// Write to a new trace.
	params = []paramtools.Params{
		{"cpu": "risc-v", "config": "8888"},
	}
	values = []float32{
		2.0,
	}
	paramset.AddParams(params[0])
	err = b.WriteTraces(258, params, values, paramset, "gs://some/other/test/location", now)
	assert.NoError(t, err)

	// Add new trace to expectations.
	expectedKeys = append(expectedKeys, "i2147483646:cpu:risc-v")
	expectedColumns["i2147483646:cpu:risc-v"] = []string{",config=8888,cpu=risc-v,"}
	expectedColumns["i2147483646:config:8888"] = append(expectedColumns["i2147483646:config:8888"], ",config=8888,cpu=risc-v,")
	ops, err = b.GetOrderedParamSet(ctx, tileKey)
	assert.NoError(t, err)
	count = assertIndices(t, ops, b, expectedKeys, expectedColumns, "Add new trace")
	assert.Equal(t, 5, count)

	// Remove indices and repopulate with WriteIndices.
	muts := []*bigtable.Mutation{}
	for range expectedKeys {
		mut := bigtable.NewMutation()
		mut.DeleteRow()
		muts = append(muts, mut)
	}
	errs, err := b.getTable().ApplyBulk(ctx, expectedKeys, muts)
	assert.NoError(t, err)
	for _, e := range errs {
		assert.NoError(t, e)
	}
	count = assertIndices(t, ops, b, expectedKeys, expectedColumns, "Add new trace")
	assert.Equal(t, 0, count)
	err = b.WriteIndices(ctx, tileKey)
	assert.NoError(t, err)
	count = assertIndices(t, ops, b, expectedKeys, expectedColumns, "Add new trace")
	assert.Equal(t, 5, count)

	// Source
	traceId, err := query.MakeKey(paramtools.Params{"cpu": "x86", "config": "8888"})
	assert.NoError(t, err)
	s, err := b.GetSource(context.Background(), 258, traceId)
	assert.NoError(t, err)
	assert.Equal(t, "gs://some/other/test/location", s)

	s, err = b.GetSource(context.Background(), 259, traceId)
	assert.Error(t, err)
	assert.Equal(t, "", s)
}

func TestTileKey(t *testing.T) {
	unittest.SmallTest(t)

	numShards := int32(3)
	tileKey := tileKeyFromOffset(0)
	assert.Equal(t, int32(math.MaxInt32), int32(tileKey))
	assert.Equal(t, int32(0), tileKey.Offset())
	assert.Equal(t, "@2147483647", tileKey.OpsRowName())
	assert.Equal(t, "2:2147483647:", tileKey.TraceRowPrefix(2))
	assert.Equal(t, "1:2147483647:,0=1,", tileKey.TraceRowName(",0=1,", numShards))

	tileKey = tileKeyFromOffset(1)
	assert.Equal(t, int32(math.MaxInt32-1), int32(tileKey))
	assert.Equal(t, "@2147483646", tileKey.OpsRowName())
	assert.Equal(t, "3:2147483646:", tileKey.TraceRowPrefix(3))
	assert.Equal(t, "1:2147483646:,0=1,", tileKey.TraceRowName(",0=1,", numShards))

	tileKey = tileKeyFromOffset(-1)
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
	unittest.SmallTest(t)
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
