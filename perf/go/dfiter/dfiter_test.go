package dfiter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dfbuilder"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracestore/sqltracestore"
	"go.skia.org/infra/perf/go/types"
)

const testTileSize = 6

var (
	defaultAnomalyConfig = config.AnomalyConfig{}
)

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
	return store.WriteTraces(context.Background(), index, params, values, ps, filename, ts)
}

func newForTest(t *testing.T) (context.Context, dataframe.DataFrameBuilder, perfgit.Git, time.Time) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, true, db, instanceConfig)
	require.NoError(t, err)

	cfg := config.DataStoreConfig{
		TileSize:      testTileSize,
		DataStoreType: config.SpannerDataStoreType,
	}

	store, err := sqltracestore.New(db, cfg)
	require.NoError(t, err)

	ts := gittest.StartTime

	// Add some points to the first and second tile.
	err = addValuesAtIndex(store, 0, map[string]float32{
		",arch=x86,config=8888,": 1.2,
		",arch=x86,config=565,":  2.1,
		",arch=arm,config=8888,": 100.5,
	}, "gs://foo.json", ts)
	assert.NoError(t, err)
	err = addValuesAtIndex(store, 1, map[string]float32{
		",arch=x86,config=8888,": 1.3,
		",arch=x86,config=565,":  2.2,
		",arch=arm,config=8888,": 100.6,
	}, "gs://foo.json", ts.Add(time.Minute))
	assert.NoError(t, err)
	err = addValuesAtIndex(store, 7, map[string]float32{
		",arch=x86,config=8888,": 1.7,
		",arch=x86,config=565,":  2.5,
		",arch=arm,config=8888,": 101.1,
	}, "gs://foo.json", ts.Add(7*time.Minute))
	assert.NoError(t, err)

	lastTimeStamp := ts.Add(8 * time.Minute)
	err = addValuesAtIndex(store, 8, map[string]float32{
		",arch=x86,config=8888,": 1.8,
		",arch=x86,config=565,":  2.6,
		",arch=arm,config=8888,": 101.2,
	}, "gs://foo.json", lastTimeStamp)
	assert.NoError(t, err)

	instanceConfig.DataStoreConfig.TileSize = testTileSize
	require.NoError(t, err)
	dfb := dfbuilder.NewDataFrameBuilderFromTraceStore(g, store, 2, false, instanceConfig.QueryConfig.CommitChunkSize, instanceConfig.QueryConfig.MaxEmptyTilesForQuery)
	return ctx, dfb, g, lastTimeStamp
}

func TestNewDataFrameIterator_MultipleDataframes_SingleFrameOfLengthThree(t *testing.T) {
	ctx, dfb, g, _ := newForTest(t)

	// This is a MultipleDataframes request because Domain.Offset = 0.

	// This request should return two frames since we only have data at four
	// commits in the entire store, and NewDataFrameIterator only produces dense
	// dataframes.
	alert := &alerts.Alert{
		Radius: 1,
	}
	domain := types.Domain{
		End:    gittest.StartTime.Add(8 * time.Minute), // Some time after the last commit.
		N:      10,
		Offset: 0,
	}
	query := "arch=x86"
	iter, err := NewDataFrameIterator(ctx, progress.New(), dfb, g, nil, query, domain, alert, defaultAnomalyConfig)
	require.NoError(t, err)
	require.True(t, iter.Next())
	df, err := iter.Value(ctx)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  types.Trace{2.1, 2.2, 2.5},
		",arch=x86,config=8888,": types.Trace{1.2, 1.3, 1.7},
	}, df.TraceSet)

	require.True(t, iter.Next())

	df, err = iter.Value(ctx)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  types.Trace{2.2, 2.5, 2.6},
		",arch=x86,config=8888,": types.Trace{1.3, 1.7, 1.8},
	}, df.TraceSet)

	require.False(t, iter.Next())
}

func TestNewDataFrameIterator_MultipleDataframes_TwoFramesOfLengthTwo(t *testing.T) {
	ctx, dfb, g, _ := newForTest(t)

	// This is a MultipleDataframes request because Domain.Offset = 0.

	// This request should only return two frames of length one since we only
	// have data at four commits in the entire store, and only three of them
	// come before the given domain.End time. NewDataFrameIterator only produces
	// dense dataframes and an Alert.Radius of 0 means the dataframe will have a
	// length of 1.
	alert := &alerts.Alert{
		Radius: 0,
	}
	domain := types.Domain{
		End:    gittest.StartTime.Add(5 * time.Minute),
		N:      2,
		Offset: 0,
	}
	query := "arch=x86"
	iter, err := NewDataFrameIterator(ctx, progress.New(), dfb, g, nil, query, domain, alert, defaultAnomalyConfig)
	require.NoError(t, err)

	require.True(t, iter.Next())
	df, err := iter.Value(ctx)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  types.Trace{2.1},
		",arch=x86,config=8888,": types.Trace{1.2},
	}, df.TraceSet)

	require.True(t, iter.Next())
	df, err = iter.Value(ctx)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  types.Trace{2.2},
		",arch=x86,config=8888,": types.Trace{1.3},
	}, df.TraceSet)
	require.False(t, iter.Next())
}

// An instance of progressCapture is used to capture Progress messages for
// testing.
type progressCapture struct {
	message string
}

// callback implements types.ProgressCallback.
func (p *progressCapture) callback(message string) {
	p.message = message
}

