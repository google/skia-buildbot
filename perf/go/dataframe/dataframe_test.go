package dataframe

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/vec32"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/types"
)

const (
	e = vec32.MissingDataSentinel
)

func TestBuildParamSet(t *testing.T) {
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
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	columnHeaders, commitNumbers, _, err := FromTimeRange(ctx, g, gittest.StartTime, gittest.StartTime.Add(2*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, 3, len(columnHeaders))
	assert.Equal(t, types.CommitNumber(0), columnHeaders[0].Offset)
	assert.Equal(t, TimestampSeconds(gittest.StartTime.Unix()), columnHeaders[0].Timestamp)
	assert.Equal(t, types.CommitNumber(1), columnHeaders[1].Offset)
	assert.Equal(t, TimestampSeconds(gittest.StartTime.Add(time.Minute).Unix()), columnHeaders[1].Timestamp)
	assert.Equal(t, []types.CommitNumber{0, 1, 2}, commitNumbers)
}

func TestFromTimeRange_EmptySlicesIfNothingInTimeRange(t *testing.T) {
	ctx, db, _, _, _, instanceConfig := gittest.NewForTest(t)
	g, err := perfgit.New(ctx, false, db, instanceConfig)
	require.NoError(t, err)

	// Query outside the time of any commit.
	columnHeaders, commitNumbers, _, err := FromTimeRange(ctx, g, gittest.StartTime.Add(-time.Hour), gittest.StartTime.Add(-time.Hour+2*time.Minute))
	require.NoError(t, err)
	assert.Empty(t, columnHeaders)
	assert.Empty(t, commitNumbers)
}

func TestMerge(t *testing.T) {
	// Simple
	a := []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	b := []*ColumnHeader{
		{Offset: 3},
		{Offset: 4},
	}
	m, aMap, bMap := MergeColumnHeaders(a, b)
	expected := []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 3},
		{Offset: 4},
	}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{0: 0, 1: 1, 2: 3}, aMap)
	assert.Equal(t, map[int]int{0: 2, 1: 3}, bMap)

	// Skips
	a = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	b = []*ColumnHeader{
		{Offset: 5},
		{Offset: 7},
	}
	m, aMap, bMap = MergeColumnHeaders(a, b)
	expected = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
		{Offset: 5},
		{Offset: 7},
	}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{0: 0, 1: 1, 2: 2}, aMap)
	assert.Equal(t, map[int]int{0: 3, 1: 4}, bMap)

	// Empty b
	a = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	b = []*ColumnHeader{}
	m, aMap, bMap = MergeColumnHeaders(a, b)
	expected = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{0: 0, 1: 1, 2: 2}, aMap)
	assert.Equal(t, map[int]int{}, bMap)

	// Empty a
	a = []*ColumnHeader{}
	b = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	m, aMap, bMap = MergeColumnHeaders(a, b)
	expected = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{}, aMap)
	assert.Equal(t, map[int]int{0: 0, 1: 1, 2: 2}, bMap)

	// Empty a and b.
	a = []*ColumnHeader{}
	b = []*ColumnHeader{}
	m, aMap, bMap = MergeColumnHeaders(a, b)
	expected = []*ColumnHeader{}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{}, aMap)
	assert.Equal(t, map[int]int{}, bMap)
}

func TestJoin(t *testing.T) {
	a := DataFrame{
		Header: []*ColumnHeader{
			{Offset: 1},
			{Offset: 2},
			{Offset: 4},
		},
		TraceSet: types.TraceSet{
			",config=8888,arch=x86,": []float32{0.1, 0.2, 0.4},
			",config=8888,arch=arm,": []float32{1.1, 1.2, 1.4},
		},
	}
	b := DataFrame{
		Header: []*ColumnHeader{
			{Offset: 3},
			{Offset: 4},
		},
		TraceSet: types.TraceSet{
			",config=565,arch=x86,": []float32{3.3, 3.4},
			",config=565,arch=arm,": []float32{4.3, 4.4},
		},
	}
	a.BuildParamSet()
	b.BuildParamSet()
	r := Join(&a, &b)

	expectedHeader := []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 3},
		{Offset: 4},
	}

	assert.Equal(t, expectedHeader, r.Header)
	assert.Len(t, r.TraceSet, 4)
	assert.Equal(t, types.Trace{0.1, 0.2, e, 0.4}, r.TraceSet[",config=8888,arch=x86,"])
	assert.Equal(t, types.Trace{1.1, 1.2, e, 1.4}, r.TraceSet[",config=8888,arch=arm,"])
	assert.Equal(t, types.Trace{e, e, 4.3, 4.4}, r.TraceSet[",config=565,arch=arm,"])
	assert.Equal(t, types.Trace{e, e, 3.3, 3.4}, r.TraceSet[",config=565,arch=x86,"])
}

