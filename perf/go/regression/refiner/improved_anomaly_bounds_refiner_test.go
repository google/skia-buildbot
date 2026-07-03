package refiner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

func TestApplyImprovedLogic_NoPrevRegression_ReturnsOriginal(t *testing.T) {
	r := &ImprovedAnomalyBoundsRefiner{}

	cr := &regression.ConfirmedRegression{
		DisplayCommitNumber: 100,
		Summary: &clustering2.ClusterSummaries{
			Clusters: []*clustering2.ClusterSummary{
				{Keys: []string{"trace1"}},
			},
		},
	}

	res := r.applyImprovedLogic(cr, &alerts.Alert{}, nil, nil, nil)

	assert.Equal(t, cr, res)
}

func TestApplyImprovedLogic_OverlapWithDB_FiltersOut(t *testing.T) {
	r := &ImprovedAnomalyBoundsRefiner{}

	cr := &regression.ConfirmedRegression{
		DisplayCommitNumber: 100,
		PrevCommitNumber:    90,
		CommitNumber:        100,
		Summary: &clustering2.ClusterSummaries{
			Clusters: []*clustering2.ClusterSummary{
				{Keys: []string{"trace1"}},
			},
		},
	}

	dbReg := &regression.Regression{
		CommitNumber:     95,
		PrevCommitNumber: 85,
	}
	batchPrev := map[string]map[types.CommitNumber]*regression.Regression{
		"trace1": {100: dbReg},
	}

	res := r.applyImprovedLogic(cr, &alerts.Alert{}, nil, batchPrev, nil)

	assert.Nil(t, res) // Filtered out
}

func TestApplyImprovedLogic_OverlapWithInMemory_FiltersOut(t *testing.T) {
	r := &ImprovedAnomalyBoundsRefiner{}

	cr := &regression.ConfirmedRegression{
		DisplayCommitNumber: 100,
		PrevCommitNumber:    90,
		CommitNumber:        100,
		Summary: &clustering2.ClusterSummaries{
			Clusters: []*clustering2.ClusterSummary{
				{Keys: []string{"trace1"}},
			},
		},
	}

	latestRefined := &regression.ConfirmedRegression{
		CommitNumber:     95,
		PrevCommitNumber: 85,
	}

	// Mock DB to return older regression
	dbReg := &regression.Regression{
		CommitNumber:     50,
		PrevCommitNumber: 40,
	}
	batchPrev := map[string]map[types.CommitNumber]*regression.Regression{
		"trace1": {100: dbReg},
	}

	res := r.applyImprovedLogic(cr, &alerts.Alert{}, latestRefined, batchPrev, nil)

	assert.Nil(t, res) // Filtered out
}

func TestApplyImprovedLogic_NilBatchPrev_ReturnsOriginal(t *testing.T) {
	r := &ImprovedAnomalyBoundsRefiner{}

	cr := &regression.ConfirmedRegression{
		DisplayCommitNumber: 100,
		Summary: &clustering2.ClusterSummaries{
			Clusters: []*clustering2.ClusterSummary{
				{Keys: []string{"trace1"}},
			},
		},
	}

	res := r.applyImprovedLogic(cr, &alerts.Alert{}, nil, nil, nil)

	assert.Equal(t, cr, res) // Fallback because no prev regression found in memory either
}

func TestApplyImprovedLogic_CohenThreshold_Capped(t *testing.T) {
	r := &ImprovedAnomalyBoundsRefiner{
		stdDevThreshold: 0.001,
	}

	cr := &regression.ConfirmedRegression{
		DisplayCommitNumber: 100,
		PrevCommitNumber:    90,
		CommitNumber:        100,
		Summary: &clustering2.ClusterSummaries{
			Clusters: []*clustering2.ClusterSummary{
				{
					Keys:     []string{"trace1"},
					StepFit:  &stepfit.StepFit{TurningPoint: 2},
					Centroid: []float32{10, 10, 20, 20},
				},
			},
		},
		RightMostSummary: &clustering2.ClusterSummaries{
			Clusters: []*clustering2.ClusterSummary{
				{
					Keys:     []string{"trace1"},
					StepFit:  &stepfit.StepFit{TurningPoint: 2},
					Centroid: []float32{10, 10, 20, 20},
				},
			},
		},
		RightMostFrame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				Header: []*dataframe.ColumnHeader{
					{Offset: 90}, {Offset: 91}, {Offset: 92}, {Offset: 93},
				},
			},
		},
	}

	dbReg := &regression.Regression{
		CommitNumber:     50,
		PrevCommitNumber: 40,
	}
	batchPrev := map[string]map[types.CommitNumber]*regression.Regression{
		"trace1": {100: dbReg},
	}

	pData := &prefetchedData{
		traces: map[string]types.Trace{
			"trace1": {10, 10, 10, 10},
		},
		commits: map[string][]types.CommitNumber{
			"trace1": {50, 60, 70, 80},
		},
	}

	alert := &alerts.Alert{
		Step:        types.CohenStep,
		Interesting: 2.0, // Alert wants 2.0, but we cap at 1.2 in refiner!
		Radius:      2,
	}

	res := r.applyImprovedLogic(cr, alert, nil, batchPrev, pData)

	assert.NotNil(t, res)
}

func TestFindPreviousRegression_PrefersConfirmedPredecessor(t *testing.T) {
	r := &ImprovedAnomalyBoundsRefiner{}

	// DB regression at commit 1
	dbReg1 := &regression.Regression{CommitNumber: 1, PrevCommitNumber: 0}
	// DB regression at commit 51
	dbReg51 := &regression.Regression{CommitNumber: 51, PrevCommitNumber: 50}

	batchPrev := map[string]map[types.CommitNumber]*regression.Regression{
		"trace1": {
			31: dbReg1,
			41: dbReg1,
			61: dbReg51,
		},
	}

	// Suppose candidate 31 was already confirmed by applyImprovedLogic
	confirmed31 := &regression.ConfirmedRegression{
		CommitNumber:     31,
		PrevCommitNumber: 30,
	}

	// Now we evaluate findPreviousRegression for candidate at 41
	prevInfo41 := r.findPreviousRegression("trace1", 41, confirmed31, batchPrev)

	require.NotNil(t, prevInfo41)
	assert.Equal(t, types.CommitNumber(31), prevInfo41.CommitNumber)
	assert.Equal(t, "in-memory", prevInfo41.Source)

	// Evaluate findPreviousRegression for candidate at 61 (where DB 51 > confirmed 31)
	prevInfo61 := r.findPreviousRegression("trace1", 61, confirmed31, batchPrev)

	require.NotNil(t, prevInfo61)
	assert.Equal(t, types.CommitNumber(51), prevInfo61.CommitNumber)
	assert.Equal(t, "DB", prevInfo61.Source)
}
