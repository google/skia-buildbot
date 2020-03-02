package regression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/stepfit"
)

func TestRegressions(t *testing.T) {
	unittest.SmallTest(t)
	r := New()
	assert.True(t, r.Triaged(), "With no clusters, it should have Triaged() == true.")

	df := &dataframe.FrameResponse{}
	cl := &clustering2.ClusterSummary{}
	r.SetLow("source_type=skp", df, cl)
	assert.False(t, r.Triaged(), "Should not be Triaged.")

	// Triage the low cluster.
	err := r.TriageLow("source_type=skp", TriageStatus{
		Status:  POSITIVE,
		Message: "SKP Update",
	})
	assert.NoError(t, err)
	assert.True(t, r.Triaged())

	// Trying to triage a high cluster that doesn't exists.
	err = r.TriageHigh("source_type=skp", TriageStatus{
		Status: NEGATIVE,
	})
	assert.Equal(t, err, ErrNoClusterFound)
	assert.True(t, r.Triaged())

	// Set a high cluster.
	r.SetHigh("source_type=skp", df, cl)
	assert.False(t, r.Triaged())

	// And triage the high cluster.
	err = r.TriageHigh("source_type=skp", TriageStatus{
		Status:  NEGATIVE,
		Message: "See bug #foo.",
	})
	assert.NoError(t, err)
	assert.True(t, r.Triaged())

	// Trying to triage an unknown query.
	err = r.TriageHigh("uknownquery", TriageStatus{
		Status: NEGATIVE,
	})
	assert.Equal(t, err, ErrNoClusterFound)

	// Try serializing to JSON.
	b, err := r.JSON()
	assert.NoError(t, err)
	assert.Equal(t, "{\"by_query\":{\"source_type=skp\":{\"low\":{\"centroid\":null,\"shortcut\":\"\",\"param_summaries2\":null,\"step_fit\":null,\"step_point\":null,\"num\":0},\"high\":{\"centroid\":null,\"shortcut\":\"\",\"param_summaries2\":null,\"step_fit\":null,\"step_point\":null,\"num\":0},\"frame\":{\"dataframe\":null,\"skps\":null,\"msg\":\"\"},\"low_status\":{\"status\":\"positive\",\"message\":\"SKP Update\"},\"high_status\":{\"status\":\"negative\",\"message\":\"See bug #foo.\"}}}}", string(b))
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
