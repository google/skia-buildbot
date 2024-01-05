package chromeperf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/exp/slices"
	"golang.org/x/oauth2/google"
)

const (
	chromePerfBaseUrl = "https://skia-bridge-dot-chromeperf.appspot.com/"
)

// chromePerfClient defines an interface for accessing chromeperf apis.
type chromePerfClient interface {
	// sendGetRequest sends a GET request to chromeperf api with the specified parameters.
	// The url is of the format <host>/{apiName}/{functionName}?{queryParams}.
	// The response from the api is unmarshalled into the provided response object.
	sendGetRequest(ctx context.Context, apiName string, functionName string, queryParams url.Values, response interface{}) error

	// sendPostRequest sends a POST request to chromeperf api with the specified parameters.
	// The url is of the format <host>/{apiName}/{functionName}.
	// The {requestObj} is marshalled into JSON and added to the body of the http object.
	// The response from the api is unmarshalled into the provided response object.
	// {acceptedStatusCodes} is a list of HTTP response codes that are considered successful. The function will return an error if any other status code is returned.
	sendPostRequest(ctx context.Context, apiName string, functionName string, requestObj interface{}, responseObj interface{}, acceptedStatusCodes []int) error
}

// chromePerfClientImpl implements ChromePerfClient.
type chromePerfClientImpl struct {
	httpClient  *http.Client
	urlOverride string
}

// NewChromePerfClient creates a new instance of ChromePerfClient.
func newChromePerfClient(ctx context.Context, urlOverride string) (chromePerfClient, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create chrome perf client.")
	}
	return &chromePerfClientImpl{
		httpClient:  httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client(),
		urlOverride: urlOverride,
	}, nil
}

// sendGetRequest sends a GET request to chromeperf api with the specified parameters.
func (client *chromePerfClientImpl) sendGetRequest(ctx context.Context, apiName string, functionName string, queryParams url.Values, response interface{}) error {
	var targetUrl string
	if client.urlOverride != "" {
		targetUrl = client.urlOverride
	} else {
		targetUrl = fmt.Sprintf("%s/%s/%s?%s", chromePerfBaseUrl, apiName, functionName, queryParams.Encode())
	}

	httpResponse, err := httputils.GetWithContext(ctx, client.httpClient, targetUrl)
	if err != nil {
		return skerr.Wrapf(err, "Failed to get chrome perf response.")
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

// sendPostRequest sends a POST request to chromeperf api with the specified parameters.
func (client *chromePerfClientImpl) sendPostRequest(ctx context.Context, apiName string, functionName string, requestObj interface{}, responseObj interface{}, acceptedStatusCodes []int) error {
	var targetUrl string
	if len(client.urlOverride) > 0 {
		targetUrl = client.urlOverride
	} else {
		targetUrl = fmt.Sprintf("%s/%s/%s", chromePerfBaseUrl, apiName, functionName)
	}

	requestBodyJSONStr, err := json.Marshal(requestObj)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create chrome perf request.")
	}

	httpResponse, err := httputils.PostWithContext(
		ctx,
		client.httpClient,
		targetUrl,
		"application/json",
		strings.NewReader(string(requestBodyJSONStr)))
	if err != nil {
		return skerr.Wrapf(err, "Failed to get chrome perf response.")
	}
	if !slices.Contains(acceptedStatusCodes, httpResponse.StatusCode) {
		return skerr.Fmt("Receive status %d from chromeperf", httpResponse.StatusCode)
	}
	if err := json.NewDecoder(httpResponse.Body).Decode(&responseObj); err != nil {
		return skerr.Wrapf(err, "Failed to parse chrome perf response body.")
	}

	return nil
}
