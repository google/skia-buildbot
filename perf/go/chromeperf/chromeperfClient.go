package chromeperf

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
)

const (
	chromePerfBaseUrl = "https://skia-bridge-dot-chromeperf.appspot.com/"
)

// ChromePerfClient defines an interface for accessing chromeperf apis.
type ChromePerfClient interface {
	SendRegression(
		ctx context.Context,
		testPath string,
		startCommitPosition int32,
		endCommitPosition int32,
		projectId string,
		isImprovement bool,
		botName string,
		internal bool,
		medianBefore float32,
		medianAfter float32) (*ChromePerfResponse, error)
}

// ChromePerfClientImpl struct is used to handle the actual api call.
type ChromePerfClientImpl struct {
	httpClient        *http.Client
	urlOverride       string
	sendAnomalyCalled metrics2.Counter
	sendAnomalyFailed metrics2.Counter
}

// NewChromePerfClient creates a new instance of ChromePerfClient.
func NewChromePerfClient(ctx context.Context, urlOverride string) (ChromePerfClient, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create chrome perf client.")
	}
	return &ChromePerfClientImpl{
		httpClient:        httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client(),
		urlOverride:       urlOverride,
		sendAnomalyCalled: metrics2.GetCounter("chrome_perf_send_anomaly_called"),
		sendAnomalyFailed: metrics2.GetCounter("chrome_perf_send_anomaly_failed"),
	}, nil
}

// SendRegression implements ChromePerfClient
// Sends the regression data by invoking the add anomaly api.
func (cp *ChromePerfClientImpl) SendRegression(
	ctx context.Context,
	testPath string,
	startCommitPosition int32,
	endCommitPosition int32,
	projectId string,
	isImprovement bool,
	botName string,
	internal bool,
	medianBefore float32,
	medianAfter float32) (*ChromePerfResponse, error) {
	request := &ChromePerfRequest{
		TestPath:            testPath,
		StartRevision:       startCommitPosition,
		EndRevision:         endCommitPosition,
		ProjectID:           projectId,
		IsImprovement:       isImprovement,
		BotName:             botName,
		Internal:            internal,
		MedianBeforeAnomaly: medianBefore,
		MedianAfterAnomaly:  medianAfter,
	}
	requestBodyJSONStr, err := json.Marshal(request)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create chrome perf request.")
	}
	sklog.Infof("Sending the json to chromeperf: %s", requestBodyJSONStr)
	chromePerfUrl := AddAnomalyUrl
	if len(cp.urlOverride) > 0 {
		chromePerfUrl = cp.urlOverride
	}

	httpResponse, err := httputils.PostWithContext(
		ctx,
		cp.httpClient,
		chromePerfUrl,
		"application/json",
		strings.NewReader(string(requestBodyJSONStr)))
	if err != nil {
		cp.sendAnomalyFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to get chrome perf response.")
	}

	acceptedStatusCodes := []int{
		200, // Success
		404, // NotFound - This is returned if the param value names are different.
	}
	if !slices.Contains(acceptedStatusCodes, httpResponse.StatusCode) {
		cp.sendAnomalyFailed.Inc(1)
		return nil, skerr.Fmt("Receive status %d from chromeperf", httpResponse.StatusCode)
	}

	var resp ChromePerfResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&resp); err != nil {
		cp.sendAnomalyFailed.Inc(1)
		return nil, skerr.Wrapf(err, "Failed to parse chrome perf response body.")
	}

	cp.sendAnomalyCalled.Inc(1)
	return &resp, nil
}
