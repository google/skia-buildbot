package clustering2

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/ctrace2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/kmeans"
	"go.skia.org/infra/perf/go/types"
)

func TestParamSummaries(t *testing.T) {
	unittest.SmallTest(t)
	obs := []kmeans.Clusterable{
		ctrace2.NewFullTrace(",arch=x86,config=8888,", []float32{1, 2}, 0.001),
		ctrace2.NewFullTrace(",arch=x86,config=565,", []float32{2, 3}, 0.001),
		ctrace2.NewFullTrace(",arch=x86,config=565,", []float32{3, 2}, 0.001),
	}
	expected := map[string][]ValueWeight{
		"arch": {
			{"x86", 26},
		},
		"config": {
			{"565", 21},
			{"8888", 16},
		},
	}
	assert.Equal(t, expected, getParamSummaries(obs))

	obs = []kmeans.Clusterable{}
	expected = map[string][]ValueWeight{}
	assert.Equal(t, expected, getParamSummaries(obs))
}

func TestCalcCusterSummaries(t *testing.T) {
	unittest.LargeTest(t)

	rand.Seed(1)
	now := time.Now()
	df := &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			",arch=x86,config=8888,": []float32{0, 0, 1, 1, 1},
			",arch=x86,config=565,":  []float32{0, 0, 1, 1, 1},
			",arch=arm,config=8888,": []float32{1, 1, 1, 1, 1},
			",arch=arm,config=565,":  []float32{1, 1, 1, 1, 1},
		},
		Header: []*dataframe.ColumnHeader{
			{
				Source:    "master",
				Offset:    0,
				Timestamp: now.Unix(),
			},
			{
				Source:    "master",
				Offset:    1,
				Timestamp: now.Add(time.Minute).Unix(),
			},
			{
				Source:    "master",
				Offset:    2,
				Timestamp: now.Add(2 * time.Minute).Unix(),
			},
			{
				Source:    "master",
				Offset:    3,
				Timestamp: now.Add(3 * time.Minute).Unix(),
			},
			{
				Source:    "master",
				Offset:    4,
				Timestamp: now.Add(4 * time.Minute).Unix(),
			},
		},
		ParamSet: paramtools.ParamSet{},
		Skip:     0,
	}
	for key := range df.TraceSet {
		df.ParamSet.AddParamsFromKey(key)
	}
	sum, err := CalculateClusterSummaries(df, 4, 0.01, nil, 50, types.ORIGINAL_STEP)
	assert.NoError(t, err)
	assert.NotNil(t, sum)
	assert.Equal(t, 2, len(sum.Clusters))
	assert.Equal(t, df.Header[2], sum.Clusters[0].StepPoint)
	assert.Equal(t, 2, len(sum.Clusters[0].Keys))
	assert.Equal(t, 2, len(sum.Clusters[1].Keys))
}

func TestCalcCusterSummariesDegenerate(t *testing.T) {
	unittest.SmallTest(t)
	rand.Seed(1)
	df := &dataframe.DataFrame{
		TraceSet: types.TraceSet{},
		Header:   []*dataframe.ColumnHeader{},
		ParamSet: paramtools.ParamSet{},
		Skip:     0,
	}
	_, err := CalculateClusterSummaries(df, 4, 0.01, nil, 50, types.ORIGINAL_STEP)
	assert.Error(t, err)
}
