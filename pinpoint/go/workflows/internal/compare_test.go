package internal

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.temporal.io/sdk/testsuite"
)

func TestCompareActivity_FunctionalDifferent_ReturnsFunctional(t *testing.T) {
	xErr := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	yErr := []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	errRate := 1.0
	cpv := CommitPairValues{
		Lower: CommitValues{
			ErrorValues: xErr,
		},
		Higher: CommitValues{
			ErrorValues: yErr,
		},
	}

	expected, err := compare.CompareFunctional(xErr, yErr, errRate)
	require.NoError(t, err)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(CompareActivity)
	res, err := env.ExecuteActivity(CompareActivity, cpv, 1.0, errRate, compare.UnknownDir)
	require.NoError(t, err)

	var actual *CombinedResults
	err = res.Get(&actual)
	require.NoError(t, err)
	assert.Equal(t, expected, actual.Result)
	assert.Nil(t, actual.OtherResult)
	assert.Equal(t, Functional, actual.ResultType)
}

func TestCompareActivity_PerformanceNil_ReturnsFunctional(t *testing.T) {
	xErr := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	valuesA := []float64{}
	magnitude := 11.8
	cpv := CommitPairValues{
		Lower: CommitValues{
			Values:      valuesA,
			ErrorValues: xErr,
		},
		Higher: CommitValues{
			Values:      valuesA,
			ErrorValues: xErr,
		},
	}

	expectedFunc, err := compare.CompareFunctional(xErr, xErr, 1.0)
	require.NoError(t, err)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(CompareActivity)
	res, err := env.ExecuteActivity(CompareActivity, cpv, magnitude, 1.0, compare.UnknownDir)
	require.NoError(t, err)

	var actual *CombinedResults
	err = res.Get(&actual)
	require.NoError(t, err)
	assert.Equal(t, expectedFunc, actual.Result)
	assert.Equal(t, compare.NilVerdict, actual.OtherResult.Verdict)
	assert.Equal(t, Functional, actual.ResultType)
}

func TestCompareActivity_PerformanceDifferent_ReturnsPerformance(t *testing.T) {
	xErr := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	valuesA := []float64{35.54, 34.799, 32.397, 35.373, 37.256, 32.199, 41.761, 33.616, 34.863, 34.588}
	valuesB := []float64{42.226, 45.616, 37.242, 48.362, 42.206, 44.049, 42.933, 51.292, 50.884, 40.601}
	cpv := CommitPairValues{
		Lower: CommitValues{
			Values:      valuesA,
			ErrorValues: xErr,
		},
		Higher: CommitValues{
			Values:      valuesB,
			ErrorValues: xErr,
		},
	}

	expectedFunc, err := compare.CompareFunctional(xErr, xErr, 1.0)
	require.NoError(t, err)
	expectedPerf, err := compare.ComparePerformance(valuesA, valuesB, 1.0, compare.UnknownDir)
	require.NoError(t, err)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(CompareActivity)
	res, err := env.ExecuteActivity(CompareActivity, cpv, 1.0, 1.0, compare.UnknownDir)
	require.NoError(t, err)

	var actual *CombinedResults
	err = res.Get(&actual)
	require.NoError(t, err)
	assert.Equal(t, expectedPerf, actual.Result)
	assert.Equal(t, expectedFunc, actual.OtherResult)
	assert.Equal(t, Performance, actual.ResultType)
}

func TestCompareActivity_FunctionalUnknownPerformanceSame_ReturnsFunctional(t *testing.T) {
	xErr := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	yErr := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	valuesA := []float64{35.54, 34.799, 32.397, 35.373, 37.256, 32.199, 41.761, 33.616, 34.863, 34.588}
	valuesB := []float64{36.176, 38.124, 34.07, 35.3, 33.921, 34.472, 33.264, 36.727, 38.353, 33.941}
	magnitude := 11.8
	cpv := CommitPairValues{
		Lower: CommitValues{
			Values:      valuesA,
			ErrorValues: xErr,
		},
		Higher: CommitValues{
			Values:      valuesB,
			ErrorValues: yErr,
		},
	}

	expectedFunc, err := compare.CompareFunctional(xErr, yErr, 0.5)
	require.NoError(t, err)
	expectedPerf, err := compare.ComparePerformance(valuesA, valuesB, magnitude, compare.UnknownDir)
	require.NoError(t, err)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(CompareActivity)
	res, err := env.ExecuteActivity(CompareActivity, cpv, magnitude, 0.5, compare.UnknownDir)
	require.NoError(t, err)

	var actual *CombinedResults
	err = res.Get(&actual)
	require.NoError(t, err)
	assert.Equal(t, expectedFunc, actual.Result)
	assert.Equal(t, expectedPerf, actual.OtherResult)
	assert.Equal(t, Functional, actual.ResultType)
}

