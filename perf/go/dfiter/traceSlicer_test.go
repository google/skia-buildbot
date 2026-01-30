package dfiter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

func TestDfTraceSlicer(t *testing.T) {
	ctx := context.Background()
	traceID1 := ",config=8888,name=foo,"
	traceID2 := ",config=565,name=bar,"

	// Create a sample DataFrame.
	ps := paramtools.NewParamSet(paramtools.NewParams(traceID1))
	ps.AddParams(paramtools.NewParams(traceID2))
	df := &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			traceID1: types.Trace{1.0, 2.0, 3.0, 4.0, 5.0},
			traceID2: types.Trace{6.0, 7.0, 8.0, 9.0, 10.0},
		},
		Header: []*dataframe.ColumnHeader{
			{Offset: 1},
			{Offset: 2},
			{Offset: 3},
			{Offset: 4},
			{Offset: 5},
		},
		ParamSet: ps.Freeze(),
	}

	// Test with radius 1 (window size 3).
	slicer := NewStepFitDfTraceSlicer(df, 1)

	// Expected results. Keys are sorted, so traceID2 comes first.
	expected := []struct {
		traceID string
		trace   types.Trace
		header  []*dataframe.ColumnHeader
	}{
		{traceID2, types.Trace{6.0, 7.0, 8.0}, df.Header[0:3]},
		{traceID2, types.Trace{7.0, 8.0, 9.0}, df.Header[1:4]},
		{traceID2, types.Trace{8.0, 9.0, 10.0}, df.Header[2:5]},
		{traceID1, types.Trace{1.0, 2.0, 3.0}, df.Header[0:3]},
		{traceID1, types.Trace{2.0, 3.0, 4.0}, df.Header[1:4]},
		{traceID1, types.Trace{3.0, 4.0, 5.0}, df.Header[2:5]},
	}

	i := 0
	for slicer.Next() {
		require.Less(t, i, len(expected), "More windows than expected")
		win, err := slicer.Value(ctx)
		require.NoError(t, err)
		require.NotNil(t, win)

		require.Len(t, win.TraceSet, 1)
		var traceID string
		var trace types.Trace
		for key, value := range win.TraceSet {
			traceID = key
			trace = value
		}
		require.Equal(t, expected[i].traceID, traceID)
		require.Equal(t, expected[i].trace, trace)
		require.Equal(t, expected[i].header, win.Header)
		i++
	}
	require.Equal(t, len(expected), i)

	// Test with radius 0 (window size 1).
	slicer = NewStepFitDfTraceSlicer(df, 0)
	count := 0
	for slicer.Next() {
		_, err := slicer.Value(ctx)
		require.NoError(t, err)
		count++
	}
	require.Equal(t, 10, count) // 5 for each of 2 traces

	// Test with a window larger than the trace length.
	slicer = NewStepFitDfTraceSlicer(df, 5) // window size 11
	require.False(t, slicer.Next())
}

func TestNewDfTraceSlicer_EmptyDataFrame_ReturnsNoData(t *testing.T) {
	// Test with an empty DataFrame.
	df := &dataframe.DataFrame{
		TraceSet: types.TraceSet{},
		Header:   []*dataframe.ColumnHeader{},
		ParamSet: paramtools.NewReadOnlyParamSet(),
	}
	slicer := NewStepFitDfTraceSlicer(df, 1)
	require.False(t, slicer.Next())
}

func TestNewDfTraceSlicer_Non_EmptyDataFrame_ReturnsNoData(t *testing.T) {
	ctx := context.Background()

	// Test with an empty DataFrame.
	traceID1 := ",config=8888,name=foo,"
	traceID2 := ",config=565,name=bar,"

	// Create a sample DataFrame.
	ps := paramtools.NewParamSet(paramtools.NewParams(traceID1))
	ps.AddParams(paramtools.NewParams(traceID2))
	df := &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			traceID1: types.Trace{1.0, 2.0, 3.0, 4.0, 5.0},
			traceID2: types.Trace{6.0, 7.0, 8.0, 9.0, 10.0},
		},
		Header: []*dataframe.ColumnHeader{
			{Offset: 1},
			{Offset: 2},
			{Offset: 3},
			{Offset: 4},
			{Offset: 5},
		},
		ParamSet: ps.Freeze(),
	}
	slicer := NewStepFitDfTraceSlicer(df, 3)
	require.False(t, slicer.Next())
	require.False(t, slicer.Next())
	require.Panics(t, func() {
		_, _ = slicer.Value(ctx)
	}, "Slicer should panic on traces shorter than the window size.")
}