func TestMultiJoin(t *testing.T) {
	a := DataFrame{
		Header: []*ColumnHeader{
			{Offset: 1},
			{Offset: 2},
			{Offset: 4},
		},
		TraceSet: types.TraceSet{
			",config=8888,arch=x86,": []float32{0.1, 0.2, 0.4},
			",config=8888,arch=arm,": []float32{1.1, 1.2, 1.4},
		},
	}
	b := DataFrame{
		Header: []*ColumnHeader{
			{Offset: 3},
			{Offset: 4},
		},
		TraceSet: types.TraceSet{
			",config=565,arch=x86,": []float32{3.3, 3.4},
			",config=565,arch=arm,": []float32{4.3, 4.4},
		},
	}
	c := DataFrame{
		Header: []*ColumnHeader{
			{Offset: 2},
			{Offset: 5},
		},
		TraceSet: types.TraceSet{
			",config=8888,arch=x86,": []float32{0.22, 0.55},
			",config=gpu,arch=arm,":  []float32{5.2, 5.5},
		},
	}
	a.BuildParamSet()
	b.BuildParamSet()
	c.BuildParamSet()

	r, err := MultiJoin(context.Background(), &a, &b, &c)
	require.NoError(t, err)

	expectedHeader := []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 3},
		{Offset: 4},
		{Offset: 5},
	}

	assert.Equal(t, expectedHeader, r.Header)
	assert.Len(t, r.TraceSet, 5)

	// config=8888,arch=x86 overlaps in a (Offsets: 1, 2, 4) and c (Offsets: 2, 5).
	// In MultiJoin, values for overlapping keys/offsets will be written by the last dataframe in sequence (c overrides a for Offset 2).
	assert.Equal(t, types.Trace{0.1, 0.22, e, 0.4, 0.55}, r.TraceSet[",config=8888,arch=x86,"])
	assert.Equal(t, types.Trace{1.1, 1.2, e, 1.4, e}, r.TraceSet[",config=8888,arch=arm,"])
	assert.Equal(t, types.Trace{e, e, 3.3, 3.4, e}, r.TraceSet[",config=565,arch=x86,"])
	assert.Equal(t, types.Trace{e, e, 4.3, 4.4, e}, r.TraceSet[",config=565,arch=arm,"])
	assert.Equal(t, types.Trace{e, 5.2, e, e, 5.5}, r.TraceSet[",config=gpu,arch=arm,"])

	// Verify ParamSet contains keys from all frames
	assert.Equal(t, []string{"565", "8888", "gpu"}, r.ParamSet["config"])
	assert.Equal(t, []string{"arm", "x86"}, r.ParamSet["arch"])
}

func TestMultiJoin_LargeScaleAndEdgeCases(t *testing.T) {
	// 1. Test nil / empty edge cases
	emptyResult, err := MultiJoin(context.Background())
	require.NoError(t, err)
	assert.Empty(t, emptyResult.Header)

	nilResult, err := MultiJoin(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, nilResult.Header)

	emptyDf := NewEmpty()
	emptyDfResult, err := MultiJoin(context.Background(), emptyDf)
	require.NoError(t, err)
	assert.Empty(t, emptyDfResult.Header)

	// 2. Test single DataFrame returns a deep copy
	singleDf := &DataFrame{
		Header: []*ColumnHeader{{Offset: 1}},
		TraceSet: types.TraceSet{
			",config=8888,arch=x86,": []float32{1.0},
		},
	}
	joinedSingle, err := MultiJoin(context.Background(), singleDf)
	require.NoError(t, err)
	assert.Equal(t, singleDf.Header, joinedSingle.Header)
	assert.NotSame(t, singleDf, joinedSingle, "MultiJoin should return a copy of single input DataFrame")

	// 3. Test large scale (>1500 keys) across multiple concurrent workers
	aTraces := make(types.TraceSet, 1000)
	bTraces := make(types.TraceSet, 1000)
	for i := 0; i < 1000; i++ {
		aKey := fmt.Sprintf(",arch=x86,id=%d,", i)
		bKey := fmt.Sprintf(",arch=arm,id=%d,", i)
		aTraces[aKey] = []float32{float32(i)}
		bTraces[bKey] = []float32{float32(i * 2)}
	}

	a := &DataFrame{
		Header:   []*ColumnHeader{{Offset: 1}},
		TraceSet: aTraces,
	}
	b := &DataFrame{
		Header:   []*ColumnHeader{{Offset: 2}},
		TraceSet: bTraces,
	}
	a.BuildParamSet()
	b.BuildParamSet()

	res, err := MultiJoin(context.Background(), a, b)
	require.NoError(t, err)
	assert.Len(t, res.TraceSet, 2000)
	assert.Len(t, res.Header, 2)
}