func TestNewDataFrameIterator_InsufficientData_ReturnsError(t *testing.T) {
	ctx, dfb, g, _ := newForTest(t)

	// This is a MultipleDataframes request because Domain.Offset = 0.

	// This request should only return an error because we ask for 11 points
	// (radius 5), and we only have 5 points in the database.
	alert := &alerts.Alert{
		Radius: 5,
	}
	domain := types.Domain{
		End:    gittest.StartTime.Add(5 * time.Minute),
		N:      2,
		Offset: 0,
	}
	query := "arch=x86"
	pc := &progressCapture{}
	_, err := NewDataFrameIterator(ctx, progress.New(), dfb, g, pc.callback, query, domain, alert, defaultAnomalyConfig)
	require.Error(t, err)
	require.Equal(t, "Query didn't return enough data points: Got 2. Want 11.", pc.message)
}

func TestNewDataFrameIterator_ExactDataframeRequest_ErrIfWeSearchAfterLastCommit(t *testing.T) {
	ctx, dfb, g, _ := newForTest(t)

	// This is an ExactDataframeRequest because Offset != 0.

	// This request should error because we start at commit 10 which doesn't
	// exist.
	alert := &alerts.Alert{
		Radius: 1,
	}
	domain := types.Domain{
		N:      2,
		Offset: 30,
	}
	q := "arch=x86"
	pc := &progressCapture{}
	_, err := NewDataFrameIterator(ctx, progress.New(), dfb, g, pc.callback, q, domain, alert, defaultAnomalyConfig)
	require.Contains(t, err.Error(), "Failed to look up CommitNumber")
	require.Equal(t, "Not a valid commit number 31. Make sure you choose a commit old enough to have 1 commits before it and 0 commits after it.", pc.message)
}

func TestNewDataFrameIterator_ExactDataframeRequest_Success(t *testing.T) {
	ctx, dfb, g, _ := newForTest(t)

	// This is an ExactDataframeRequest because Offset != 0.
	alert := &alerts.Alert{
		Radius: 1,
	}
	domain := types.Domain{
		N:      2,
		Offset: 6, // Start at 6 with a radius of 1 to get the commit at 7.
	}
	q := "arch=x86"
	iter, err := NewDataFrameIterator(ctx, progress.New(), dfb, g, nil, q, domain, alert, defaultAnomalyConfig)
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
	ctx, dfb, g, _ := newForTest(t)

	// This is an ExactDataframeRequest because Offset != 0.

	// This request should error because we start at commit -5 which doesn't
	// exist.
	alert := &alerts.Alert{
		Radius: 1,
	}
	domain := types.Domain{
		N:      2,
		Offset: -5,
	}
	q := "arch=x86"
	pc := &progressCapture{}
	_, err := NewDataFrameIterator(ctx, progress.New(), dfb, g, pc.callback, q, domain, alert, defaultAnomalyConfig)
	require.Contains(t, err.Error(), "Failed to look up CommitNumber")
	require.Equal(t, "Not a valid commit number -4. Make sure you choose a commit old enough to have 1 commits before it and 0 commits after it.", pc.message)
}

func TestNewDataFrameIterator_MultipleDataframes_ErrIfWeSearchBeforeFirstCommit(t *testing.T) {
	ctx, dfb, g, _ := newForTest(t)

	// This is a MultipleDataframes request because Domain.Offset = 0.

	// This request should error because we start at a commit time before the
	// first commit in the repo.
	alert := &alerts.Alert{
		Radius: 1,
	}
	domain := types.Domain{
		End:    gittest.StartTime.Add(-1 * time.Minute),
		N:      2,
		Offset: 0,
	}
	q := "arch=x86"
	pc := &progressCapture{}
	_, err := NewDataFrameIterator(ctx, progress.New(), dfb, g, pc.callback, q, domain, alert, defaultAnomalyConfig)
	require.Contains(t, err.Error(), "Failed to build dataframe iterator")
	require.Equal(t, "Failed querying the data due to an internal error.", pc.message)
}

func TestNewDataFrameIterator_MultipleDataframesWithSettlingTime_OneFramesOfLengthThree(t *testing.T) {
	_, dfb, g, lastTimeStamp := newForTest(t)

	// This is a MultipleDataframes request because Domain.Offset = 0.

	// This request should only return one frame of length three since we only
	// have data at four commits in the entire store, and one of them comes
	// outside of the settling time. An Alert.Radius of 1 means the dataframe
	// will have a length of 3.
	alert := &alerts.Alert{
		Radius: 1,
	}
	domain := types.Domain{
		End:    gittest.StartTime.Add(8 * time.Minute),
		N:      4,
		Offset: 0,
	}
	query := "arch=x86"
	anomalyConfig := config.AnomalyConfig{
		SettlingTime: config.DurationAsString(30 * time.Second),
	}

	ctx := now.TimeTravelingContext(lastTimeStamp)
	iter, err := NewDataFrameIterator(ctx, progress.New(), dfb, g, nil, query, domain, alert, anomalyConfig)
	require.NoError(t, err)

	require.True(t, iter.Next())
	df, err := iter.Value(ctx)
	require.NoError(t, err)
	assert.Equal(t, types.TraceSet{
		",arch=x86,config=565,":  types.Trace{2.1, 2.2, 2.5},
		",arch=x86,config=8888,": types.Trace{1.2, 1.3, 1.7},
	}, df.TraceSet)

	// Only one trace returned.
	require.False(t, iter.Next())
}