func TestDfTraceSlicer_MatchesDataFrameSlicerLegacy(t *testing.T) {
	ctx := context.Background()
	traceID1 := ",config=8888,name=foo,"

	// Create a sample DataFrame with a single trace.
	ps := paramtools.NewParamSet(paramtools.NewParams(traceID1))
	df := &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			traceID1: types.Trace{1.0, 2.0, 3.0, 4.0, 5.0},
		},
		Header: []*dataframe.ColumnHeader{
			{Offset: 1},
			{Offset: 2},
			{Offset: 3},
			{Offset: 4},
			{Offset: 5},
		},
		ParamSet: ps.Freeze(),
	}

	radius := 1
	slicer := NewStepFitDfTraceSlicer(df, radius)
	lSlicer := NewKmeansDataframeSlicer(df, radius)

	for {
		slicerHasNext := slicer.Next()
		newHasNext := lSlicer.Next()
		require.Equal(t, newHasNext, slicerHasNext, "Next() mismatch")

		if !slicerHasNext {
			break
		}

		win, err := slicer.Value(ctx)
		require.NoError(t, err)

		newWin, err := lSlicer.Value(ctx)
		require.NoError(t, err)

		require.Equal(t, newWin, win)
	}
}

func TestStepFitDfTraceSlicerWithMissingData(t *testing.T) {
	ctx := context.Background()
	traceID1 := ",config=8888,name=foo,"

	// Create a sample DataFrame.
	ps := paramtools.NewParamSet(paramtools.NewParams(traceID1))
	df := &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			traceID1: types.Trace{1.0, 2.0, vec32.MissingDataSentinel, 4.0, 5.0, 6.0},
		},
		Header: []*dataframe.ColumnHeader{
			{Offset: 1},
			{Offset: 2},
			{Offset: 3},
			{Offset: 4},
			{Offset: 5},
			{Offset: 6},
		},
		ParamSet: ps.Freeze(),
	}

	// Test with radius 1 (window size 3).
	slicer := NewStepFitDfTraceSlicer(df, 1)

	// Expected results.
	expected := []struct {
		traceID string
		trace   types.Trace
		header  []*dataframe.ColumnHeader
	}{
		{traceID1, types.Trace{1.0, 2.0, 4.0}, []*dataframe.ColumnHeader{{Offset: 1}, {Offset: 2}, {Offset: 4}}},
		{traceID1, types.Trace{2.0, 4.0, 5.0}, []*dataframe.ColumnHeader{{Offset: 2}, {Offset: 4}, {Offset: 5}}},
		{traceID1, types.Trace{4.0, 5.0, 6.0}, []*dataframe.ColumnHeader{{Offset: 4}, {Offset: 5}, {Offset: 6}}},
	}

	i := 0
	for slicer.Next() {
		require.Less(t, i, len(expected), "More windows than expected")
		win, err := slicer.Value(ctx)
		require.NoError(t, err)
		require.NotNil(t, win)

		require.Len(t, win.TraceSet, 1)
		var traceID string
		var trace types.Trace
		for key, value := range win.TraceSet {
			traceID = key
			trace = value
		}
		require.Equal(t, expected[i].traceID, traceID)
		require.Equal(t, expected[i].trace, trace)
		require.Equal(t, expected[i].header, win.Header)
		i++
	}
	require.Equal(t, len(expected), i)
}