func TestMultiJoin_EquivalentToSequentialJoin(t *testing.T) {
	df1 := &DataFrame{
		Header: []*ColumnHeader{
			{Offset: 1},
			{Offset: 2},
			{Offset: 3},
		},
		TraceSet: types.TraceSet{
			",config=8888,arch=x86,": []float32{1.0, e, 1.2},
			",config=565,arch=x86,":  []float32{e, 2.1, 2.2},
		},
		SourceInfo: map[string]*types.TraceSourceInfo{
			",config=8888,arch=x86,": types.NewTraceSourceInfo(),
		},
	}
	df1.SourceInfo[",config=8888,arch=x86,"].Add(1, 101)

	df2 := &DataFrame{
		Header: []*ColumnHeader{
			{Offset: 2},
			{Offset: 3},
			{Offset: 4},
		},
		TraceSet: types.TraceSet{
			",config=8888,arch=x86,": []float32{1.15, e, 1.4},
			",config=gpu,arch=arm,":  []float32{3.0, 3.1, e},
		},
		SourceInfo: map[string]*types.TraceSourceInfo{
			",config=gpu,arch=arm,": types.NewTraceSourceInfo(),
		},
	}
	df2.SourceInfo[",config=gpu,arch=arm,"].Add(3, 102)

	df3 := &DataFrame{
		Header: []*ColumnHeader{
			{Offset: 3},
			{Offset: 5},
		},
		TraceSet: types.TraceSet{
			",config=8888,arch=x86,": []float32{1.3, e},
			",config=565,arch=x86,":  []float32{2.4, 2.5},
		},
	}

	df1.BuildParamSet()
	df2.BuildParamSet()
	df3.BuildParamSet()

	// 1. Join sequentially: Join(Join(df1, df2), df3)
	seqResult := Join(Join(df1, df2), df3)

	// 2. MultiJoin: MultiJoin(df1, df2, df3)
	multiResult, err := MultiJoin(context.Background(), df1, df2, df3)
	require.NoError(t, err)

	// Assert headers match
	assert.Equal(t, seqResult.Header, multiResult.Header)

	// Assert trace sets match (including missing data sentinels)
	assert.Equal(t, seqResult.TraceSet, multiResult.TraceSet)

	// Assert ParamSets match
	assert.Equal(t, seqResult.ParamSet, multiResult.ParamSet)

	// Assert SourceInfos match
	assert.Equal(t, len(seqResult.SourceInfo), len(multiResult.SourceInfo))
	for traceID, seqInfo := range seqResult.SourceInfo {
		multiInfo, ok := multiResult.SourceInfo[traceID]
		assert.True(t, ok)
		assert.ElementsMatch(t, seqInfo.GetAllSourceFileIds(), multiInfo.GetAllSourceFileIds())
	}
}

func TestMultiJoin_DifferentCommitNumbers(t *testing.T) {
	// df1 has commits: 1, 2, 3
	df1 := &DataFrame{
		Header: []*ColumnHeader{
			{Offset: 1},
			{Offset: 2},
			{Offset: 3},
		},
		TraceSet: types.TraceSet{
			",config=8888,arch=x86,": []float32{1.1, 1.2, 1.3},
		},
	}

	// df2 has commits: 1, 100, 200
	df2 := &DataFrame{
		Header: []*ColumnHeader{
			{Offset: 1},
			{Offset: 100},
			{Offset: 200},
		},
		TraceSet: types.TraceSet{
			",config=8888,arch=x86,": []float32{1.15, 100.0, 200.0},
		},
	}

	// df3 has commits: 3, 5, 10
	df3 := &DataFrame{
		Header: []*ColumnHeader{
			{Offset: 3},
			{Offset: 5},
			{Offset: 10},
		},
		TraceSet: types.TraceSet{
			",config=8888,arch=x86,": []float32{3.3, 5.0, 10.0},
		},
	}

	df1.BuildParamSet()
	df2.BuildParamSet()
	df3.BuildParamSet()

	seqResult := Join(Join(df1, df2), df3)
	multiResult, err := MultiJoin(context.Background(), df1, df2, df3)
	require.NoError(t, err)

	// Expected sorted headers: 1, 2, 3, 5, 10, 100, 200
	expectedOffsets := []types.CommitNumber{1, 2, 3, 5, 10, 100, 200}
	actualOffsets := make([]types.CommitNumber, len(multiResult.Header))
	for i, h := range multiResult.Header {
		actualOffsets[i] = h.Offset
	}
	assert.Equal(t, expectedOffsets, actualOffsets)

	// Assert trace values match between MultiJoin and sequential Join
	assert.Equal(t, seqResult.TraceSet, multiResult.TraceSet)

	// Explicitly check expected trace values:
	// Offset 1: 1.15 (df2 overrides df1)
	// Offset 2: 1.2  (df1)
	// Offset 3: 3.3  (df3 overrides df1)
	// Offset 5: 5.0  (df3)
	// Offset 10: 10.0 (df3)
	// Offset 100: 100.0 (df2)
	// Offset 200: 200.0 (df2)
	expectedTrace := types.Trace{1.15, 1.2, 3.3, 5.0, 10.0, 100.0, 200.0}
	assert.Equal(t, expectedTrace, multiResult.TraceSet[",config=8888,arch=x86,"])
}

