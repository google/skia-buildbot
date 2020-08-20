package regression

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dfbuilder"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracestore/sqltracestore"
	"go.skia.org/infra/perf/go/types"
)

const testTileSize = 6

const CockroachDatabaseName = "dfiter"

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
	db, dbCleanup := sqltest.NewCockroachDBForTests(t, CockroachDatabaseName)

	cfg := config.DataStoreConfig{
		TileSize: testTileSize,
	}

	store, err := sqltracestore.New(db, cfg)
	require.NoError(t, err)

	// Add some points to the first and second tile.
	err = addValuesAtIndex(store, 0, map[string]float32{
		",arch=x86,config=8888,": 1.2,
		",arch=x86,config=565,":  2.1,
		",arch=arm,config=8888,": 100.5,
	}, "gs://foo.json", time.Now()) // Time is irrelevent.
	assert.NoError(t, err)
	err = addValuesAtIndex(store, 1, map[string]float32{
		",arch=x86,config=8888,": 1.3,
		",arch=x86,config=565,":  2.2,
		",arch=arm,config=8888,": 100.6,
	}, "gs://foo.json", time.Now()) // Time is irrelevent.
	assert.NoError(t, err)
	err = addValuesAtIndex(store, 7, map[string]float32{
		",arch=x86,config=8888,": 1.0,
		",arch=x86,config=565,":  2.5,
		",arch=arm,config=8888,": 101.1,
	}, "gs://foo.json", time.Now()) // Time is irrelevent.
	assert.NoError(t, err)

	ctx, db, _, _, instanceConfig, _, gitCleanup := gittest.NewForTest(t)
	instanceConfig.DataStoreConfig.TileSize = testTileSize
	g, err := perfgit.New(ctx, true, db, instanceConfig)
	require.NoError(t, err)
	dfb := dfbuilder.NewDataFrameBuilderFromTraceStore(g, store)
	cleanup := func() {
		gitCleanup()
		dbCleanup()
	}

	return ctx, dfb, g, cleanup
}

func TestNewDataFrameIterator_MultipleDataframes_SingleFrameOfLengthThree(t *testing.T) {
	unittest.LargeTest(t)
	ctx, dfb, g, cleanup := newForTest(t)
	defer cleanup()

	// This is a MultipleDataframes request because Domain.Offset = 0.

	// This request should only return one frame since we only have data at
	// three commits in the entire store, and NewDataFrameIterator only produces
	// dense dataframes.
	request := &RegressionDetectionRequest{
		Alert: &alerts.Alert{
			Radius: 1,
		},
		Domain: types.Domain{
			End:    gittest.StartTime.Add(8 * time.Minute), // Some time after the last commit.
			N:      10,
			Offset: 0,
		},
		Query: "arch=x86",
	}
	iter, err := NewDataFrameIterator(ctx, nil, request, dfb, g, nil)
	require.NoError(t, err)
	require.True(t, iter.Next())
	df, err := iter.Value(ctx)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  types.Trace{2.1, 2.2, 2.5},
		",arch=x86,config=8888,": types.Trace{1.2, 1.3, 1},
	}, df.TraceSet)
	require.False(t, iter.Next())
}

func TestNewDataFrameIterator_MultipleDataframes_TwoFramesOfLengthTwo(t *testing.T) {
	unittest.LargeTest(t)
	ctx, dfb, g, cleanup := newForTest(t)
	defer cleanup()

	// This is a MultipleDataframes request because Domain.Offset = 0.

	// This request should only return two frames of length one since we only
	// have data at three commits in the entire store, and NewDataFrameIterator
	// only produces dense dataframes and an Alert.Radius of 0 means the
	// dataframe will have a length of 1.
	request := &RegressionDetectionRequest{
		Alert: &alerts.Alert{
			Radius: 0,
		},
		Domain: types.Domain{
			End:    gittest.StartTime.Add(8 * time.Minute),
			N:      2,
			Offset: 0,
		},
		Query: "arch=x86",
	}
	iter, err := NewDataFrameIterator(ctx, nil, request, dfb, g, nil)
	require.NoError(t, err)

	require.True(t, iter.Next())
	df, err := iter.Value(ctx)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  types.Trace{2.2},
		",arch=x86,config=8888,": types.Trace{1.3},
	}, df.TraceSet)

	require.True(t, iter.Next())
	df, err = iter.Value(ctx)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  types.Trace{2.5},
		",arch=x86,config=8888,": types.Trace{1},
	}, df.TraceSet)
	require.False(t, iter.Next())
}

