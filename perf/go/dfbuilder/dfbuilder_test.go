package dfbuilder

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"golang.org/x/oauth2"
)

var (
	cfg = &config.IngesterConfig{
		TileSize:     256,
		Project:      "test",
		Instance:     "test",
		Table:        "test",
		Topic:        "",
		GitUrl:       "",
		Subscription: "",
		Bucket:       "",
		RootDir:      "",
		Shards:       8,
	}
)

type MockTS struct{}

func (t *MockTS) Token() (*oauth2.Token, error) {
	return nil, nil
}

func createTestTable(t *testing.T) {
	ctx := context.Background()
	client, _ := bigtable.NewAdminClient(ctx, "test", "test")
	err := client.CreateTableFromConf(ctx, &bigtable.TableConf{
		TableID: "test",
		Families: map[string]bigtable.GCPolicy{
			"V": bigtable.MaxVersionsPolicy(1),
			"S": bigtable.MaxVersionsPolicy(1),
			"D": bigtable.MaxVersionsPolicy(1),
			"H": bigtable.MaxVersionsPolicy(1),
		},
	})
	assert.NoError(t, err)
}

func cleanUpTestTable(t *testing.T) {
	ctx := context.Background()
	client, _ := bigtable.NewAdminClient(ctx, "test", "test")
	err := client.DeleteTable(ctx, "test")
	assert.NoError(t, err)
}

func TestFromIndexCommit(t *testing.T) {
	testutils.SmallTest(t)

	ts0 := time.Unix(1406721642, 0).UTC()
	ts1 := time.Unix(1406721715, 0).UTC()

	commits := []*vcsinfo.IndexCommit{
		{
			Hash:      "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f",
			Index:     0,
			Timestamp: ts0,
		},
		{
			Hash:      "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18",
			Index:     1,
			Timestamp: ts1,
		},
	}
	expected_headers := []*dataframe.ColumnHeader{
		{
			Source:    "master",
			Offset:    0,
			Timestamp: ts0.Unix(),
		},
		{
			Source:    "master",
			Offset:    1,
			Timestamp: ts1.Unix(),
		},
	}
	expected_indices := []int32{0, 1}

	headers, pcommits, _ := fromIndexCommit(commits, 0)
	assert.Equal(t, 2, len(headers))
	assert.Equal(t, 2, len(pcommits))
	deepequal.AssertDeepEqual(t, expected_headers, headers)
	deepequal.AssertDeepEqual(t, expected_indices, pcommits)

	headers, pcommits, _ = fromIndexCommit([]*vcsinfo.IndexCommit{}, 0)
	assert.Equal(t, 0, len(headers))
	assert.Equal(t, 0, len(pcommits))
}

func TestBuildTraceMapper(t *testing.T) {
	testutils.LargeTest(t)
	ctx := context.Background()
	createTestTable(t)
	defer cleanUpTestTable(t)

	store, err := btts.NewBigTableTraceStoreFromConfig(ctx, cfg, &MockTS{})
	assert.NoError(t, err)

	tileMap := buildTraceMapper([]int32{0, 1, 255, 256, 257}, store)
	expected := TileMap{2147483647: map[int32]int32{0: 0, 1: 1, 255: 2}, 2147483646: map[int32]int32{0: 3, 1: 4}}
	assert.Equal(t, expected, tileMap)

	tileMap = buildTraceMapper([]int32{}, store)
	expected = TileMap{}
	assert.Equal(t, expected, tileMap)
}