func TestMultiJoin_DisjointTracesAndCommits(t *testing.T) {
	// df1: trace1 at commits 1, 2
	df1 := &DataFrame{
		Header: []*ColumnHeader{
			{Offset: 1},
			{Offset: 2},
		},
		TraceSet: types.TraceSet{
			",arch=x86,config=8888,": []float32{1.1, 1.2},
		},
	}

	// df2: trace2 at commits 10, 20
	df2 := &DataFrame{
		Header: []*ColumnHeader{
			{Offset: 10},
			{Offset: 20},
		},
		TraceSet: types.TraceSet{
			",arch=arm,config=565,": []float32{10.1, 20.1},
		},
	}

	// df3: trace3 at commits 100, 200
	df3 := &DataFrame{
		Header: []*ColumnHeader{
			{Offset: 100},
			{Offset: 200},
		},
		TraceSet: types.TraceSet{
			",arch=gpu,config=test,": []float32{100.1, 200.1},
		},
	}

	df1.BuildParamSet()
	df2.BuildParamSet()
	df3.BuildParamSet()

	seqResult := Join(Join(df1, df2), df3)
	multiResult, err := MultiJoin(context.Background(), df1, df2, df3)
	require.NoError(t, err)

	// 1. Expected unified headers sorted ascending: 1, 2, 10, 20, 100, 200
	expectedOffsets := []types.CommitNumber{1, 2, 10, 20, 100, 200}
	actualOffsets := make([]types.CommitNumber, len(multiResult.Header))
	for i, h := range multiResult.Header {
		actualOffsets[i] = h.Offset
	}
	assert.Equal(t, expectedOffsets, actualOffsets)

	// 2. Assert multiResult TraceSet matches seqResult TraceSet 100%
	assert.Equal(t, seqResult.TraceSet, multiResult.TraceSet)

	// 3. Verify each trace retains its exact values at its commit columns and has 'e' at all other columns:
	// trace1 (commits 1, 2) -> [1.1, 1.2, e, e, e, e]
	assert.Equal(t, types.Trace{1.1, 1.2, e, e, e, e}, multiResult.TraceSet[",arch=x86,config=8888,"])

	// trace2 (commits 10, 20) -> [e, e, 10.1, 20.1, e, e]
	assert.Equal(t, types.Trace{e, e, 10.1, 20.1, e, e}, multiResult.TraceSet[",arch=arm,config=565,"])

	// trace3 (commits 100, 200) -> [e, e, e, e, 100.1, 200.1]
	assert.Equal(t, types.Trace{e, e, e, e, 100.1, 200.1}, multiResult.TraceSet[",arch=gpu,config=test,"])

	// 4. Verify combined ParamSet includes keys from all 3 DataFrames
	assert.Equal(t, []string{"565", "8888", "test"}, multiResult.ParamSet["config"])
	assert.Equal(t, []string{"arm", "gpu", "x86"}, multiResult.ParamSet["arch"])
}

func TestMultiJoin_NilHeaderSafety(t *testing.T) {
	a := &DataFrame{
		Header: []*ColumnHeader{nil, {Offset: 10}},
		TraceSet: types.TraceSet{
			",config=8888,": []float32{999.0, 1.5},
		},
	}
	b := &DataFrame{
		Header: []*ColumnHeader{{Offset: 5}},
		TraceSet: types.TraceSet{
			",config=8888,": []float32{2.5},
		},
	}
	res, err := MultiJoin(context.Background(), a, b)
	require.NoError(t, err)
	assert.Len(t, res.Header, 2)
	assert.Equal(t, types.CommitNumber(5), res.Header[0].Offset)
	assert.Equal(t, types.CommitNumber(10), res.Header[1].Offset)
	// Verify that index 0 of nil header (999.0) did not corrupt column 0 (2.5)
	assert.Equal(t, types.Trace{2.5, 1.5}, res.TraceSet[",config=8888,"])
}

