package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/btts_testutils"
	"go.skia.org/infra/perf/go/config"
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

func TestCloseOnCancel(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	tileKey := btts.TileKeyFromOffset(1)
	now := time.Now()
	ctx := context.Background()
	btts_testutils.CreateTestTable(t)
	defer btts_testutils.CleanUpTestTable(t)

	b, err := btts.NewBigTableTraceStoreFromConfig(ctx, cfg, &btts_testutils.MockTS{}, true)
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

	ops, err := b.GetOrderedParamSet(ctx, tileKey)
	assert.NoError(t, err)

	p, err := ops.EncodeParams(params[0])
	key := ""
	value := ""
	for k, v := range p {
		key = k
		value = v
		break
	}
	assert.NoError(t, err)
	errCh := make(chan error, 10)

	ctx, cancel := context.WithCancel(context.Background())
	out := ParamIndex(ctx, b.GetTable(), tileKey, key, value, errCh)
	cancel()
	<-out
}

func TestParamIndex(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	tileKey := btts.TileKeyFromOffset(1)
	now := time.Now()
	ctx := context.Background()
	btts_testutils.CreateTestTable(t)
	defer btts_testutils.CleanUpTestTable(t)

	b, err := btts.NewBigTableTraceStoreFromConfig(ctx, cfg, &btts_testutils.MockTS{}, true)
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

	ops, err := b.GetOrderedParamSet(ctx, tileKey)
	assert.NoError(t, err)

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

	out := ParamIndex(context.Background(), b.GetTable(), tileKey, key, value, errCh)
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
}
