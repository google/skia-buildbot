package btts

import (
	"context"
	"math"
	"testing"

	"cloud.google.com/go/bigtable"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"golang.org/x/oauth2"
)

type MockTS struct{}

func (t *MockTS) Token() (*oauth2.Token, error) {
	return nil, nil
}

func createTestTable() {
	ctx := context.Background()
	client, _ := bigtable.NewAdminClient(ctx, "test", "test")
	client.CreateTableFromConf(ctx, &bigtable.TableConf{
		TableID: "skia",
		Families: map[string]bigtable.GCPolicy{
			"V": bigtable.MaxVersionsPolicy(1),
			"S": bigtable.MaxVersionsPolicy(1),
			"D": bigtable.MaxVersionsPolicy(1),
		},
	})
}

func cleanUpTestTable() {
	ctx := context.Background()
	client, _ := bigtable.NewAdminClient(ctx, "test", "test")
	client.DeleteTable(ctx, "skia")
}

func TestBasic(t *testing.T) {
	testutils.LargeTest(t)
	ctx := context.Background()
	createTestTable()
	defer cleanUpTestTable()

	b, err := NewBigTableTraceStore(ctx, 256, "skia", "test", "test", &MockTS{})
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
	b2, err := NewBigTableTraceStore(ctx, 256, "skia", "test", "test", &MockTS{})
	assert.NoError(t, err)

	// OPS for tile 4 should be a no-op since it's already in BT.
	op2, err := b2.UpdateOrderedParamSet(tileKey2, paramtools.ParamSet{
		// Note we reverse "linux", "win" order, but still get the same
		// result as op.
		"os": []string{"linux", "win"},
	})
	assert.Equal(t, op, op2)
}

func TestTileKey(t *testing.T) {
	testutils.SmallTest(t)

	tileKey := TileKeyFromOffset(0)
	assert.Equal(t, int32(math.MaxInt32), int32(tileKey))
	assert.Equal(t, int32(0), tileKey.Offset())
	assert.Equal(t, "@2147483647", tileKey.OpsRowName())
	assert.Equal(t, "2147483647:", tileKey.TraceRowPrefix())
	assert.Equal(t, "2147483647:,0=1,", tileKey.TraceRowName(",0=1,"))

	tileKey = TileKeyFromOffset(1)
	assert.Equal(t, int32(math.MaxInt32-1), int32(tileKey))
	assert.Equal(t, "@2147483646", tileKey.OpsRowName())
	assert.Equal(t, "2147483646:", tileKey.TraceRowPrefix())
	assert.Equal(t, "2147483646:,0=1,", tileKey.TraceRowName(",0=1,"))

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
