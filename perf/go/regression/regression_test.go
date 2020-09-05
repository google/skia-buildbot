package regression

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/stepfit"
)

var testTime = time.Date(2020, 05, 01, 12, 00, 00, 00, time.UTC)

func TestRegressions(t *testing.T) {
	unittest.SmallTest(t)
	r := New()

	// Try serializing to JSON.
	b, err := r.JSON()
	assert.NoError(t, err)
	assert.Equal(t, "{\"by_query\":{}}", string(b))
}

func TestMerge(t *testing.T) {
	unittest.SmallTest(t)

	r := NewRegression()

	rhs := NewRegression()
	df := &dataframe.FrameResponse{}
	cl := &clustering2.ClusterSummary{
		StepFit: &stepfit.StepFit{
			Regression: 1,
		},
	}
	rhs.Low = cl
	rhs.Frame = df

	r = r.Merge(rhs)
	assert.Equal(t, r.Low, cl)
	assert.Equal(t, r.Frame, df)

	r = r.Merge(rhs)
	assert.Equal(t, r.Low, cl)
	assert.Equal(t, r.Frame, df)

	clbetter := &clustering2.ClusterSummary{
		StepFit: &stepfit.StepFit{
			Regression: 2,
		},
	}
	dfbetter := &dataframe.FrameResponse{}
	betterlow := NewRegression()
	betterlow.Low = clbetter
	betterlow.Frame = dfbetter

	r = r.Merge(betterlow)
	assert.Equal(t, r.Low, clbetter)
	assert.Equal(t, r.Frame, dfbetter)

	r = r.Merge(betterlow)
	assert.Equal(t, r.Low, clbetter)
	assert.Equal(t, r.Frame, dfbetter)

	// Now the same for High.
	rhs = NewRegression()
	df = &dataframe.FrameResponse{}
	cl = &clustering2.ClusterSummary{
		StepFit: &stepfit.StepFit{
			Regression: -1,
		},
	}
	rhs.High = cl
	rhs.Frame = df

	r = r.Merge(rhs)
	assert.Equal(t, r.High, cl)
	assert.Equal(t, r.Frame, df)

	r = r.Merge(rhs)
	assert.Equal(t, r.High, cl)
	assert.Equal(t, r.Frame, df)

	clbetter = &clustering2.ClusterSummary{
		StepFit: &stepfit.StepFit{
			Regression: -2,
		},
	}
	dfbetter = &dataframe.FrameResponse{}
	betterhigh := NewRegression()
	betterhigh.High = clbetter
	betterhigh.Frame = dfbetter

	r = r.Merge(betterhigh)
	assert.Equal(t, r.High, clbetter)
	assert.Equal(t, r.Frame, dfbetter)

	r = r.Merge(betterhigh)
	assert.Equal(t, r.High, clbetter)
	assert.Equal(t, r.Frame, dfbetter)
}
