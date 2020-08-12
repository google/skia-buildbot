package btts

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/tracestore/btts/btts_testutils"
	"go.skia.org/infra/perf/go/types"
)

func TestValidate(t *testing.T) {
	unittest.SmallTest(t)

	// Test that a normal plan validates.
	plan := paramtools.ParamSet{
		"foo": []string{"bar"},
	}
	err := validatePlan(plan)
	assert.NoError(t, err)

	// Test that a huge plan is rejected.
	for i := 0; i < maxParallelParamIndex+1; i++ {
		plan[fmt.Sprintf("x%d", i)] = []string{"bar"}
	}
	err = validatePlan(plan)
	assert.Error(t, err)
}
func TestExecuteCancel(t *testing.T) {
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

	ops, err := b.GetOrderedParamSet(ctx, tileNumber)
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
	plan := paramtools.ParamSet{
		key: []string{value},
	}

	// Create a cancellable context.
	ctx, cancel := context.WithCancel(context.Background())

	// Start the querying.
	out, err := ExecutePlan(ctx, plan, b.getTable(), tileKey, "test")
	assert.NoError(t, err)

	// Cancel the context which should abort the BigTable read.
	cancel()

	// Confirm that the channel is closed.
	_, ok := <-out
	assert.False(t, ok)
}

func TestExecuteGoodQuery(t *testing.T) {
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
		"cpu":    []string{"x86", "arm", "risc-v"},
	}
	assert.NoError(t, err)
	params := []paramtools.Params{
		{"cpu": "x86", "config": "8888"},
		{"cpu": "x86", "config": "565"},
		{"cpu": "arm", "config": "8888"},
		{"cpu": "arm", "config": "565"},
		{"cpu": "risc-v", "config": "gles"},
	}
	values := []float32{
		1.0,
		1.1,
		1.2,
		1.3,
	}
	err = b.WriteTraces(257, params, values, paramset, "gs://some/test/location", now)
	assert.NoError(t, err)

	ops, err := b.GetOrderedParamSet(ctx, tileNumber)
	assert.NoError(t, err)

	// Now that the Tile is populated construct an encoded paramset to use as a
	// query. We'll try to match the first trace in params.
	p, err := ops.EncodeParams(params[0])
	assert.NoError(t, err)
	plan := paramtools.NewParamSet(p)

	// Start the querying.
	out, err := ExecutePlan(context.Background(), plan, b.getTable(), tileKey, "test")
	assert.NoError(t, err)

	expected := []paramtools.Params{
		{"cpu": "x86", "config": "8888"},
	}
	assertExpectedTraces(t, expected, ops, out)

	// Confirm that the channel is closed.
	_, ok := <-out
	assert.False(t, ok)

	// Now do another query with just a single key=value.
	p, err = ops.EncodeParams(paramtools.Params{"config": "8888"})
	assert.NoError(t, err)
	plan = paramtools.NewParamSet(p)

	// Start the querying.
	out, err = ExecutePlan(context.Background(), plan, b.getTable(), tileKey, "test")
	assert.NoError(t, err)

	expected = []paramtools.Params{
		{"cpu": "x86", "config": "8888"},
		{"cpu": "arm", "config": "8888"},
	}
	assertExpectedTraces(t, expected, ops, out)

	// Confirm that the channel is closed.
	_, ok = <-out
	assert.False(t, ok)

	// Now do another query with that will miss all traces.
	p, err = ops.EncodeParams(paramtools.Params{"cpu": "risc-v", "config": "8888"})
	assert.NoError(t, err)
	plan = paramtools.NewParamSet(p)

	// Start the querying.
	out, err = ExecutePlan(context.Background(), plan, b.getTable(), tileKey, "test")
	assert.NoError(t, err)

	expected = []paramtools.Params{}
	assertExpectedTraces(t, expected, ops, out)

	// Confirm that the channel is closed.
	_, ok = <-out
	assert.False(t, ok)
}

func assertExpectedTraces(t *testing.T, expected []paramtools.Params, ops *paramtools.OrderedParamSet, out <-chan string) {
	count := 0
	for {
		encodedKey, ok := <-out
		if !ok {
			break
		}
		dp, err := ops.DecodeParamsFromString(encodedKey)
		assert.NoError(t, err)
		assert.Contains(t, expected, dp)
		count++
	}
	assert.Equal(t, len(expected), count)
}
