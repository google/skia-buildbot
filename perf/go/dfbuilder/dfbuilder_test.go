package dfbuilder

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/config"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracestore/sqltracestore"
	"go.skia.org/infra/perf/go/types"
)

var (
	cfg = &config.InstanceConfig{
		DataStoreConfig: config.DataStoreConfig{
			TileSize: 256,
			Project:  "test",
			Instance: "test",
			Table:    "test",
			Shards:   8,
		},
	}
)

func TestBuildTraceMapper(t *testing.T) {
	unittest.LargeTest(t)

	db, cleanup := sqltest.NewSQLite3DBForTests(t)
	defer cleanup()

	store, err := sqltracestore.New(db, perfsql.SQLiteDialect, 6)
	require.NoError(t, err)

	tileMap := buildTileMapOffsetToIndex([]types.CommitNumber{0, 1, 255, 256, 257}, store)
	expected := tileMapOffsetToIndex{0: map[int32]int32{0: 0, 1: 1, 255: 2}, 1: map[int32]int32{0: 3, 1: 4}}
	assert.Equal(t, expected, tileMap)

	tileMap = buildTileMapOffsetToIndex([]types.CommitNumber{}, store)
	expected = tileMapOffsetToIndex{}
	assert.Equal(t, expected, tileMap)
}

// The keys of values are structured keys, not encoded keys.
func addValuesAtIndex(store tracestore.TraceStore, index types.CommitNumber, keyValues map[string]float32, filename string, ts time.Time) error {
	ps := paramtools.ParamSet{}
	params := []paramtools.Params{}
	values := []float32{}
	for k, v := range keyValues {
		p, err := query.ParseKey(k)
		if err != nil {
			return err
		}
		ps.AddParams(p)
		params = append(params, p)
		values = append(values, v)
	}
	return store.WriteTraces(index, params, values, ps, filename, ts)
}

