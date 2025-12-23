package notifier

import (
	"context"
	"fmt"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/alerts"
	ag "go.skia.org/infra/perf/go/anomalygroup/utils"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/git/provider"
	perf_issuetracker "go.skia.org/infra/perf/go/issuetracker"
	"go.skia.org/infra/perf/go/ui/frame"
)

// AnomalyGroupNotifier struct used to group regression and trigger group action.
type AnomalyGroupNotifier struct {
	grouper ag.AnomalyGrouper
}

// NewAnomalyGroupNotifier returns a new AnomalyGroupNotifier instance.
func NewAnomalyGroupNotifier(ctx context.Context, anomalygrouper ag.AnomalyGrouper, issuetracker perf_issuetracker.IssueTracker) *AnomalyGroupNotifier {
	if anomalygrouper == nil {
		anomalygrouper = ag.New(issuetracker)
	}

	sklog.Info("Creating a new anomaly group notifier")

	return &AnomalyGroupNotifier{
		grouper: anomalygrouper,
	}
}

// RegressionFound implements notify.Notifier.
// Invoked when a new regression is detected.
func (n *AnomalyGroupNotifier) RegressionFound(
	ctx context.Context,
	commit,
	previousCommit provider.Commit,
	alert *alerts.Alert,
	cl *clustering2.ClusterSummary,
	frame *frame.FrameResponse,
	regressionID string) (string, error) {
	ctx, span := trace.StartSpan(ctx, "anomalygroupnotifier.RegressionFound")
	defer span.End()

	sklog.Infof("[AG] %d traces in regression found information for alert %s", len(frame.DataFrame.TraceSet), alert.DisplayName)

	// If the the trace is on a summary level, e.g., browse_media without specific story, we may
	// have multiple trace returned by the query. We will ignore those summary level data in anomaly
	// grouping.
	if len(frame.DataFrame.TraceSet) > 1 {
		traceset_keys := make([]string, len(frame.DataFrame.TraceSet))
		i := 0
		for traceset_key := range frame.DataFrame.TraceSet {
			traceset_keys[i] = traceset_key
			i++
		}
		sklog.Debugf("[AG] Ignore regression on summary level. Anomaly: %s. Keys: %s", regressionID, traceset_keys)
		return "", nil
	}

	for key := range frame.DataFrame.TraceSet {
		paramset, err := query.ParseKey(key)
		if err != nil {
			return "", skerr.Wrapf(err, "Error parsing key %s", key)
		}

		if !isParamSetValid(paramset) {
			return "", skerr.Fmt("Invalid paramset %s for chromeperf data", paramset)
		}

		medianBeforeAnomaly, _, _, _ := vec32.TwoSidedStdDev(cl.Centroid[:cl.StepFit.TurningPoint])
		medianAfterAnomaly, _, _, _ := vec32.TwoSidedStdDev(cl.Centroid[cl.StepFit.TurningPoint:])
		sklog.Infof("Median Before: %f, Median After: %f", medianBeforeAnomaly, medianAfterAnomaly)

		testPath := getTestPath(paramset)

		_, err = n.grouper.ProcessRegressionInGroup(
			ctx,
			alert,
			regressionID,
			int64(previousCommit.CommitNumber+1),
			int64(commit.CommitNumber),
			testPath,
			paramset)
		if err != nil {
			return "", skerr.Wrapf(err, "error processing regression")
		}
	}

	return "", nil
}

// RegressionMissing implements notify.Notifier.
// Invoked when a previous regression is recovered.
func (n *AnomalyGroupNotifier) RegressionMissing(
	ctx context.Context,
	commit,
	previousCommit provider.Commit,
	alert *alerts.Alert,
	cl *clustering2.ClusterSummary,
	frame *frame.FrameResponse,
	threadingReference string) error {

	sklog.Info("No op function for AnomalyGroupNotifier.RegressionMissing")
	return nil
}

// ExampleSend is for dummy data. Do nothing!
func (n *AnomalyGroupNotifier) ExampleSend(ctx context.Context, alert *alerts.Alert) error {
	sklog.Info("No op function for AnomalyGroupNotifier.ExampleSend")
	return nil
}

// UpdateRegressionNotification implements Transport.
func (n *AnomalyGroupNotifier) UpdateNotification(ctx context.Context, commit, previousCommit provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, frame *frame.FrameResponse, notificationId string) error {
	return nil
}

// GetTestPath returns a test path based on the values found in the paramset.
// TODO(wenbinzhang): using kvp in config to control which keys to expect
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

// isParamSetValid returns true if the paramsets contains all the
// keys required by chromeperf api.
// TODO(wenbinzhang): using kvp in config to control which keys to expect
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
