package btts

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/tracestore/btts/btts_testutils"
	"go.skia.org/infra/perf/go/types"
)

func TestCloseOnCancel(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	// Set up a BigTableTraceStore with some data to read from.
	tileNumber := types.TileNumber(1)
	tileKey := TileKeyFromTileNumber(tileNumber)
	now := time.Now()
	ctx := context.Background()
	btts_testutils.CreateTestTable(t)
	defer btts_testutils.CleanUpTestTable(t)

	b, err := NewBigTableTraceStoreFromConfig(ctx, cfg, &btts_testutils.MockTS{}, true)
	assert.NoError(t, err)

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

	ops, err := b.GetOrderedParamSet(ctx, tileNumber, time.Now())
	assert.NoError(t, err)

	// Now that the Tile is populated construct an encoded key=value pair to
	// query over by encoding a paramset and choosing a single key=value pair.
	p, err := ops.EncodeParams(params[0])
	key := ""
	value := ""
	for k, v := range p {
		key = k
		value = v
		break
	}
	assert.NoError(t, err)

	// Create a cancellable context.
	ctx, cancel := context.WithCancel(context.Background())

	// Start the querying.
	out := ParamIndex(ctx, b.getTable(), tileKey, key, value, "test")

	// Cancel the context which should abort the BigTable read.
	cancel()

	// Confirm that the channel is closed.
	_, ok := <-out
	assert.False(t, ok)
}

func TestParamIndex(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	// Set up a BigTableTraceStore with some data to read from.
	tileNumber := types.TileNumber(1)
	tileKey := TileKeyFromTileNumber(tileNumber)
	now := time.Now()
	ctx := context.Background()
	btts_testutils.CreateTestTable(t)
	defer btts_testutils.CleanUpTestTable(t)

	b, err := NewBigTableTraceStoreFromConfig(ctx, cfg, &btts_testutils.MockTS{}, true)
	assert.NoError(t, err)

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

	ops, err := b.GetOrderedParamSet(ctx, tileNumber, time.Now())
	assert.NoError(t, err)

	// Pick out an encoded key=value pair that corresponds to a know unencoded
	// key=value pair.
	p, err := ops.EncodeParams(paramtools.Params{"cpu": "x86"})
	assert.NoError(t, err)
	key := ""
	value := ""
	for k, v := range p {
		key = k
		value = v
		break
	}

	errCh := make(chan error, 10)

	// Start the query.
	out := ParamIndex(context.Background(), b.getTable(), tileKey, key, value, "test")

	// Confirm that we get the keys to traces that match the key=value pair.
	expected := []paramtools.Params{
		{"cpu": "x86", "config": "8888"},
		{"cpu": "x86", "config": "565"},
	}
	count := 0
	for encodedKey := range out {
		dp, err := ops.DecodeParamsFromString(encodedKey)
		assert.NoError(t, err)
		assert.Contains(t, expected, dp)
		count++
	}
	assert.Equal(t, 2, count)

	// Now do a query for a key=value pair that doesn't exist.
	out = ParamIndex(context.Background(), b.getTable(), tileKey, "unknown", "unknown", "test")

	// Confirm that we get no trace ids.
	count = 0
	for range out {
		count++
	}
	assert.Equal(t, 0, count)

	close(errCh)

	// Confirm that we get no errors.
	count = 0
	for {
		_, ok := <-errCh
		if !ok {
			break
		}
		count++
	}
	assert.Equal(t, 0, count)

}