func TestStepFitDfTraceSlicerWithMoreMissingData(t *testing.T) {
	ctx := context.Background()
	// Note: The trace keys are chosen so they are sorted alphabetically as traceID2, traceID3, traceID1.
	traceID1 := ",name=foo," // Will be processed last.
	traceID2 := ",name=bar," // Will be processed first.
	traceID3 := ",name=baz," // Will be processed second.

	h := func(offsets ...int) []*dataframe.ColumnHeader {
		ret := make([]*dataframe.ColumnHeader, len(offsets))
		for i, offset := range offsets {
			ret[i] = &dataframe.ColumnHeader{Offset: types.CommitNumber(offset)}
		}
		return ret
	}

	type expectedWindow struct {
		traceID string
		trace   types.Trace
		header  []*dataframe.ColumnHeader
	}

	testCases := []struct {
		name            string
		traceSet        types.TraceSet
		expectedWindows []expectedWindow
	}{
		{
			name: "All missing in some traces",
			traceSet: types.TraceSet{
				traceID1: {1.0, 2.0, vec32.MissingDataSentinel, 4.0, 5.0, 6.0},
				traceID2: {vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel},
				traceID3: {vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel},
			},
			expectedWindows: []expectedWindow{
				{traceID1, types.Trace{1.0, 2.0, 4.0}, h(1, 2, 4)},
				{traceID1, types.Trace{2.0, 4.0, 5.0}, h(2, 4, 5)},
				{traceID1, types.Trace{4.0, 5.0, 6.0}, h(4, 5, 6)},
			},
		},
		{
			name: "Missing at the beginning",
			traceSet: types.TraceSet{
				traceID1: {1.0, 2.0, vec32.MissingDataSentinel, 4.0, 5.0, 6.0},
				traceID2: {vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel},
				traceID3: {vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, 4, 5, 6},
			},
			expectedWindows: []expectedWindow{
				{traceID3, types.Trace{4, 5, 6}, h(4, 5, 6)},
				{traceID1, types.Trace{1.0, 2.0, 4.0}, h(1, 2, 4)},
				{traceID1, types.Trace{2.0, 4.0, 5.0}, h(2, 4, 5)},
				{traceID1, types.Trace{4.0, 5.0, 6.0}, h(4, 5, 6)},
			},
		},
		{
			name: "Missing at the end",
			traceSet: types.TraceSet{
				traceID1: {1.0, 2.0, vec32.MissingDataSentinel, 4.0, 5.0, 6.0},
				traceID2: {vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel},
				traceID3: {1, 2, 3, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel},
			},
			expectedWindows: []expectedWindow{
				{traceID3, types.Trace{1, 2, 3}, h(1, 2, 3)},
				{traceID1, types.Trace{1.0, 2.0, 4.0}, h(1, 2, 4)},
				{traceID1, types.Trace{2.0, 4.0, 5.0}, h(2, 4, 5)},
				{traceID1, types.Trace{4.0, 5.0, 6.0}, h(4, 5, 6)},
			},
		},
		{
			name: "Interleaved missing data",
			traceSet: types.TraceSet{
				traceID1: {1.0, 2.0, vec32.MissingDataSentinel, 4.0, 5.0, 6.0},
				traceID2: {vec32.MissingDataSentinel, 2, vec32.MissingDataSentinel, 4, vec32.MissingDataSentinel, 6},
				traceID3: {1, 2, 3, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel},
			},
			expectedWindows: []expectedWindow{
				{traceID2, types.Trace{2, 4, 6}, h(2, 4, 6)},
				{traceID3, types.Trace{1, 2, 3}, h(1, 2, 3)},
				{traceID1, types.Trace{1.0, 2.0, 4.0}, h(1, 2, 4)},
				{traceID1, types.Trace{2.0, 4.0, 5.0}, h(2, 4, 5)},
				{traceID1, types.Trace{4.0, 5.0, 6.0}, h(4, 5, 6)},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ps := paramtools.NewParamSet()
			for k := range tc.traceSet {
				ps.AddParams(paramtools.NewParams(k))
			}

			df := &dataframe.DataFrame{
				TraceSet: tc.traceSet,
				Header:   h(1, 2, 3, 4, 5, 6),
				ParamSet: ps.Freeze(),
			}

			slicer := NewStepFitDfTraceSlicer(df, 1) // window size 3

			i := 0
			for slicer.Next() {
				require.Less(t, i, len(tc.expectedWindows), "More windows than expected")
				win, err := slicer.Value(ctx)
				require.NoError(t, err)

				var traceID string
				var trace types.Trace
				for k, v := range win.TraceSet {
					traceID = k
					trace = v
				}

				expected := tc.expectedWindows[i]
				require.Equal(t, expected.traceID, traceID)
				require.Equal(t, expected.trace, trace)
				require.Equal(t, expected.header, win.Header)
				i++
			}
			require.Equal(t, len(tc.expectedWindows), i, "Wrong number of windows.")
		})
	}
}
