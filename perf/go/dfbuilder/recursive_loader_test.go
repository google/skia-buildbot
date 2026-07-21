package dfbuilder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

func TestJaccardSimilarity_Identical(t *testing.T) {
	a := []int{1, 5, 10, 15}
	b := []int{1, 5, 10, 15}
	assert.InEpsilon(t, 1.0, jaccardSimilarity(a, b), 0.0001)
}

func TestJaccardSimilarity_Disjoint(t *testing.T) {
	a := []int{1, 2, 3}
	b := []int{4, 5, 6}
	assert.Equal(t, 0.0, jaccardSimilarity(a, b))
}

func TestJaccardSimilarity_PartialOverlap(t *testing.T) {
	// Intersection: {2, 4} (size 2)
	// Union: {1, 2, 3, 4, 5, 6} (size 6)
	// Similarity = 2 / 6 = 0.3333...
	a := []int{1, 2, 4, 5}
	b := []int{2, 3, 4, 6}
	assert.InEpsilon(t, 2.0/6.0, jaccardSimilarity(a, b), 0.0001)
}

func TestJaccardSimilarity_Empty(t *testing.T) {
	assert.Equal(t, 1.0, jaccardSimilarity(nil, nil))
	assert.Equal(t, 0.0, jaccardSimilarity([]int{1}, nil))
}

func TestRecursiveTracker(t *testing.T) {
	tracker := newRecursiveTracker()

	assert.False(t, tracker.isExhausted("key1"))

	tracker.setExhausted("key1")
	assert.True(t, tracker.isExhausted("key1"))
}

func TestCalculateClusterSearchWindow(t *testing.T) {
	b := &builder{tileSize: 256}

	// Case 1: Empty Header should not divide by zero and return minTileSize / prevRange safely
	emptyDf := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{},
		TraceSet: types.TraceSet{
			"key1": types.Trace([]float32{}),
		},
	}
	window := b.calculateClusterSearchWindow(emptyDf, []string{"key1"}, 100, 10)
	assert.Equal(t, int32(256), window)

	// Case 2: Sparse df header density calculation
	df := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{{Offset: 1}, {Offset: 2}, {Offset: 3}, {Offset: 4}},
		TraceSet: types.TraceSet{
			"key1": types.Trace([]float32{1.0, vec32.MissingDataSentinel, 2.0, vec32.MissingDataSentinel}),
		},
	}
	// Needed: 10 - 2 = 8 points. Density = 2 / 4 = 0.5. Estimated = 8 / 0.5 = 16. With 1.5 multiplier = 24.
	window2 := b.calculateClusterSearchWindow(df, []string{"key1"}, 10, 10)
	assert.Equal(t, int32(256), window2) // minTileSize 256 applies

	// Case 3: 0 non-missing data points found so far
	zeroDf := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{{Offset: 1}, {Offset: 2}, {Offset: 3}, {Offset: 4}},
		TraceSet: types.TraceSet{
			"key1": types.Trace([]float32{vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel}),
		},
	}
	// prevRange = 256. nonMissingCount = 0 -> searchCommits = 256 * 2 = 512
	window3 := b.calculateClusterSearchWindow(zeroDf, []string{"key1"}, 256, 10)
	assert.Equal(t, int32(512), window3)
}

func TestMaskExcessPoints(t *testing.T) {
	df := &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			"key1": types.Trace([]float32{1.0, vec32.MissingDataSentinel, 2.0, 3.0, vec32.MissingDataSentinel, 4.0}),
		},
	}
	maskExcessPoints(df, 2)
	assert.Equal(t, types.Trace([]float32{vec32.MissingDataSentinel, vec32.MissingDataSentinel, vec32.MissingDataSentinel, 3.0, vec32.MissingDataSentinel, 4.0}), df.TraceSet["key1"])
}

func TestCountNonMissing(t *testing.T) {
	b := &builder{}
	df := &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			"key1": types.Trace([]float32{1.0, vec32.MissingDataSentinel, 2.0, 3.0}),
		},
	}
	assert.Equal(t, 3, b.countNonMissing(df, "key1"))
}

func TestUpdateTrackingState_ExhaustsOnEmptyResultsEvenWithPositiveTargetCommit(t *testing.T) {
	b := &builder{}
	tracker := newRecursiveTracker()
	keys := []string{"trace1", "trace2"}

	// Case 1: dfExt is nil -> must mark keys as exhausted even if targetCommit > 0
	dfClean := b.updateTrackingState(nil, keys, 93645, tracker)
	assert.Nil(t, dfClean)
	assert.True(t, tracker.isExhausted("trace1"))
	assert.True(t, tracker.isExhausted("trace2"))

	// Case 2: dfExt compresses to empty -> must mark keys as exhausted even if targetCommit > 0
	tracker2 := newRecursiveTracker()
	emptyDfExt := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{},
		TraceSet: types.TraceSet{
			"trace1": types.Trace([]float32{vec32.MissingDataSentinel}),
		},
	}
	dfClean2 := b.updateTrackingState(emptyDfExt, keys, 50000, tracker2)
	assert.Nil(t, dfClean2)
	assert.True(t, tracker2.isExhausted("trace1"))
	assert.True(t, tracker2.isExhausted("trace2"))
}
