package notify

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/ui/frame"
)

// ChromeperfNotifier struct used to send regression data to chromeperf.
type ChromePerfNotifier struct {
	chromePerfClient chromeperf.AnomalyApiClient
}

// NewChromePerfNotifier returns a new ChromePerfNotifier instance.
func NewChromePerfNotifier(ctx context.Context, anomalyApiClient chromeperf.AnomalyApiClient) (*ChromePerfNotifier, error) {
	var err error
	if anomalyApiClient == nil {
		anomalyApiClient, err = chromeperf.NewAnomalyApiClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	sklog.Info("Creating a new chromeperf notifier")

	return &ChromePerfNotifier{
		chromePerfClient: anomalyApiClient,
	}, nil
}

// RegressionFound implements notify.Notifier.
// Invoked when a new regression is detected.
func (n *ChromePerfNotifier) RegressionFound(
	ctx context.Context,
	commit,
	previousCommit provider.Commit,
	alert *alerts.Alert,
	cl *clustering2.ClusterSummary,
	frame *frame.FrameResponse,
	regressionID string) (string, error) {

	sklog.Infof("%d traces in regression found information for alert %s", len(frame.DataFrame.TraceSet), alert.DisplayName)

	anomalyIds := []string{}
	for key := range frame.DataFrame.TraceSet {
		paramset, err := query.ParseKey(key)
		if err != nil {
			return "", skerr.Wrapf(err, "Error parsing key %s", key)
		}

		if !isParamSetValid(paramset) {
			return "", skerr.Fmt("Invalid paramset %s for chromeperf", paramset)
		}

		medianBeforeAnomaly, _, _, _ := vec32.TwoSidedStdDev(cl.Centroid[:cl.StepFit.TurningPoint])
		medianAfterAnomaly, _, _, _ := vec32.TwoSidedStdDev(cl.Centroid[cl.StepFit.TurningPoint:])
		sklog.Infof("Median Before: %f, Median After: %f", medianBeforeAnomaly, medianAfterAnomaly)
		response, err := n.chromePerfClient.ReportRegression(
			ctx,
			getTestPath(paramset),
			int32(previousCommit.CommitNumber),
			int32(commit.CommitNumber),
			"chromium",
			isRegressionImprovement(paramset, cl.StepFit.Status),
			paramset["bot"],
			true,
			medianBeforeAnomaly,
			medianAfterAnomaly)

		if err != nil {
			return "", err
		}
		anomalyIds = append(anomalyIds, response.AnomalyId)
	}

	return strings.Join(anomalyIds, ","), nil
}

// RegressionMissing implements notify.Notifier.
// Invoked when a previous regression is recovered.
func (n *ChromePerfNotifier) RegressionMissing(
	ctx context.Context,
	commit,
	previousCommit provider.Commit,
	alert *alerts.Alert,
	cl *clustering2.ClusterSummary,
	frame *frame.FrameResponse,
	threadingReference string) error {
	sklog.Info("Sending regression missing information to Chromeperf")
	for key := range frame.DataFrame.TraceSet {
		paramset, err := query.ParseKey(key)
		if err != nil {
			return skerr.Wrapf(err, "Error parsing key %s", key)
		}

		if !isParamSetValid(paramset) {
			const errorFormat = "Invalid paramset %s for chromeperf"
			sklog.Debugf(errorFormat, paramset)
			return skerr.Fmt(errorFormat, paramset)
		}

		medianBeforeAnomaly, _, _, _ := vec32.TwoSidedStdDev(cl.Centroid[:cl.StepFit.TurningPoint])
		medianAfterAnomaly, _, _, _ := vec32.TwoSidedStdDev(cl.Centroid[cl.StepFit.TurningPoint:])
		sklog.Infof("Median Before: %f, Median After: %f", medianBeforeAnomaly, medianAfterAnomaly)
		_, err = n.chromePerfClient.ReportRegression(
			ctx,
			getTestPath(paramset),
			int32(previousCommit.CommitNumber),
			int32(commit.CommitNumber),
			"chromium",
			isRegressionImprovement(paramset, cl.StepFit.Status),
			paramset["bot"],
			true,
			medianBeforeAnomaly,
			medianAfterAnomaly)
		if err != nil {
			return err
		}
	}

	return nil
}

// ExampleSend is for dummy data. Do nothing!
func (n *ChromePerfNotifier) ExampleSend(ctx context.Context, alert *alerts.Alert) error {
	sklog.Info("Doing example send on chromeperf notifier")
	return nil
}

// UpdateRegressionNotification implements Transport.
func (n *ChromePerfNotifier) UpdateNotification(ctx context.Context, commit, previousCommit provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, frame *frame.FrameResponse, notificationId string) error {
	return nil
}

// isParamSetValid returns true if the paramsets contains all the
// keys required by chromeperf api.
func isParamSetValid(paramset map[string]string) bool {
	requiredKeys := []string{"master", "bot", "benchmark", "test", "subtest_1"}
	for _, key := range requiredKeys {
		_, ok := paramset[key]
		if !ok {
			return false
		}
	}

	return true
}

// isRegressionImprovement returns true if the metric has moved towards the improvement direction.
func isRegressionImprovement(paramset map[string]string, stepFitStatus stepfit.StepFitStatus) bool {
	if _, ok := paramset["improvement_direction"]; ok {
		improvementDirection := paramset["improvement_direction"]
		return improvementDirection == "down" && stepFitStatus == stepfit.LOW || improvementDirection == "up" && stepFitStatus == stepfit.HIGH
	}

	return false
}

// GetTestPath returns a test path based on the values found in the paramset.
func getTestPath(paramset map[string]string) string {
	keys := []string{"bot", "benchmark", "test", "subtest_1", "subtest_2", "subtest_3"}
	testPath := paramset["master"]
	for _, key := range keys {
		val, ok := paramset[key]
		if ok {
			testPath = fmt.Sprintf("%s/%s", testPath, val)
		} else {
			break
		}
	}
	return testPath
}
