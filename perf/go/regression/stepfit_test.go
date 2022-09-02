package regression

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/types"
)

func TestStepFit(t *testing.T) {

	ctx := context.Background()
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
				Offset:    0,
				Timestamp: now.Unix(),
			},
			{
				Offset:    1,
				Timestamp: now.Add(time.Minute).Unix(),
			},
			{
				Offset:    2,
				Timestamp: now.Add(2 * time.Minute).Unix(),
			},
			{
				Offset:    3,
				Timestamp: now.Add(3 * time.Minute).Unix(),
			},
			{
				Offset:    4,
				Timestamp: now.Add(4 * time.Minute).Unix(),
			},
		},
		ParamSet: paramtools.NewReadOnlyParamSet(),
		Skip:     0,
	}
	ps := paramtools.NewParamSet()
	for key := range df.TraceSet {
		ps.AddParamsFromKey(key)
	}
	ps.Normalize()
	df.ParamSet = ps.Freeze()

	sum, err := StepFit(ctx, df, 4, 0.01, nil, 50, types.OriginalStep)
	assert.NoError(t, err)
	assert.NotNil(t, sum)
	assert.Equal(t, 1, len(sum.Clusters))
	assert.Equal(t, df.Header[2], sum.Clusters[0].StepPoint)
	assert.Equal(t, 2, len(sum.Clusters[0].Keys))
}
