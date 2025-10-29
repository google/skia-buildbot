package notifier

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/alerts"
	ag_mock "go.skia.org/infra/perf/go/anomalygroup/utils/mocks"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/git/provider"
	it_mock "go.skia.org/infra/perf/go/issuetracker/mocks"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

func Setup(paramset map[string]string) (context.Context, *alerts.Alert, frame.FrameResponse, clustering2.ClusterSummary) {
	ctx := context.Background()
	alert := alerts.NewConfig()
	frame := &frame.FrameResponse{}
	key, _ := query.MakeKey(paramset)
	frame.DataFrame = &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			key: []float32{1.0, 2.0},
		},
	}
	cl := &clustering2.ClusterSummary{
		Centroid: []float32{1.0, 2.0},
		StepFit:  &stepfit.StepFit{TurningPoint: 1},
	}

	return ctx, alert, *frame, *cl
}

func TestSuccess(t *testing.T) {
	paramset := map[string]string{
		"master":    "m",
		"bot":       "b",
		"benchmark": "be",
		"test":      "me",
		"subtest_1": "t",
	}
	ctx, alert, frame, cl := Setup(paramset)
	mockAnomalyGrouper := ag_mock.NewAnomalyGrouper(t)
	mockIssuetracker := it_mock.NewIssueTracker(t)
	ag_notifier := NewAnomalyGroupNotifier(ctx, mockAnomalyGrouper, mockIssuetracker)
	regression_id := "550c78a3-ff99-4f28-8a46-106f81a34840"
	mockAnomalyGrouper.On("ProcessRegressionInGroup",
		ctx, alert, regression_id, int64(101), int64(200), "m/b/be/me/t", paramset).Return("", nil)

	_, err := ag_notifier.RegressionFound(ctx, provider.Commit{CommitNumber: 200}, provider.Commit{CommitNumber: 100}, alert, &cl, &frame, regression_id)
	assert.NoError(t, err)
}

func TestInvalidParamSet(t *testing.T) {
	paramset := map[string]string{
		"master": "mAsTeR",
		"bot":    "bOt",
		"test":   "tEsT",
	}
	ctx, alert, frame, cl := Setup(paramset)
	mockAnomalyGrouper := ag_mock.NewAnomalyGrouper(t)
	mockIssuetracker := it_mock.NewIssueTracker(t)
	ag_notifier := NewAnomalyGroupNotifier(ctx, mockAnomalyGrouper, mockIssuetracker)

	_, err := ag_notifier.RegressionFound(ctx, provider.Commit{}, provider.Commit{}, alert, &cl, &frame, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid paramset")
}

func TestFailedProcess(t *testing.T) {
	paramset := map[string]string{
		"master":    "m",
		"bot":       "b",
		"benchmark": "be",
		"test":      "me",
		"subtest_1": "t",
	}
	ctx, alert, frame, cl := Setup(paramset)
	mockAnomalyGrouper := ag_mock.NewAnomalyGrouper(t)
	mockIssuetracker := it_mock.NewIssueTracker(t)
	ag_notifier := NewAnomalyGroupNotifier(ctx, mockAnomalyGrouper, mockIssuetracker)
	regression_id := "550c78a3-ff99-4f28-8a46-106f81a34840"
	mockAnomalyGrouper.On("ProcessRegressionInGroup",
		ctx, alert, regression_id, int64(101), int64(200), "m/b/be/me/t", paramset).Return("", errors.New(("oops")))

	_, err := ag_notifier.RegressionFound(ctx, provider.Commit{CommitNumber: 200}, provider.Commit{CommitNumber: 100}, alert, &cl, &frame, regression_id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error processing regression")
}
