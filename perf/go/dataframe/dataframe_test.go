package dataframe

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/types"
)

func TestBuildParamSet(t *testing.T) {
	unittest.SmallTest(t)
	// Test the empty case first.
	df := &DataFrame{
		TraceSet: types.TraceSet{},
		ParamSet: paramtools.NewReadOnlyParamSet(),
	}
	df.BuildParamSet()
	assert.Equal(t, 0, len(df.ParamSet))

	df = &DataFrame{
		TraceSet: types.TraceSet{
			",arch=x86,config=565,":  types.Trace([]float32{1.2, 2.1}),
			",arch=x86,config=8888,": types.Trace([]float32{1.3, 3.1}),
			",arch=x86,config=gpu,":  types.Trace([]float32{1.4, 4.1}),
		},
		ParamSet: paramtools.NewReadOnlyParamSet(),
	}
	df.BuildParamSet()
	assert.Equal(t, 2, len(df.ParamSet))
	values, ok := df.ParamSet["arch"]
	assert.True(t, ok)
	assert.Equal(t, []string{"x86"}, values)
	values, ok = df.ParamSet["config"]
	assert.True(t, ok)
	assert.Equal(t, []string{"565", "8888", "gpu"}, values)
}

func TestFilter(t *testing.T) {
	unittest.SmallTest(t)
	df := &DataFrame{
		TraceSet: types.TraceSet{
			",arch=x86,config=565,":  types.Trace([]float32{1.2, 2.1}),
			",arch=x86,config=8888,": types.Trace([]float32{1.3, 3.1}),
			",arch=x86,config=gpu,":  types.Trace([]float32{1.4, 4.1}),
		},
		ParamSet: paramtools.NewReadOnlyParamSet(),
	}
	f := func(tr types.Trace) bool {
		return tr[0] > 1.25
	}
	df.FilterOut(f)
	assert.Equal(t, 1, len(df.TraceSet))
	assert.Equal(t, []string{"565"}, df.ParamSet["config"])

	df = &DataFrame{
		TraceSet: types.TraceSet{
			",arch=x86,config=565,":  types.Trace([]float32{1.2, 2.1}),
			",arch=x86,config=8888,": types.Trace([]float32{1.3, 3.1}),
			",arch=x86,config=gpu,":  types.Trace([]float32{1.4, 4.1}),
		},
		ParamSet: paramtools.NewReadOnlyParamSet(),
	}
	f = func(tr types.Trace) bool {
		return true
	}
	df.FilterOut(f)
	assert.Equal(t, 0, len(df.TraceSet))
}

func TestSlice(t *testing.T) {
	unittest.SmallTest(t)
	df := &DataFrame{
		Header: []*ColumnHeader{
			{Offset: 10},
			{Offset: 12},
			{Offset: 14},
			{Offset: 15},
			{Offset: 16},
			{Offset: 17},
		},
		TraceSet: types.TraceSet{
			",arch=x86,config=565,":  types.Trace([]float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6}),
			",arch=x86,config=8888,": types.Trace([]float32{1.1, 1.2, 1.3, 1.4, 1.5, 1.6}),
			",arch=x86,config=gpu,":  types.Trace([]float32{2.1, 2.2, 2.3, 2.4, 2.5, 2.6}),
		},
		ParamSet: paramtools.NewReadOnlyParamSet(),
	}

	// Test error conditions.
	_, err := df.Slice(0, 10)
	assert.Error(t, err)

	_, err = df.Slice(4, 3)
	assert.Error(t, err)

	// Test boundary conditions.
	sub, err := df.Slice(1, 0)
	assert.NoError(t, err)
	assert.Equal(t, []*ColumnHeader{}, sub.Header)
	assert.Len(t, sub.TraceSet, 3)
	assert.Len(t, sub.TraceSet[",arch=x86,config=gpu,"], 0)

	// Test the happy path.
	sub, err = df.Slice(0, 3)
	assert.NoError(t, err)
	assert.Equal(t, []*ColumnHeader{
		{Offset: 10},
		{Offset: 12},
		{Offset: 14},
	}, sub.Header)
	assert.Len(t, sub.TraceSet, 3)
	assert.Equal(t, sub.TraceSet[",arch=x86,config=gpu,"], types.Trace([]float32{2.1, 2.2, 2.3}))
	assert.Equal(t, sub.ParamSet, paramtools.ReadOnlyParamSet{"arch": []string{"x86"}, "config": []string{"565", "8888", "gpu"}})

	sub, err = df.Slice(1, 3)
	assert.NoError(t, err)
	assert.Equal(t, []*ColumnHeader{
		{Offset: 12},
		{Offset: 14},
		{Offset: 15},
	}, sub.Header)
	assert.Len(t, sub.TraceSet, 3)
	assert.Equal(t, sub.TraceSet[",arch=x86,config=gpu,"], types.Trace([]float32{2.2, 2.3, 2.4}))
	assert.Equal(t, sub.ParamSet, paramtools.ReadOnlyParamSet{"arch": []string{"x86"}, "config": []string{"565", "8888", "gpu"}})

}

func TestFromTimeRange_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, db, _, _, instanceConfig, cleanup := gittest.NewForTest(t)
	defer cleanup()
	g, err := perfgit.New(ctx, true, db, instanceConfig)
	require.NoError(t, err)

	columnHeaders, commitNumbers, _, err := FromTimeRange(ctx, g, gittest.StartTime, gittest.StartTime.Add(2*time.Minute), false)
	require.NoError(t, err)
	assert.Equal(t, []*ColumnHeader{
		{
			Offset:    0,
			Timestamp: gittest.StartTime.Unix(),
		},
		{
			Offset:    1,
			Timestamp: gittest.StartTime.Add(time.Minute).Unix(),
		},
	}, columnHeaders)
	assert.Equal(t, []types.CommitNumber{0, 1}, commitNumbers)
}

func TestFromTimeRange_EmptySlicesIfNothingInTimeRange(t *testing.T) {
	unittest.LargeTest(t)
	ctx, db, _, _, instanceConfig, cleanup := gittest.NewForTest(t)
	defer cleanup()
	g, err := perfgit.New(ctx, true, db, instanceConfig)
	require.NoError(t, err)

	// Query outside the time of any commit.
	columnHeaders, commitNumbers, _, err := FromTimeRange(ctx, g, gittest.StartTime.Add(-time.Hour), gittest.StartTime.Add(-time.Hour+2*time.Minute), false)
	require.NoError(t, err)
	assert.Empty(t, columnHeaders)
	assert.Empty(t, commitNumbers)
}
