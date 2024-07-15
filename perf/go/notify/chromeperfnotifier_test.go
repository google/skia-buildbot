package notify

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/chromeperf"
	mocks "go.skia.org/infra/perf/go/chromeperf/mock"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

func TestInvalidQuery(t *testing.T) {
	paramset := map[string]string{
		"master":        "m",
		"somethingElse": "invalid",
	}

	testNotifierFunctions_InvalidParams_ReturnsError(paramset, t)
}

func TestMissingParamInQuery(t *testing.T) {
	paramset := map[string]string{
		"master":    "m",
		"benchmark": "b",
		"subtest_1": "s1",
	}
	testNotifierFunctions_InvalidParams_ReturnsError(paramset, t)
}

func TestValidRegression_Success(t *testing.T) {
	paramset := map[string]string{
		"master":                "m",
		"bot":                   "testBot",
		"benchmark":             "b",
		"test":                  "t",
		"subtest_1":             "s",
		"improvement_direction": "down",
	}

	ctx := context.Background()
	mockChromeperfClient := mocks.NewAnomalyApiClient(t)
	startCommit := provider.Commit{
		CommitNumber: 1,
	}
	endCommit := provider.Commit{
		CommitNumber: 10,
	}

	chromePerfResponse := &chromeperf.ReportRegressionResponse{AnomalyId: "123", AlertGroupId: "567"}
	mockChromeperfClient.On("ReportRegression", ctx, "m/testBot/b/t/s", int32(startCommit.CommitNumber), int32(endCommit.CommitNumber), "chromium", false, "testBot", true, mock.Anything, mock.Anything).Return(chromePerfResponse, nil)
	notifier, _ := NewChromePerfNotifier(ctx, mockChromeperfClient)
	key, _ := query.MakeKey(paramset)
	frame := &frame.FrameResponse{}
	frame.DataFrame = &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			key: []float32{1.0, 2.0},
		},
	}
	cl := &clustering2.ClusterSummary{
		Centroid: []float32{1.0, 2.0},
		StepFit:  &stepfit.StepFit{TurningPoint: 1, Status: stepfit.HIGH},
	}
	anomalyId, err := notifier.RegressionFound(ctx, endCommit, startCommit, alerts.NewConfig(), cl, frame, "")
	assert.Nil(t, err, "No error expected")
	assert.Equal(t, chromePerfResponse.AnomalyId, anomalyId)
}

func TestValidRegressionMissing_Success(t *testing.T) {
	paramset := map[string]string{
		"master":                "m",
		"bot":                   "testBot",
		"benchmark":             "b",
		"test":                  "t",
		"subtest_1":             "s",
		"improvement_direction": "down",
	}

	ctx := context.Background()
	mockChromeperfClient := mocks.NewAnomalyApiClient(t)
	startCommit := provider.Commit{
		CommitNumber: 1,
	}
	endCommit := provider.Commit{
		CommitNumber: 10,
	}
	chromePerfResponse := &chromeperf.ReportRegressionResponse{AnomalyId: "123", AlertGroupId: "567"}
	mockChromeperfClient.On("ReportRegression", ctx, "m/testBot/b/t/s", int32(startCommit.CommitNumber), int32(endCommit.CommitNumber), "chromium", true, "testBot", true, mock.Anything, mock.Anything).Return(chromePerfResponse, nil)
	notifier, _ := NewChromePerfNotifier(ctx, mockChromeperfClient)
	key, _ := query.MakeKey(paramset)
	frame := &frame.FrameResponse{}
	frame.DataFrame = &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			key: []float32{1.0, 2.0},
		},
	}

	cl := &clustering2.ClusterSummary{
		Centroid: []float32{1.0, 2.0},
		StepFit:  &stepfit.StepFit{TurningPoint: 1, Status: stepfit.LOW},
	}
	err := notifier.RegressionMissing(ctx, endCommit, startCommit, alerts.NewConfig(), cl, frame, "ref")
	assert.Nil(t, err, "No error expected")
}

func testNotifierFunctions_InvalidParams_ReturnsError(paramset map[string]string, t *testing.T) {
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
	notifier, _ := NewChromePerfNotifier(ctx, mocks.NewAnomalyApiClient(t))
	_, err := notifier.RegressionFound(ctx, provider.Commit{}, provider.Commit{}, alert, cl, frame, "")
	assert.NotNil(t, err, "Error expected due to invalid query")

	err = notifier.RegressionMissing(ctx, provider.Commit{}, provider.Commit{}, alert, cl, frame, "")
	assert.NotNil(t, err, "Error expected due to invalid query")
}
