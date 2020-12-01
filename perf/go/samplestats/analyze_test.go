package samplestats

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/ingest/parser"
)

func TestAnalyze_MannWhitneyUTest_SuccessfullyDetectChange(t *testing.T) {
	unittest.SmallTest(t)

	before := map[string]parser.Samples{
		",name=test1,": {
			Params: paramtools.Params{"name": "test1"},
			Values: []float64{12, 11, 13, 15},
		},
	}
	after := map[string]parser.Samples{
		",name=test1,": {
			Params: paramtools.Params{"name": "test1"},
			Values: []float64{2, 1, 3, 5},
		},
	}

	res := Analyze(Config{}, before, after)
	require.Len(t, res.Rows, 1)
	expected := Row{
		Name: ",name=test1,",
		Samples: [2]Metrics{
			{
				Mean:    12.75,
				StdDev:  1.707825127659933,
				Values:  []float64{12, 11, 13, 15},
				Percent: 13.394706883607318},
			{
				Mean:    2.75,
				StdDev:  1.707825127659933,
				Values:  []float64{2, 1, 3, 5},
				Percent: 62.10273191490665},
		},
		Params: paramtools.Params{"name": "test1"},
		Delta:  -78.43137254901961,
		P:      0.028571428571428577, /* value copied from https://github.com/aclements/go-moremath/blob/222cfcba2589e908ed7477cd696845996bf3593c/stats/utest_test.go#L45 */
		Note:   "",
	}
	assert.Equal(t, expected, res.Rows[0])
}

func TestAnalyze_TwoSampleWelchTTest_SuccessfullyDetectChange(t *testing.T) {
	unittest.SmallTest(t)

	before := map[string]parser.Samples{
		",name=test1,": {
			Params: paramtools.Params{"name": "test1"},
			Values: []float64{12, 11, 13, 15},
		},
	}
	after := map[string]parser.Samples{
		",name=test1,": {
			Params: paramtools.Params{"name": "test1"},
			Values: []float64{2, 1, 3, 5},
		},
	}

	res := Analyze(Config{
		Test: TTest,
	}, before, after)
	require.Len(t, res.Rows, 1)
	expected := Row{
		Name: ",name=test1,",
		Samples: [2]Metrics{
			{
				Mean:    12.75,
				StdDev:  1.707825127659933,
				Values:  []float64{12, 11, 13, 15},
				Percent: 13.394706883607318},
			{
				Mean:    2.75,
				StdDev:  1.707825127659933,
				Values:  []float64{2, 1, 3, 5},
				Percent: 62.10273191490665},
		},
		Params: paramtools.Params{"name": "test1"},
		Delta:  -78.43137254901961,
		P:      0.00016793700357609076,
		Note:   "",
	}
	assert.Equal(t, expected, res.Rows[0])
}

func TestAnalyze_TwoResults_ResultsAreSortedCorrectly(t *testing.T) {
	unittest.SmallTest(t)

	// Set up samples where test1 has a smaller delta than test2.
	before := map[string]parser.Samples{
		",name=test1,": {
			Params: paramtools.Params{"name": "test1"},
			Values: []float64{1, 1, 1, 1},
		},
		",name=test2,": {
			Params: paramtools.Params{"name": "test2"},
			Values: []float64{1, 1, 1, 1},
		},
	}
	after := map[string]parser.Samples{
		",name=test1,": {
			Params: paramtools.Params{"name": "test1"},
			Values: []float64{2, 2, 2, 2},
		},
		",name=test2,": {
			Params: paramtools.Params{"name": "test2"},
			Values: []float64{3, 3, 3, 3},
		},
	}

	// Sort by Delta.
	res := Analyze(Config{
		Order: ByDelta,
		All:   true,
	}, before, after)
	require.Len(t, res.Rows, 2)
	assert.Equal(t, ",name=test1,", res.Rows[0].Name)
	assert.Equal(t, ",name=test2,", res.Rows[1].Name)

	// Sort by Reverse Delta.
	res = Analyze(Config{
		Order: Reverse(ByDelta),
		All:   true,
	}, before, after)
	require.Len(t, res.Rows, 2)
	assert.Equal(t, ",name=test2,", res.Rows[0].Name)
	assert.Equal(t, ",name=test1,", res.Rows[1].Name)
}

func TestAnalyze_ErrorCalculatingTest_NoteContainsErrorMessage(t *testing.T) {
	unittest.SmallTest(t)

	// Set up samples where test1 doesn't change, but test2 does,
	// and confirm we sort correctly on the delta.
	before := map[string]parser.Samples{
		",name=test1,": {
			Params: paramtools.Params{"name": "test1"},
			Values: []float64{1, 1, 1, 1},
		},
	}
	after := map[string]parser.Samples{
		",name=test1,": {
			Params: paramtools.Params{"name": "test1"},
			Values: []float64{1, 1, 1, 1},
		},
	}

	res := Analyze(Config{
		Order: Reverse(ByDelta),
		All:   true,
	}, before, after)
	require.Len(t, res.Rows, 1)

	// Test for the NaN value first.
	assert.True(t, math.IsNaN(res.Rows[0].Delta))

	// NaNs don't equal, so replace Delta with a non-Nan value before testing
	// the equality of the rest of the struct.
	const nonNaN = 1.0
	expected := Row{
		Name: ",name=test1,",
		Samples: [2]Metrics{
			{
				Mean:    1,
				StdDev:  0,
				Values:  []float64{1, 1, 1, 1},
				Percent: 0},
			{
				Mean:    1,
				StdDev:  0,
				Values:  []float64{1, 1, 1, 1},
				Percent: 0},
		},
		Params: paramtools.Params{"name": "test1"},
		Delta:  nonNaN,
		P:      1.0,
		Note:   "all samples are equal",
	}
	row := res.Rows[0]
	row.Delta = nonNaN
	assert.Equal(t, expected, row)
}
