package regression

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

func TestStepFit(t *testing.T) {
	unittest.LargeTest(t)

	cleanup := testutil.InitDatastore(t, ds.REGRESSION)
	defer cleanup()

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

	sum, err := StepFit(df, 4, 0.01, nil, 50, types.ORIGINAL_STEP)
	assert.NoError(t, err)
	assert.NotNil(t, sum)
	assert.Equal(t, 1, len(sum.Clusters))
	assert.Equal(t, df.Header[2], sum.Clusters[0].StepPoint)
	assert.Equal(t, 2, len(sum.Clusters[0].Keys))
}
