package notify

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/ui/frame"
)

// ChromeperfNotifier struct used to send regression data to chromeperf.
type ChromePerfNotifier struct {
	chromePerfClient chromeperf.ChromePerfClient
}

// NewChromePerfNotifier returns a new ChromePerfNotifier instance.
func NewChromePerfNotifier(ctx context.Context, cpClient chromeperf.ChromePerfClient) (*ChromePerfNotifier, error) {

	var err error
	if cpClient == nil {
		cpClient, err = chromeperf.NewChromePerfClient(ctx, "")
		if err != nil {
			return nil, skerr.Wrapf(err, "Error creating a new chromeperf client.")
		}
	}

	sklog.Info("Creating a new chromeperf notifier")

	return &ChromePerfNotifier{
		chromePerfClient: cpClient,
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
	frame *frame.FrameResponse) (string, error) {

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

		response, err := n.chromePerfClient.SendRegression(
			ctx,
			getTestPath(paramset),
			int32(previousCommit.CommitNumber),
			int32(commit.CommitNumber),
			"chromium",
			false,
			paramset["bot"],
			true)

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

		_, err = n.chromePerfClient.SendRegression(
			ctx,
			getTestPath(paramset),
			int32(previousCommit.CommitNumber),
			int32(commit.CommitNumber),
			"chromium",
			true,
			paramset["bot"],
			true)
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
