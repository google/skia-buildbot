package dfbuilder

import (
	"context"
	"net/url"
	"testing"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"golang.org/x/oauth2"
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

	store, err := btts.NewBigTableTraceStoreFromConfig(ctx, cfg, &MockTS{}, true)
	assert.NoError(t, err)

	tileMap := buildTileMapOffsetToIndex([]int32{0, 1, 255, 256, 257}, store)
	expected := tileMapOffsetToIndex{2147483647: map[int32]int32{0: 0, 1: 1, 255: 2}, 2147483646: map[int32]int32{0: 3, 1: 4}}
	assert.Equal(t, expected, tileMap)

	tileMap = buildTileMapOffsetToIndex([]int32{}, store)
	expected = tileMapOffsetToIndex{}
	assert.Equal(t, expected, tileMap)
}

// The keys of values are structured keys, not encoded keys.
func addValusAtIndex(store *btts.BigTableTraceStore, index int32, values map[string]float32, filename string, ts time.Time) error {
	tileKey := store.TileKey(index)
	ps := paramtools.ParamSet{}
	for structuredKey, _ := range values {
		p, err := query.ParseKey(structuredKey)
		if err != nil {
			return err
		}
		ps.AddParams(p)
	}
	ops, err := store.UpdateOrderedParamSet(tileKey, ps)
	if err != nil {
		return err
	}
	encoded := map[string]float32{}
	for structuredKey, value := range values {
		p, err := query.ParseKey(structuredKey)
		if err != nil {
			return err
		}
		encodedKey, err := ops.EncodeParamsAsString(p)
		if err != nil {
			return err
		}
		encoded[encodedKey] = value
	}

	return store.WriteTraces(index, encoded, filename, ts)
}

func TestBuildNew(t *testing.T) {
	testutils.LargeTest(t)
	ctx := context.Background()
	createTestTable(t)
	defer cleanUpTestTable(t)

	cfg := &config.PerfBigTableConfig{
		TileSize:     6,
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
	// Should not fail on an empty table.
	store, err := btts.NewBigTableTraceStoreFromConfig(ctx, cfg, &MockTS{}, false)
	assert.NoError(t, err)
	v := &mockVCS{
		ret: []*vcsinfo.IndexCommit{
			&vcsinfo.IndexCommit{Index: 0},
			&vcsinfo.IndexCommit{Index: 1},
			&vcsinfo.IndexCommit{Index: 2},
			&vcsinfo.IndexCommit{Index: 3},
			&vcsinfo.IndexCommit{Index: 4},
			&vcsinfo.IndexCommit{Index: 5},
			&vcsinfo.IndexCommit{Index: 6},
			&vcsinfo.IndexCommit{Index: 7},
		},
	}
	builder := NewDataFrameBuilderFromBTTS(v, store)
	df, err := builder.New(nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 0)
	assert.Len(t, df.Header, 8)
	assert.Len(t, df.ParamSet, 0)
	assert.Equal(t, 0, df.Skip)

	// Add some points to the first and second tile.
	err = addValusAtIndex(store, 0, map[string]float32{
		",arch=x86,config=8888,": 1.2,
		",arch=x86,config=565,":  2.1,
		",arch=arm,config=8888,": 100.5,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)
	err = addValusAtIndex(store, 1, map[string]float32{
		",arch=x86,config=8888,": 1.3,
		",arch=x86,config=565,":  2.2,
		",arch=arm,config=8888,": 100.6,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)
	err = addValusAtIndex(store, 7, map[string]float32{
		",arch=x86,config=8888,": 1.0,
		",arch=x86,config=565,":  2.5,
		",arch=arm,config=8888,": 101.1,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)

	// Load those points.
	df, err = builder.New(nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 3)
	assert.Len(t, df.Header, 8)
	assert.Len(t, df.TraceSet[",arch=x86,config=8888,"], 8)
	assert.Equal(t, float32(1.0), df.TraceSet[",arch=x86,config=8888,"][7])

	// Load last N points.
	df, err = builder.NewN(nil, 2)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 3)
	assert.Len(t, df.Header, 2)
	assert.Len(t, df.TraceSet[",arch=x86,config=8888,"], 2)
	assert.Equal(t, float32(1.0), df.TraceSet[",arch=x86,config=8888,"][1])
	assert.Equal(t, vec32.MISSING_DATA_SENTINEL, df.TraceSet[",arch=x86,config=8888,"][0])

	// NewFromQueryAndRange
	q, err := query.New(url.Values{"config": []string{"8888"}})
	assert.NoError(t, err)
	now := time.Now()

	df, err = builder.NewFromQueryAndRange(now, now, q, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 2)
	assert.Len(t, df.Header, 8)
	assert.Len(t, df.TraceSet[",arch=x86,config=8888,"], 8)
	assert.Len(t, df.TraceSet[",arch=arm,config=8888,"], 8)

	// NewFromQueryAndRange where query doesn't encode.
	q, err = query.New(url.Values{"config": []string{"nvpr"}})
	assert.NoError(t, err)

	df, err = builder.NewFromQueryAndRange(now, now, q, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 0)
	assert.Len(t, df.Header, 8)

	// NewFromKeysAndRange.
	df, err = builder.NewFromKeysAndRange([]string{",arch=x86,config=8888,", ",arch=x86,config=565,"}, now, now, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 2)
	assert.Len(t, df.Header, 8)
	assert.Len(t, df.TraceSet[",arch=x86,config=8888,"], 8)
	assert.Len(t, df.TraceSet[",arch=x86,config=565,"], 8)

	// Empty set of keys should not fail.
	df, err = builder.NewFromKeysAndRange([]string{}, now, now, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 0)
	assert.Len(t, df.Header, 8)

	// Add a value that only appears in one of the tiles.
	err = addValusAtIndex(store, 7, map[string]float32{
		",arch=riscv,config=8888,": 3.0,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)

	// This query will only encode for one tile and should still succeed.
	df, err = builder.NewFromKeysAndRange([]string{",arch=riscv,"}, now, now, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 0)
	assert.Len(t, df.Header, 8)
}

// mockVCS is a mock vcsinfo.VCS that implements just LastNIndex and Range, the
// only two func's that dfbuilder.builder uses.
type mockVCS struct {
	ret []*vcsinfo.IndexCommit
}

func (m *mockVCS) LastNIndex(N int) []*vcsinfo.IndexCommit {
	if N > len(m.ret) {
		return m.ret
	}
	return m.ret[len(m.ret)-N:]
}

func (m *mockVCS) Range(begin time.Time, end time.Time) []*vcsinfo.IndexCommit {
	return m.ret
}

func (m *mockVCS) Update(ctx context.Context, pull bool, allBranches bool) error { return nil }
func (m *mockVCS) From(start time.Time) []string                                 { return []string{} }
func (m *mockVCS) Details(ctx context.Context, hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	return nil, nil
}
func (m *mockVCS) IndexOf(ctx context.Context, hash string) (int, error)           { return 0, nil }
func (m *mockVCS) ByIndex(ctx context.Context, N int) (*vcsinfo.LongCommit, error) { return nil, nil }
func (m *mockVCS) GetFile(ctx context.Context, fileName string, commitHash string) (string, error) {
	return "", nil
}
func (m *mockVCS) ResolveCommit(ctx context.Context, commitHash string) (string, error) {
	return "", nil
}