func TestBuildNew(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()

	ctx, db, _, _, dialect, instanceConfig, cleanup := gittest.NewForTest(t, perfsql.SQLiteDialect)
	defer cleanup()
	g, err := perfgit.New(ctx, true, db, dialect, instanceConfig)
	require.NoError(t, err)

	instanceConfig.DataStoreConfig.TileSize = 6

	store, err := sqltracestore.New(db, perfsql.SQLiteDialect, instanceConfig.DataStoreConfig.TileSize)
	require.NoError(t, err)

	builder := NewDataFrameBuilderFromTraceStore(g, store)

	// Add some points to the first and second tile.
	err = addValuesAtIndex(store, 0, map[string]float32{
		",arch=x86,config=8888,": 1.2,
		",arch=x86,config=565,":  2.1,
		",arch=arm,config=8888,": 100.5,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)
	err = addValuesAtIndex(store, 1, map[string]float32{
		",arch=x86,config=8888,": 1.3,
		",arch=x86,config=565,":  2.2,
		",arch=arm,config=8888,": 100.6,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)
	err = addValuesAtIndex(store, 7, map[string]float32{
		",arch=x86,config=8888,": 1.0,
		",arch=x86,config=565,":  2.5,
		",arch=arm,config=8888,": 101.1,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)

	// NewFromQueryAndRange
	q, err := query.New(url.Values{"config": []string{"8888"}})
	assert.NoError(t, err)
	now := gittest.StartTime.Add(7 * time.Minute)

	df, err := builder.NewFromQueryAndRange(ctx, now.Add(-7*time.Minute), now.Add(time.Second), q, false, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 2)
	assert.Len(t, df.Header, 8)
	assert.Len(t, df.TraceSet[",arch=x86,config=8888,"], 8)
	assert.Len(t, df.TraceSet[",arch=arm,config=8888,"], 8)

	// A dense response from NewNFromQuery().
	df, err = builder.NewNFromQuery(ctx, now, q, 4, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 2)
	assert.Len(t, df.Header, 3)
	assert.Equal(t, df.Header[0].Offset, int64(0))
	assert.Equal(t, df.Header[1].Offset, int64(1))
	assert.Equal(t, df.Header[2].Offset, int64(7))
	assert.Equal(t, df.TraceSet[",arch=x86,config=8888,"][0], float32(1.2))
	assert.Equal(t, df.TraceSet[",arch=x86,config=8888,"][1], float32(1.3))
	assert.Equal(t, df.TraceSet[",arch=x86,config=8888,"][2], float32(1.0))

	df, err = builder.NewNFromQuery(ctx, now, q, 2, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 2)
	assert.Len(t, df.Header, 2)
	assert.Equal(t, df.Header[1].Offset, int64(7))
	assert.Equal(t, df.TraceSet[",arch=x86,config=8888,"][1], float32(1.0))

	// NewFromQueryAndRange where query doesn't encode.
	q, err = query.New(url.Values{"config": []string{"nvpr"}})
	assert.NoError(t, err)

	df, err = builder.NewFromQueryAndRange(ctx, now.Add(-7*time.Minute), now.Add(time.Second), q, false, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 0)
	assert.Len(t, df.Header, 8)

	// NewFromKeysAndRange.
	df, err = builder.NewFromKeysAndRange(ctx, []string{",arch=x86,config=8888,", ",arch=x86,config=565,"}, now.Add(-7*time.Minute), now.Add(time.Second), false, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 2)
	assert.Len(t, df.Header, 8)
	assert.Len(t, df.ParamSet, 2)
	assert.Len(t, df.TraceSet[",arch=x86,config=8888,"], 8)
	assert.Len(t, df.TraceSet[",arch=x86,config=565,"], 8)

	// NewNFromKeys.
	df, err = builder.NewNFromKeys(ctx, now, []string{",arch=x86,config=8888,", ",arch=x86,config=565,"}, 2, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 2)
	assert.Len(t, df.Header, 2)
	assert.Len(t, df.ParamSet, 2)
	assert.Len(t, df.TraceSet[",arch=x86,config=8888,"], 2)
	assert.Len(t, df.TraceSet[",arch=x86,config=565,"], 2)

	df, err = builder.NewNFromKeys(ctx, now, []string{",arch=x86,config=8888,", ",arch=x86,config=565,"}, 3, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 2)
	assert.Len(t, df.Header, 3)
	assert.Len(t, df.TraceSet[",arch=x86,config=8888,"], 3)
	assert.Len(t, df.TraceSet[",arch=x86,config=565,"], 3)

	df, err = builder.NewNFromKeys(ctx, now, []string{",arch=x86,config=8888,"}, 3, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 1)
	assert.Len(t, df.Header, 3)
	assert.Len(t, df.TraceSet[",arch=x86,config=8888,"], 3)

	df, err = builder.NewNFromKeys(ctx, now, []string{}, 3, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 0)
	assert.Len(t, df.Header, 0)

	// Empty set of keys should not fail.
	df, err = builder.NewFromKeysAndRange(ctx, []string{}, now.Add(-7*time.Minute), now.Add(time.Second), false, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 0)
	assert.Len(t, df.Header, 8)

	// Add a value that only appears in one of the tiles.
	err = addValuesAtIndex(store, 7, map[string]float32{
		",config=8888,model=Pixel,": 3.0,
	}, "gs://foo.json", time.Now())
	assert.NoError(t, err)

	// This query will only encode for one tile and should still succeed.
	q, err = query.New(url.Values{"model": []string{"Pixel"}})
	assert.NoError(t, err)
	df, err = builder.NewFromQueryAndRange(ctx, now.Add(-7*time.Minute), now.Add(time.Second), q, false, nil)
	assert.NoError(t, err)
	assert.Len(t, df.TraceSet, 1)
	assert.Len(t, df.Header, 8)
}

// mockVCS is a mock vcsinfo.VCS that implements just LastNIndex and Range, the
// only two func's that dfbuilder.builder uses.
type mockVCS struct {
	ret []*vcsinfo.IndexCommit
}

func (m *mockVCS) GetBranch() string { return "master" }

func (m *mockVCS) LastNIndex(N int) []*vcsinfo.IndexCommit {
	if N > len(m.ret)-1 {
		return m.ret
	}
	return m.ret[len(m.ret)-N:]
}

func (m *mockVCS) Range(begin time.Time, end time.Time) []*vcsinfo.IndexCommit {
	return m.ret
}

func (m *mockVCS) ByIndex(ctx context.Context, N int) (*vcsinfo.LongCommit, error) {
	if N >= len(m.ret) || N < 0 {
		return nil, fmt.Errorf("Index out of range.")
	}
	c := m.ret[N]
	ret := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash: c.Hash,
		},
		Timestamp: c.Timestamp,
	}
	return ret, nil
}

func (m *mockVCS) Update(ctx context.Context, pull bool, allBranches bool) error { return nil }
func (m *mockVCS) From(start time.Time) []string {
	ret := []string{}
	for _, c := range m.ret {
		if c.Timestamp.After(start) {
			ret = append(ret, c.Hash)
		}
	}

	return ret
}
func (m *mockVCS) Details(ctx context.Context, hash string, includeBranchInfo bool) (*vcsinfo.LongCommit, error) {
	return nil, nil
}
func (m *mockVCS) DetailsMulti(ctx context.Context, hashes []string, includeBranchInfo bool) ([]*vcsinfo.LongCommit, error) {
	return nil, nil
}
func (m *mockVCS) IndexOf(ctx context.Context, hash string) (int, error) {
	for i, c := range m.ret {
		if c.Hash == hash {
			return i, nil
		}
	}

	return 0, fmt.Errorf("Not found")
}
func (m *mockVCS) GetFile(ctx context.Context, fileName string, commitHash string) (string, error) {
	return "", nil
}
