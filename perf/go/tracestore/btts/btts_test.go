package btts

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/tracestore/btts/btts_testutils"
	"go.skia.org/infra/perf/go/types"
)

var (
	cfg = &config.InstanceConfig{
		DataStoreConfig: config.DataStoreConfig{
			TileSize: 256,
			Project:  "testtest",
			Instance: "testtest",
			Table:    "testtest",
			Shards:   8,
		},
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
	tileKey := TileKeyFromTileNumber(1)
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
	assert.Equal(t, types.TileNumber(1), latest)

	// Add an OPS for a new tile.
	tileKey2 := TileKeyFromTileNumber(4)
	op, err = b.updateOrderedParamSet(tileKey2, paramtools.ParamSet{
		"os": []string{"win", "linux"},
	})

	// Do we calculate LatestTile correctly?
	latest, err = b.GetLatestTile()
	assert.NoError(t, err)
	assert.Equal(t, types.TileNumber(4), latest)

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

func TestOPSThreaded(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	ctx := context.Background()
	btts_testutils.CreateTestTable(t)
	defer btts_testutils.CleanUpTestTable(t)

	b, err := NewBigTableTraceStoreFromConfig(ctx, cfg, &btts_testutils.MockTS{}, true)
	assert.NoError(t, err)

	tileKey := TileKeyFromTileNumber(1)

	expected := paramtools.ParamSet{}
	// Add multiple params to the OPS in goroutines.
	wg := sync.WaitGroup{}
	for _, cpu := range []string{"x86", "arm"} {
		for _, config := range []string{"8888", "565"} {
			for _, os := range []string{"linux", "win"} {
				paramset := paramtools.ParamSet{
					"cpu":    []string{cpu},
					"config": []string{config},
					"os":     []string{os},
				}
				expected.AddParamSet(paramset)
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, err := b.updateOrderedParamSet(tileKey, paramset)
					assert.NoError(t, err)
				}()
			}
		}
	}
	wg.Wait()

	// read current OPS
	entry, _, err := b.getOPS(tileKey)
	assert.NoError(t, err)
	expected.Normalize()
	entry.ops.ParamSet.Normalize()
	assert.Equal(t, expected, entry.ops.ParamSet)
}

// assertIndices asserts that the indices in the table match expectedKeys and
// expectedColumns. The 'params' is a slice of Params, one for each trace id we
// expect to find. Returns the number of rows actually retrieved.
func assertIndices(t *testing.T, ops *paramtools.OrderedParamSet, b *BigTableTraceStore, params []paramtools.Params, msg string) int {
	var count int
	err := b.getTable().ReadRows(context.Background(), bigtable.PrefixRange(INDEX_PREFIX), func(row bigtable.Row) bool {
		count++
		parts := strings.Split(row.Key(), ":")
		// The key (1) and value (2) should appear as part of the encoded
		// structured key (trace id) (3)
		substr := fmt.Sprintf(",%s=%s,", parts[1], parts[2])
		assert.Contains(t, parts[3], substr)
		p, err := ops.DecodeParamsFromString(parts[3])
		assert.NoError(t, err)
		assert.Contains(t, params, p)
		return true
	}, bigtable.RowFilter(
		bigtable.ChainFilters(
			bigtable.LatestNFilter(1),
			bigtable.FamilyFilter(INDEX_FAMILY),
		),
	),
	)
	assert.NoError(t, err)
	return count
}

// getIndexRowKeys returns the row keys for all the index entries.
func getIndexRowKeys(t *testing.T, b *BigTableTraceStore) []string {
	ret := []string{}
	err := b.getTable().ReadRows(context.Background(), bigtable.PrefixRange(INDEX_PREFIX), func(row bigtable.Row) bool {
		ret = append(ret, row.Key())
		return true
	})
	assert.NoError(t, err)
	return ret
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

	tileNumber := types.TileNumber(1)
	ops, err := b.GetOrderedParamSet(ctx, tileNumber)
	assert.NoError(t, err)
	assertIndices(t, ops, b, nil, "Start empty")

	paramset := paramtools.ParamSet{
		"config": []string{"8888", "565"},
		"cpu":    []string{"x86", "arm"},
	}
	assert.NoError(t, err)
	expectedParams := []paramtools.Params{
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
	err = b.WriteTraces(257, expectedParams, values, paramset, "gs://some/test/location", now)
	assert.NoError(t, err)

	ops, err = b.GetOrderedParamSet(ctx, tileNumber)
	assert.NoError(t, err)
	count := assertIndices(t, ops, b, expectedParams, "First write")
	assert.Equal(t, 8, count)
	indexCount, err := b.CountIndices(context.Background(), tileNumber)
	assert.NoError(t, err)
	assert.Equal(t, int64(8), indexCount)

	q, err := query.New(url.Values{"config": []string{"8888"}})
	assert.NoError(t, err)

	results, err := b.queryTraces(ctx, tileNumber, q)
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

	results, err = b.QueryTracesByIndex(context.Background(), tileNumber, q)
	assert.NoError(t, err)
	assert.Equal(t, expected, results)

	outCh, err := b.QueryTracesIDOnlyByIndex(ctx, tileNumber, q)
	keys := []string{}
	for key := range expected {
		keys = append(keys, key)
	}
	assert.NoError(t, err)
	for p := range outCh {
		key, err := query.MakeKeyFast(p)
		assert.NoError(t, err)
		assert.Contains(t, keys, key)
	}

	out, errCh := b.tileKeys(ctx, tileNumber)
	assert.Empty(t, errCh)
	keys = []string{}
	for s := range out {
		ps, err := ops.DecodeParamsFromString(s)
		assert.NoError(t, err)
		key, err := query.MakeKeyFast(ps)
		assert.NoError(t, err)
		keys = append(keys, key)
	}
	assert.NoError(t, err)
	sort.Strings(keys)
	assert.Equal(t, []string{",config=565,cpu=arm,", ",config=565,cpu=x86,", ",config=8888,cpu=arm,", ",config=8888,cpu=x86,"}, keys)

	// Now overwrite a value.
	overWriteParams := []paramtools.Params{
		{"cpu": "x86", "config": "8888"},
	}
	values = []float32{
		2.0,
	}
	err = b.WriteTraces(257, overWriteParams, values, paramset, "gs://some/other/test/location", now)
	assert.NoError(t, err)
	ops, err = b.GetOrderedParamSet(ctx, tileNumber)
	assert.NoError(t, err)
	count = assertIndices(t, ops, b, expectedParams, "Overwrite")
	assert.Equal(t, 8, count)

	// Query again to get the updated value.
	results, err = b.queryTraces(context.Background(), tileNumber, q)
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

	results, err = b.QueryTracesByIndex(context.Background(), tileNumber, q)
	assert.NoError(t, err)
	assert.Equal(t, expected, results)

	// Write in the next column.
	writeParams := []paramtools.Params{
		{"cpu": "x86", "config": "8888"},
	}
	values = []float32{
		3.0,
	}
	err = b.WriteTraces(258, writeParams, values, paramset, "gs://some/other/test/location", now)
	assert.NoError(t, err)

	// Query again to get the updated value.
	results, err = b.queryTraces(context.Background(), tileNumber, q)
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
	count = assertIndices(t, ops, b, expectedParams, "Write new value.")
	assert.Equal(t, 8, count)

	results, err = b.QueryTracesByIndex(context.Background(), tileNumber, q)
	assert.NoError(t, err)
	assert.Equal(t, expected, results)

	// Write to a new trace.
	writeParams = []paramtools.Params{
		{"cpu": "risc-v", "config": "8888"},
	}
	values = []float32{
		2.0,
	}
	paramset.AddParams(writeParams[0])
	err = b.WriteTraces(258, writeParams, values, paramset, "gs://some/other/test/location", now)
	assert.NoError(t, err)

	// Add new trace to expectations.
	expectedParams = append(expectedParams, writeParams[0])
	ops, err = b.GetOrderedParamSet(ctx, tileNumber)
	assert.NoError(t, err)
	count = assertIndices(t, ops, b, expectedParams, "Add new trace")
	assert.Equal(t, 10, count)

	// Remove indices and repopulate with WriteIndices.
	muts := []*bigtable.Mutation{}
	indexRowKeys := getIndexRowKeys(t, b)
	for range indexRowKeys {
		mut := bigtable.NewMutation()
		mut.DeleteRow()
		muts = append(muts, mut)
	}
	errs, err := b.getTable().ApplyBulk(ctx, indexRowKeys, muts)
	assert.NoError(t, err)
	for _, e := range errs {
		assert.NoError(t, e)
	}

	// Confirm they have all been deleted.
	count = assertIndices(t, ops, b, expectedParams, "Add new trace")
	assert.Equal(t, 0, count)

	// Write fresh indices.
	err = b.WriteIndices(ctx, tileNumber)
	assert.NoError(t, err)

	// Confirm they are correct.
	count = assertIndices(t, ops, b, expectedParams, "Add new trace")
	assert.Equal(t, 10, count)

	// Confirm we can get the source file location back.
	traceId, err := query.MakeKey(paramtools.Params{"cpu": "x86", "config": "8888"})
	assert.NoError(t, err)
	s, err := b.GetSource(context.Background(), 258, traceId)
	assert.NoError(t, err)
	assert.Equal(t, "gs://some/other/test/location", s)

	// Confirm we get an error trying to retrieve a source file that doesn't exist.
	s, err = b.GetSource(context.Background(), 259, traceId)
	assert.Error(t, err)
	assert.Equal(t, "", s)
}

func TestTileKey(t *testing.T) {
	unittest.SmallTest(t)

	numShards := int32(3)
	tileKey := TileKeyFromTileNumber(0)
	assert.Equal(t, int32(math.MaxInt32), int32(tileKey))
	assert.Equal(t, types.TileNumber(0), tileKey.Offset())
	assert.Equal(t, "@2147483647", tileKey.OpsRowName())
	assert.Equal(t, "2:2147483647:", tileKey.TraceRowPrefix(2))
	assert.Equal(t, "1:2147483647:,0=1,", tileKey.TraceRowName(",0=1,", numShards))
	rowName, shard := tileKey.TraceRowNameAndShard(",0=1,", numShards)
	assert.Equal(t, "1:2147483647:,0=1,", rowName)
	assert.Equal(t, uint32(1), shard)

	tileKey = TileKeyFromTileNumber(1)
	assert.Equal(t, int32(math.MaxInt32-1), int32(tileKey))
	assert.Equal(t, "@2147483646", tileKey.OpsRowName())
	assert.Equal(t, "3:2147483646:", tileKey.TraceRowPrefix(3))
	rowName, shard = tileKey.TraceRowNameAndShard(",0=1,", numShards)
	assert.Equal(t, "1:2147483646:,0=1,", rowName)
	assert.Equal(t, uint32(1), shard)

	rowName, shard = tileKey.TraceRowNameAndShard(",0=2,", numShards)
	assert.Equal(t, "0:2147483646:,0=2,", rowName)
	assert.Equal(t, uint32(0), shard)

	tileKey = TileKeyFromTileNumber(-1)
	assert.Equal(t, badBttsTileKey, tileKey)

	var err error
	tileKey, err = TileKeyFromOpsRowName("2147483646")
	assert.Error(t, err)
	assert.Equal(t, badBttsTileKey, tileKey)

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

func TestBigTableTraceStore_IndexOfTileStart(t *testing.T) {
	unittest.SmallTest(t)
	tests := []struct {
		name     string
		tileSize int32
		index    types.CommitNumber
		want     types.CommitNumber
	}{
		{
			name:     "basic",
			tileSize: 100,
			index:    2,
			want:     0,
		},
		{
			name:     "offset",
			tileSize: 100,
			index:    202,
			want:     200,
		},
		{
			name:     "offset exact",
			tileSize: 100,
			index:    200,
			want:     200,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BigTableTraceStore{
				tileSize: tt.tileSize,
			}
			if got := b.CommitNumberOfTileStart(tt.index); got != tt.want {
				t.Errorf("BigTableTraceStore.IndexOfTileStart() = %v, want %v", got, tt.want)
			}
		})
	}
}
