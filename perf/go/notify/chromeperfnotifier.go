package notify

import (
	"context"
	"fmt"

	"go.skia.org/infra/go/paramtools"
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

	sklog.Infof("Sending regression found information to Chromeperf for alert %s", alert.DisplayName)

	// frame.DataFrame.ParamSet contains all the parameters expanded from the alert.Query
	// and this is used to get the test metadata for the regression.
	paramset := frame.DataFrame.ParamSet
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
		paramset["bot"][0],
		true)

	if err != nil {
		return "", err
	}
	return response.AnomalyId, nil
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
	paramset := frame.DataFrame.ParamSet
	if !isParamSetValid(paramset) {
		const errorFormat = "Invalid paramset %s for chromeperf"
		sklog.Debugf(errorFormat, paramset)
		return skerr.Fmt(errorFormat, paramset)
	}

	_, err := n.chromePerfClient.SendRegression(
		ctx,
		getTestPath(paramset),
		int32(previousCommit.CommitNumber),
		int32(commit.CommitNumber),
		"chromium",
		true,
		paramset["bot"][0],
		true)
	if err != nil {
		return err
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
func isParamSetValid(paramset paramtools.ReadOnlyParamSet) bool {
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
func getTestPath(paramset paramtools.ReadOnlyParamSet) string {
	keys := []string{"bot", "benchmark", "test", "subtest_1", "subtest_2", "subtest_3"}
	testPath := paramset["master"][0]
	for _, key := range keys {
		vals, ok := paramset[key]
		if ok {
			testPath = fmt.Sprintf("%s/%s", testPath, vals[0])
		} else {
			break
		}
	}
	return testPath
}
