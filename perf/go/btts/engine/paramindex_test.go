package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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

	ctx := context.Background()
	btts_testutils.CreateTestTable(t)
	defer btts_testutils.CleanUpTestTable(t)

	_, err := btts.NewBigTableTraceStoreFromConfig(ctx, cfg, &btts_testutils.MockTS{}, true)
	assert.NoError(t, err)
}
