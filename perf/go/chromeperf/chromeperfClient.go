package chromeperf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	slices0 "slices"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/exp/slices"
	"golang.org/x/oauth2/google"
)

const (
	// Change to "http://localhost:8080" for local testing of skia-bridge.
	chromePerfBaseUrl       = "https://skia-bridge-dot-chromeperf.appspot.com"
	chromePerfLegacyBaseUrl = "https://chromeperf.appspot.com"
)

// ChromePerfClient defines an interface for accessing chromeperf apis.
type ChromePerfClient interface {
	// SendGetRequest sends a GET request to chromeperf api with the specified parameters.
	// The url is of the format <host>/{apiName}/{functionName}?{queryParams}.
	// The response from the api is unmarshalled into the provided response object.
	SendGetRequest(ctx context.Context, apiName string, functionName string, queryParams url.Values, response interface{}) error

	// SendPostRequest sends a POST request to chromeperf api with the specified parameters.
	// The url is of the format <host>/{apiName}/{functionName}.
	// The {requestObj} is marshalled into JSON and added to the body of the http object.
	// The response from the api is unmarshalled into the provided response object.
	// {acceptedStatusCodes} is a list of HTTP response codes that are considered successful. The function will return an error if any other status code is returned.
	SendPostRequest(ctx context.Context, apiName string, functionName string, requestObj interface{}, responseObj interface{}, acceptedStatusCodes []int) error
}

// chromePerfClientImpl implements ChromePerfClient.
type chromePerfClientImpl struct {
	httpClient  *http.Client
	urlOverride string
	// If true, requests are sent to chromeperf without skia-bridge
	directCallLegacy bool
}

// NewChromePerfClient creates a new instance of ChromePerfClient.
func NewChromePerfClient(ctx context.Context, urlOverride string, directCall bool) (ChromePerfClient, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create chrome perf client.")
	}
	return &chromePerfClientImpl{
		httpClient:       httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client(),
		urlOverride:      urlOverride,
		directCallLegacy: directCall,
	}, nil
}

func isAllowedStatusCode(statusCode int) bool {
	allowedSuccessCodes := []int{
		http.StatusOK,                   // 200
		http.StatusNoContent,            // 204
		http.StatusPartialContent,       // 206
		http.StatusNonAuthoritativeInfo, // 203
		http.StatusNotModified,          // 304
	}

	return slices0.Contains(allowedSuccessCodes, statusCode)
}

// SendGetRequest sends a GET request to chromeperf api with the specified parameters.
func (client *chromePerfClientImpl) SendGetRequest(ctx context.Context, apiName string, functionName string, queryParams url.Values, response interface{}) error {
	targetUrl := generateTargetUrl(client.urlOverride, client.directCallLegacy, apiName, functionName)
	targetUrl = fmt.Sprintf("%s?%s", targetUrl, queryParams.Encode())

	// TODO(b/475429610) this is for monitoring chromeperf usage on the ng instances.
	sklog.Warningf("chromeperf usage warning: sending get request to %s", targetUrl)
	httpResponse, err := httputils.GetWithContext(ctx, client.httpClient, targetUrl)
	if err != nil {
		return skerr.Wrapf(err, "Failed to get chrome perf response.")
	}

	// Ensure the response body is closed no matter how the function exits.
	defer httpResponse.Body.Close()

	if !isAllowedStatusCode(httpResponse.StatusCode) {
		return skerr.Fmt("chrome perf request failed: %v", httpResponse.StatusCode)
	}

	respBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return skerr.Wrapf(err, "Failed to read body from chrome perf response.")
	}

	err = json.Unmarshal([]byte(respBody), &response)
	if err != nil {
		return skerr.Wrapf(err, "Failed to parse chrome perf response body.")
	}

	return nil
}

// SendPostRequest sends a POST request to chromeperf api with the specified parameters.
func (client *chromePerfClientImpl) SendPostRequest(ctx context.Context, apiName string, functionName string, requestObj interface{}, responseObj interface{}, acceptedStatusCodes []int) error {
	ctx, span := trace.StartSpan(ctx, "chromeperf.chromePerfClientImpl.SendPostRequest")
	defer span.End()

	targetUrl := generateTargetUrl(client.urlOverride, client.directCallLegacy, apiName, functionName)

	var buff bytes.Buffer
	err := json.NewEncoder(&buff).Encode(requestObj)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create chrome perf request.")
	}
	requestBodyJSONStr := buff.String()
	sklog.Debugf("Sending Post request to chromePerf: %s", requestBodyJSONStr)

	// TODO(b/475429610) this is for monitoring chromeperf usage on the ng instances.
	sklog.Warningf("chromeperf usage warning: sending post request to %s", targetUrl)
	httpResponse, err := httputils.PostWithContext(
		ctx,
		client.httpClient,
		targetUrl,
		"application/json",
		strings.NewReader(string(requestBodyJSONStr)))
	if err != nil {
		return skerr.Wrapf(err, "Failed to get chrome perf response.")
	}

	// Ensure the response body is closed no matter how the function exits.
	defer httpResponse.Body.Close()

	if !slices.Contains(acceptedStatusCodes, httpResponse.StatusCode) {
		return skerr.Fmt("Receive status %d from chromeperf", httpResponse.StatusCode)
	}
	if err := json.NewDecoder(httpResponse.Body).Decode(&responseObj); err != nil {
		return skerr.Wrapf(err, "Failed to parse chrome perf response body.")
	}

	return nil
}

func generateTargetUrl(urlOverride string, directCallLegacy bool, apiName string, functionName string) string {
	if len(urlOverride) > 0 {
		return urlOverride
	}

	if directCallLegacy {
		return fmt.Sprintf("%s/%s", chromePerfLegacyBaseUrl, apiName)
	}
	return fmt.Sprintf("%s/%s/%s", chromePerfBaseUrl, apiName, functionName)
}
