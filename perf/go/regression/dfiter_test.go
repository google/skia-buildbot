package regression

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dfbuilder"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	perfsql "go.skia.org/infra/perf/go/sql"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracestore/sqltracestore"
	"go.skia.org/infra/perf/go/types"
)

const testTileSize = 6

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

type cleanupFunc func()

func newForTest(t *testing.T) (context.Context, dataframe.DataFrameBuilder, *perfgit.Git, cleanupFunc) {
	db, dbCleanup := sqltest.NewSQLite3DBForTests(t)

	store, err := sqltracestore.New(db, perfsql.SQLiteDialect, testTileSize)
	require.NoError(t, err)

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

	ctx, db, _, _, dialect, instanceConfig, gitCleanup := gittest.NewForTest(t, perfsql.SQLiteDialect)
	instanceConfig.DataStoreConfig.TileSize = testTileSize
	g, err := perfgit.New(ctx, true, db, dialect, instanceConfig)
	require.NoError(t, err)
	dfb := dfbuilder.NewDataFrameBuilderFromTraceStore(g, store)
	cleanup := func() {
		gitCleanup()
		dbCleanup()
	}

	return ctx, dfb, g, cleanup
}

func TestNewDataFrameIterator(t *testing.T) {
	ctx, dfb, g, cleanup := newForTest(t)
	defer cleanup()
	request := &RegressionDetectionRequest{
		Alert: &alerts.Alert{
			Radius: 1,
		},
		Domain: types.Domain{
			End:    gittest.StartTime.Add(8 * time.Minute),
			N:      3,
			Offset: 0,
		},
		Query: "arch=x86",
	}
	iter, err := NewDataFrameIterator(ctx, nil, request, dfb, g)
	require.NoError(t, err)
	for iter.Next() {
		df, err := iter.Value(ctx)
		require.NoError(t, err)
		assert.Equal(t, types.TraceSet{
			",arch=x86,config=565,":  types.Trace{2.1, 2.2, 2.5},
			",arch=x86,config=8888,": types.Trace{1.2, 1.3, 1},
		}, df.TraceSet)
	}
}