func TestMultiJoin_NilSourceInfoSafety(t *testing.T) {
	a := &DataFrame{
		Header: []*ColumnHeader{{Offset: 10}},
		TraceSet: types.TraceSet{
			",config=8888,": []float32{1.5},
		},
		SourceInfo: map[string]*types.TraceSourceInfo{
			",config=8888,": nil,
		},
	}
	b := &DataFrame{
		Header: []*ColumnHeader{{Offset: 20}},
		TraceSet: types.TraceSet{
			",config=8888,": []float32{2.5},
		},
	}
	res, err := MultiJoin(context.Background(), a, b)
	require.NoError(t, err)
	assert.Len(t, res.Header, 2)
}

func TestMultiJoin_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := &DataFrame{
		Header: []*ColumnHeader{{Offset: 10}},
		TraceSet: types.TraceSet{
			",config=8888,": []float32{1.5},
		},
	}
	b := &DataFrame{
		Header: []*ColumnHeader{{Offset: 20}},
		TraceSet: types.TraceSet{
			",config=8888,": []float32{2.5},
		},
	}
	_, err := MultiJoin(ctx, a, b)
	require.Error(t, err)
}

type dummyLoaderMetrics struct{}

func (d *dummyLoaderMetrics) RecordDbQuery(duration time.Duration) {}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()
	sem := make(chan struct{}, 5)
	ctxWithSem := WithQuerySemaphore(ctx, sem)

	retSem, ok := QuerySemaphoreFromContext(ctxWithSem)
	assert.True(t, ok)
	assert.Equal(t, sem, retSem)

	metrics := &dummyLoaderMetrics{}
	ctxWithMetrics := WithLoaderMetrics(ctx, metrics)
	retMetrics, ok := LoaderMetricsFromContext(ctxWithMetrics)
	assert.True(t, ok)
	assert.Equal(t, metrics, retMetrics)
}
func TestCompress(t *testing.T) {
	tests := []struct {
		name   string
		source *DataFrame
		want   *DataFrame
	}{
		{
			name: "DropLastColumn",
			source: &DataFrame{
				Header: []*ColumnHeader{
					{Offset: 1},
					{Offset: 2},
					{Offset: 3},
				},
				TraceSet: types.TraceSet{
					",arch=x86,": []float32{1, e, e},
					",arch=arm,": []float32{e, 2, e},
				},
			},
			want: &DataFrame{
				Header: []*ColumnHeader{
					{Offset: 1},
					{Offset: 2},
				},
				TraceSet: types.TraceSet{
					",arch=x86,": []float32{1, e},
					",arch=arm,": []float32{e, 2},
				},
			},
		},
		{
			name: "DropSecondColumn",
			source: &DataFrame{
				Header: []*ColumnHeader{
					{Offset: 1},
					{Offset: 2},
					{Offset: 3},
				},
				TraceSet: types.TraceSet{
					",arch=x86,": []float32{1, e, 3},
					",arch=arm,": []float32{e, e, 3.1},
				},
			},
			want: &DataFrame{
				Header: []*ColumnHeader{
					{Offset: 1},
					{Offset: 3},
				},
				TraceSet: types.TraceSet{
					",arch=x86,": []float32{1, 3},
					",arch=arm,": []float32{e, 3.1},
				},
			},
		},

		{
			name: "DoNotDropAnyColumns",
			source: &DataFrame{
				Header: []*ColumnHeader{
					{Offset: 1},
					{Offset: 2},
					{Offset: 3},
				},
				TraceSet: types.TraceSet{
					",arch=x86,": []float32{1, 2, 3},
				},
			},
			want: &DataFrame{
				Header: []*ColumnHeader{
					{Offset: 1},
					{Offset: 2},
					{Offset: 3},
				},
				TraceSet: types.TraceSet{
					",arch=x86,": []float32{1, 2, 3},
				},
			},
		},
		{
			name: "HandlesEmptyDataFrames",
			source: &DataFrame{
				Header:   []*ColumnHeader{},
				TraceSet: types.TraceSet{},
			},
			want: &DataFrame{
				Header:   []*ColumnHeader{},
				TraceSet: types.TraceSet{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertdeep.Equal(t, tt.want, tt.source.Compress())
		})
	}
}