func TestCompareActivity_FunctionalSame_ReturnsPerformance(t *testing.T) {
	xErr := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	valuesA := []float64{35.54, 34.799, 32.397, 35.373, 37.256, 32.199, 41.761, 33.616, 34.863, 34.588}
	valuesB := []float64{36.176, 38.124, 34.07, 35.3, 33.921, 34.472, 33.264, 36.727, 38.353, 33.941}
	magnitude := 11.8
	cpv := CommitPairValues{
		Lower: CommitValues{
			Values:      valuesA,
			ErrorValues: xErr,
		},
		Higher: CommitValues{
			Values:      valuesB,
			ErrorValues: xErr,
		},
	}

	expectedFunc, err := compare.CompareFunctional(xErr, xErr, 1.0)
	require.NoError(t, err)
	expectedPerf, err := compare.ComparePerformance(valuesA, valuesB, magnitude, compare.UnknownDir)
	require.NoError(t, err)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(CompareActivity)
	res, err := env.ExecuteActivity(CompareActivity, cpv, magnitude, 1.0, compare.UnknownDir)
	require.NoError(t, err)

	var actual *CombinedResults
	err = res.Get(&actual)
	require.NoError(t, err)
	assert.Equal(t, expectedPerf, actual.Result)
	assert.Equal(t, expectedFunc, actual.OtherResult)
	assert.Equal(t, Performance, actual.ResultType)
}

func TestComparePairwise_GivenSimpleValues_ReturnsResult(t *testing.T) {
	valuesA := []float64{8491008, 8491008, 8491008, 8491008, 8491008, 8491008, 8491008, 8491008, 8491008, 8491008}
	valuesB := []float64{14225408, 14225408, 14225408, 14225408, 14225408, 14225408, 14225408, 14225408, 14225408, 14225408}

	// demonstrate the edge case
	nanPerf, err := compare.ComparePairwise(valuesA, valuesB, compare.UnknownDir)
	require.NoError(t, err)
	require.True(t, math.IsNaN(nanPerf.PairwiseWilcoxonSignedRankedTestResult.LowerCi))
	require.True(t, math.IsNaN(nanPerf.PairwiseWilcoxonSignedRankedTestResult.UpperCi))

	valuesB = handlePairwiseEdgeCase(valuesA, valuesB)
	expectedPerf, err := compare.ComparePairwise(valuesA, valuesB, compare.UnknownDir)
	require.NoError(t, err)
	require.False(t, math.IsNaN(expectedPerf.PairwiseWilcoxonSignedRankedTestResult.LowerCi))
	require.False(t, math.IsNaN(expectedPerf.PairwiseWilcoxonSignedRankedTestResult.UpperCi))

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(ComparePairwiseActivity)
	res, err := env.ExecuteActivity(ComparePairwiseActivity, valuesA, valuesB, compare.UnknownDir)
	require.NoError(t, err)

	var actual *compare.ComparePairwiseResult
	err = res.Get(&actual)
	require.NoError(t, err)
	assert.Equal(t, expectedPerf, actual)
}

func TestComparePairwise_GivenAllSameValues_ReturnsResult(t *testing.T) {
	valuesA := []float64{8491008, 8491008, 8491008, 8491008, 8491008, 8491008, 8491008, 8491008, 8491008, 8491008}
	valuesB := []float64{14225408, 14225408, 14225408, 14225408, 14225408, 14225408, 14225408, 14225408, 14225408, 14225408}

	// demonstrate the edge case
	nanPerf, err := compare.ComparePairwise(valuesA, valuesB, compare.UnknownDir)
	require.NoError(t, err)
	require.True(t, math.IsNaN(nanPerf.PairwiseWilcoxonSignedRankedTestResult.LowerCi))
	require.True(t, math.IsNaN(nanPerf.PairwiseWilcoxonSignedRankedTestResult.UpperCi))

	valuesB = handlePairwiseEdgeCase(valuesA, valuesB)
	expectedPerf, err := compare.ComparePairwise(valuesA, valuesB, compare.UnknownDir)
	require.NoError(t, err)
	require.False(t, math.IsNaN(expectedPerf.PairwiseWilcoxonSignedRankedTestResult.LowerCi))
	require.False(t, math.IsNaN(expectedPerf.PairwiseWilcoxonSignedRankedTestResult.UpperCi))

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()
	env.RegisterActivity(ComparePairwiseActivity)
	res, err := env.ExecuteActivity(ComparePairwiseActivity, valuesA, valuesB, compare.UnknownDir)
	require.NoError(t, err)

	var actual *compare.ComparePairwiseResult
	err = res.Get(&actual)
	require.NoError(t, err)
	assert.Equal(t, expectedPerf, actual)
}
