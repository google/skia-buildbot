package alertgroup

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2/google"
)

const (
	chromePerfURL = "https://skia-bridge-dot-chromeperf.appspot.com/alert_group/details"
)

// ChromePerfClient implements alertgroup.Service.
type ChromePerfClient struct {
	httpClient                 *http.Client
	getAlertGroupDetailsCalled metrics2.Counter
	getAlertGroupDetailsFailed metrics2.Counter
}

// New returns a new ChromePerf instance.
func New(ctx context.Context) (*ChromePerfClient, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create chrome perf client.")
	}

	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()
	return &ChromePerfClient{
		httpClient:                 client,
		getAlertGroupDetailsCalled: metrics2.GetCounter("chrome_perf_get_alertgroup_details_called"),
		getAlertGroupDetailsFailed: metrics2.GetCounter("chrome_perf_get_alertgroup_details_failed"),
	}, nil
}

// GetAlertGroupDetails implements ChromePerf, it calls chrome perf API to get the details of specific alert groups
func (cp *ChromePerfClient) GetAlertGroupDetails(ctx context.Context, groupKey string) (*AlertGroupDetails, error) {
	if groupKey != "" {
		cp.getAlertGroupDetailsCalled.Inc(1)
		// Call Chrome Perf API to fetch alert group details
		chromePerfResp, err := cp.callChromePerf(ctx, groupKey)
		if err != nil {
			cp.getAlertGroupDetailsFailed.Inc(1)
			return nil, skerr.Wrapf(err, "Failed to call chrome perf endpoint.")
		}
		return chromePerfResp, nil
	}

	return nil, nil
}

// callChromePerf implements the call to chromeperf api
func (cp *ChromePerfClient) callChromePerf(ctx context.Context, groupKey string) (*AlertGroupDetails, error) {
	url := chromePerfURL + "?key=" + groupKey

	httpResponse, err := httputils.GetWithContext(ctx, cp.httpClient, url)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get chrome perf response.")
	}

	respBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to read body from chrome perf response.")
	}

	resp := AlertGroupDetails{}
	err = json.Unmarshal([]byte(respBody), &resp)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse chrome perf response body.")
	}

	return &resp, nil
}