func TestNewDataFrameIterator_ExactDataframeRequest_ErrIfWeSearchAfterLastCommit(t *testing.T) {
	unittest.LargeTest(t)
	ctx, dfb, g, cleanup := newForTest(t)
	defer cleanup()

	// This is an ExactDataframeRequest because Offset != 0.

	// This request should error because we start at commit 10 which doesn't
	// exist.
	request := &RegressionDetectionRequest{
		Alert: &alerts.Alert{
			Radius: 1,
		},
		Domain: types.Domain{
			N:      2,
			Offset: 10,
		},
		Query: "arch=x86",
	}
	_, err := NewDataFrameIterator(ctx, nil, request, dfb, g, nil)
	require.Contains(t, err.Error(), "Failed to look up CommitNumber")
}

func TestNewDataFrameIterator_ExactDataframeRequest_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, dfb, g, cleanup := newForTest(t)
	defer cleanup()

	// This is an ExactDataframeRequest because Offset != 0.
	request := &RegressionDetectionRequest{
		Alert: &alerts.Alert{
			Radius: 1,
		},
		Domain: types.Domain{
			N:      2,
			Offset: 6, // Start at 6 with a radius of 1 to get the commit at 7.
		},
		Query: "arch=x86",
	}
	iter, err := NewDataFrameIterator(ctx, nil, request, dfb, g, nil)
	require.NoError(t, err)
	require.True(t, iter.Next())
	df, err := iter.Value(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, len(df.Header))
	assert.Equal(t, types.CommitNumber(0), df.Header[0].Offset)
	assert.Equal(t, types.CommitNumber(1), df.Header[1].Offset)
	assert.Equal(t, types.CommitNumber(7), df.Header[2].Offset)
}

func TestNewDataFrameIterator_ExactDataframeRequest_ErrIfWeSearchBeforeFirstCommit(t *testing.T) {
	unittest.LargeTest(t)
	ctx, dfb, g, cleanup := newForTest(t)
	defer cleanup()

	// This is an ExactDataframeRequest because Offset != 0.

	// This request should error because we start at commit -5 which doesn't
	// exist.
	request := &RegressionDetectionRequest{
		Alert: &alerts.Alert{
			Radius: 1,
		},
		Domain: types.Domain{
			N:      2,
			Offset: -5,
		},
		Query: "arch=x86",
	}
	_, err := NewDataFrameIterator(ctx, nil, request, dfb, g, nil)
	require.Contains(t, err.Error(), "Failed to look up CommitNumber")
}

func TestNewDataFrameIterator_MultipleDataframes_ErrIfWeSearchBeforeFirstCommit(t *testing.T) {
	unittest.LargeTest(t)
	ctx, dfb, g, cleanup := newForTest(t)
	defer cleanup()

	// This is a MultipleDataframes request because Domain.Offset = 0.

	// This request should error because we start at a commit time before the
	// first commit in the repo.
	request := &RegressionDetectionRequest{
		Alert: &alerts.Alert{
			Radius: 1,
		},
		Domain: types.Domain{
			End:    gittest.StartTime.Add(-1 * time.Minute),
			N:      2,
			Offset: 0,
		},
		Query: "arch=x86",
	}
	_, err := NewDataFrameIterator(ctx, nil, request, dfb, g, nil)
	require.Contains(t, err.Error(), "Failed to build dataframe iterator")
}
