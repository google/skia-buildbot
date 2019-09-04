package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

var (
	testOne     = types.TestName("test-1")
	testTwo     = types.TestName("test-2")
	digestOne   = types.Digest("abcefgh")
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
