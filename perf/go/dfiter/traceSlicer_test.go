package dfiter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

func TestNewDfTraceSlicer_EmptyDataFrame_ReturnsNoData(t *testing.T) {
	df := &dataframe.DataFrame{
		TraceSet: types.TraceSet{},
		Header:   []*dataframe.ColumnHeader{},
		ParamSet: paramtools.NewReadOnlyParamSet(),
	}

	iter := NewDfTraceSlicer(df)
	assert.False(t, iter.Next())
}

func TestNewDfTraceSlicer_SingleTrace_ReturnsOneDataFrame(t *testing.T) {
	traceID := ",config=8888,name=foo,"
	df := &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			traceID: types.Trace{1.0, 2.0},
		},
		Header: []*dataframe.ColumnHeader{
			{Offset: 1}, {Offset: 2},
		},
		ParamSet: paramtools.NewReadOnlyParamSet(),
	}

	iter := NewDfTraceSlicer(df)
	ctx := context.Background()

	assert.True(t, iter.Next())
	val, err := iter.Value(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, len(val.TraceSet))
	assert.Equal(t, types.Trace{1.0, 2.0}, val.TraceSet[traceID])
	assert.Equal(t, df.Header, val.Header)
	// Verify ParamSet is correctly rebuilt for the trace
	assert.Equal(t, []string{"8888"}, val.ParamSet["config"])
	assert.Equal(t, []string{"foo"}, val.ParamSet["name"])

	assert.False(t, iter.Next())
}

func TestNewDfTraceSlicer_MultipleTraces_ReturnsMultipleDataFrames(t *testing.T) {
	traceID1 := ",config=8888,name=foo,"
	traceID2 := ",config=565,name=bar,"
	df := &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			traceID1: types.Trace{1.0, 2.0},
			traceID2: types.Trace{3.0, 4.0},
		},
		Header: []*dataframe.ColumnHeader{
			{Offset: 1}, {Offset: 2},
		},
		ParamSet: paramtools.NewReadOnlyParamSet(),
	}

	iter := NewDfTraceSlicer(df)
	ctx := context.Background()

	// Since map iteration order is random, we collect the results and verify them.
	results := map[string]types.Trace{}
	count := 0
	for iter.Next() {
		val, err := iter.Value(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, len(val.TraceSet))
		assert.Equal(t, df.Header, val.Header)
		count++

		// Extract the only trace in the set
		for id, tr := range val.TraceSet {
			results[id] = tr
			// Verify params
			params, err := query.ParseKey(id)
			require.NoError(t, err)
			for k, v := range params {
				assert.Equal(t, []string{v}, val.ParamSet[k])
			}
		}
	}

	assert.Equal(t, 2, count)
	assert.Equal(t, types.Trace{1.0, 2.0}, results[traceID1])
	assert.Equal(t, types.Trace{3.0, 4.0}, results[traceID2])
}
