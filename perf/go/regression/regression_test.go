package regression

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/ui/frame"
)

var testTime = time.Date(2020, 05, 01, 12, 00, 00, 00, time.UTC)

func TestMerge(t *testing.T) {

	r := NewRegression()

	rhs := NewRegression()
	df := &frame.FrameResponse{}
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
	dfbetter := &frame.FrameResponse{}
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
	df = &frame.FrameResponse{}
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
	dfbetter = &frame.FrameResponse{}
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

func TestDetermineIsImprovement(t *testing.T) {
	// 1. Tests with improvement_direction in ParamSet (no fallback used)
	t.Run("with_improvement_direction", func(t *testing.T) {
		tests := []struct {
			direction string
			status    stepfit.StepFitStatus
			expected  bool
		}{
			{"up", stepfit.HIGH, true},
			{"up", stepfit.LOW, false},
			{"down", stepfit.LOW, true},
			{"down", stepfit.HIGH, false},
			{"invalid", stepfit.HIGH, false},
		}

		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s_with_%s", tt.direction, tt.status), func(t *testing.T) {
				r := NewRegression()
				r.Frame = &frame.FrameResponse{
					DataFrame: &dataframe.DataFrame{
						ParamSet: paramtools.ParamSet{"improvement_direction": []string{tt.direction}}.Freeze(),
					},
				}
				setRegressionStatus(r, tt.status)
				assert.Equal(t, tt.expected, r.DetermineIsImprovement("UP")) // Pass fallback, should be ignored
			})
		}
	})

	// 2. Tests without improvement_direction (should use fallback)
	t.Run("fallback", func(t *testing.T) {
		tests := []struct {
			fallback string
			status   stepfit.StepFitStatus
			expected bool
		}{
			{"UP", stepfit.HIGH, true},
			{"UP", stepfit.LOW, false},
			{"DOWN", stepfit.LOW, true},
			{"DOWN", stepfit.HIGH, false},
			{"BOTH", stepfit.HIGH, false},
			{"", stepfit.HIGH, false},
		}

		for _, nilFrame := range []bool{false, true} {
			namePrefix := "empty_frame"
			if nilFrame {
				namePrefix = "nil_frame"
			}
			for _, tt := range tests {
				t.Run(fmt.Sprintf("%s_%s_with_%s", namePrefix, tt.fallback, tt.status), func(t *testing.T) {
					r := NewRegression()
					if !nilFrame {
						r.Frame = &frame.FrameResponse{
							DataFrame: &dataframe.DataFrame{
								ParamSet: paramtools.ParamSet{}.Freeze(),
							},
						}
					}
					setRegressionStatus(r, tt.status)
					assert.Equal(t, tt.expected, r.DetermineIsImprovement(tt.fallback))
				})
			}
		}
	})
}

func setRegressionStatus(r *Regression, status stepfit.StepFitStatus) {
	cl := &clustering2.ClusterSummary{
		StepFit: &stepfit.StepFit{
			Status: status,
		},
	}
	if status == stepfit.HIGH {
		r.High = cl
	} else {
		r.Low = cl
	}
}
