package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

func TestTraceViewFn(t *testing.T) {
	unittest.SmallTest(t)

	type testCase struct {
		name string
		// inputs
		startHash string
		endHash   string

		// outputs
		lastTraceIdx      int
		trimmedStartIndex int
		trimmedEndIndex   int
	}

	testCases := []testCase{
		{
			name:      "whole tile",
			startHash: data.FirstCommitHash,
			endHash:   data.ThirdCommitHash,

			lastTraceIdx:      2,
			trimmedEndIndex:   2,
			trimmedStartIndex: 0,
		},
		{
			name:      "empty means whole tile",
			startHash: "",
			endHash:   "",

			lastTraceIdx:      2,
			trimmedEndIndex:   2,
			trimmedStartIndex: 0,
		},
		{
			name:      "invalid means whole tile",
			startHash: "not found",
			endHash:   "not found",

			lastTraceIdx:      2,
			trimmedEndIndex:   2,
			trimmedStartIndex: 0,
		},
		{
			name:      "last two",
			startHash: data.SecondCommitHash,
			endHash:   data.ThirdCommitHash,

			lastTraceIdx:      1,
			trimmedEndIndex:   2,
			trimmedStartIndex: 1,
		},
		{
			name:      "first only",
			startHash: data.FirstCommitHash,
			endHash:   data.FirstCommitHash,

			lastTraceIdx:      0,
			trimmedEndIndex:   0,
			trimmedStartIndex: 0,
		},
		{
			name:      "first two",
			startHash: data.FirstCommitHash,
			endHash:   data.SecondCommitHash,

			lastTraceIdx:      1,
			trimmedEndIndex:   1,
			trimmedStartIndex: 0,
		},
		{
			name:      "invalid start means beginning",
			startHash: "not found",
			endHash:   data.SecondCommitHash,

			lastTraceIdx:      1,
			trimmedEndIndex:   1,
			trimmedStartIndex: 0,
		},
		{
			name:      "invalid end means last",
			startHash: data.SecondCommitHash,
			endHash:   "not found",

			lastTraceIdx:      1,
			trimmedEndIndex:   2,
			trimmedStartIndex: 1,
		},
	}

	for _, tc := range testCases {
		lastIdx, traceViewFn, err := getTraceViewFn(data.MakeTestTile(), tc.startHash, tc.endHash)
		require.NoError(t, err, tc.name)
		assert.Equal(t, tc.lastTraceIdx, lastIdx, tc.name)
		assert.NotNil(t, traceViewFn, tc.name)
		// Run through all the traces and make sure they are properly trimmed
		for _, trace := range data.MakeTestTile().Traces {
			tr := trace.(*types.GoldenTrace)
			reducedTr := traceViewFn(tr)
			assert.Equal(t, tr.Digests[tc.trimmedStartIndex:tc.trimmedEndIndex+1], reducedTr.Digests, "test case %s with trace %v", tc.name, tr.Keys)
		}
	}
}

func TestTraceViewFnErr(t *testing.T) {
	unittest.SmallTest(t)

	// It's an error to swap the order of the hashes
	_, _, err := getTraceViewFn(data.MakeTestTile(), data.ThirdCommitHash, data.SecondCommitHash)
	require.Error(t, err)
	require.Contains(t, err.Error(), "later than end")
}

const (
	testOne   = types.TestName("test-1")
	testTwo   = types.TestName("test-2")
	digestOne = types.Digest("abcefgh")
)

var (
	paramSetOne = paramtools.ParamSet{
		"param-01": {"val-01"},
		"param-02": {"val-02"},
	}

	paramsTwo = paramtools.Params{
		"param-01": "gato",
		"param-03": "robato",
	}

	goldTrace = types.GoldenTrace{
		Keys: map[string]string{"param-01": "dog"},
	}
)

// TestIntermediate adds a few entries to the intermediate
// representation and makes sure that the data properly reflects it.
func TestIntermediate(t *testing.T) {
	unittest.SmallTest(t)

	srMap := srInterMap{}
	srMap.Add(testOne, digestOne, "", nil, paramSetOne)
	srMap.AddTestParams(testOne, digestOne, paramsTwo)
	srMap.AddTestParams(testTwo, digestOne, paramsTwo)
	srMap.Add(testTwo, digestOne, "mytrace", &goldTrace, paramSetOne)

	assert.Equal(t, srInterMap{
		testOne: map[types.Digest]*srIntermediate{
			digestOne: {
				test:   testOne,
				digest: digestOne,
				params: paramtools.ParamSet{
					"param-01": {"val-01", "gato"},
					"param-02": {"val-02"},
					"param-03": {"robato"},
				},
				traces: map[tiling.TraceId]*types.GoldenTrace{},
			},
		},
		testTwo: map[types.Digest]*srIntermediate{
			digestOne: {
				test:   testTwo,
				digest: digestOne,
				params: paramtools.ParamSet{
					"param-01": {"gato", "dog"},
					"param-03": {"robato"},
				},
				traces: map[tiling.TraceId]*types.GoldenTrace{
					"mytrace": &goldTrace,
				},
			},
		},
	}, srMap)
}
