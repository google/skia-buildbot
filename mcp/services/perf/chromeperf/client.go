package chromeperf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	scomm "go.skia.org/infra/mcp/services/common"
)

const (
	ChromeperfUrl = "https://chromeperf.appspot.com"
	ConfigAPI     = "/api/config"
	DescribeAPI   = "/api/describe"
	// There's no response object for /api/test_suites because the
	// return onject is a literal []string.
	TestSuitesAPI = "/api/test_suites"
)

// ChromeperfClient is a lightweight wrapper.
type ChromeperfClient struct {
	// Full URL of Chromeperf to target, usually "https://chromeperf.appspot.com"
	Url  string
	args map[string]any
}

func NewChromeperfClient(args map[string]any) *ChromeperfClient {
	return &ChromeperfClient{
		Url:  ChromeperfUrl,
		args: args,
	}
}

// ChromeperfConfigurationsResponse is the response object to /api/config
type ChromeperfConfigurationsResponse struct {
	Configurations []string `json:"configurations"`
}

// ChromeperfDescribeResponse is the response object to /api/describe
type ChromeperfDescribeResponse struct {
	// String array of bot names (ie/ mac-m3-pro-perf)
	Bots []string `json:"bots"`

	CaseTags map[string]any `json:"caseTags"`

	// Cases are also known as stories.
	Cases []string `json:"cases"`

	Measurements []string `json:"measurements"`
}

// ListBenchmarks returns all available benchmarks to execute.
func (cp *ChromeperfClient) ListBenchmarks(ctx context.Context, c *http.Client) (string, error) {
	reqUrl := fmt.Sprintf("%s%s", cp.Url, TestSuitesAPI)
	resp, err := httputils.PostWithContext(ctx, c, reqUrl, scomm.ContentType, nil)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to list benchmark options")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to read body of response")
	}

	// Ret format string array
	return string(respBody), nil
}

// buildDescribeAPIUrl returns a fully parameterized target URL for /api/describe
func (cp *ChromeperfClient) buildDescribeAPIUrl(benchmark string) string {
	params := url.Values{}
	if benchmark != "" {
		params.Set("test_suite", benchmark)
	}
	// This is a leagcy thing.
	params.Set("master", "ChromiumPerf")

	reqUrl := fmt.Sprintf("%s%s?%s", cp.Url, DescribeAPI, params.Encode())
	return reqUrl
}

// callDescribeApi triggers the POST request and return the content body.
func (cp *ChromeperfClient) callDescribeApi(ctx context.Context, c *http.Client, reqUrl string) (*ChromeperfDescribeResponse, error) {
	resp, err := httputils.PostWithContext(ctx, c, reqUrl, scomm.ContentType, nil)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to list bot configurations")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to read body of response")
	}

	res := &ChromeperfDescribeResponse{}
	err = json.Unmarshal([]byte(respBody), &res)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse chromeperf response body.")
	}

	return res, nil
}

func (cp *ChromeperfClient) ListBotConfigurations(ctx context.Context, c *http.Client) (*ChromeperfConfigurationsResponse, error) {
	if cp.args != nil && cp.args["benchmark"] != nil && cp.args["benchmark"].(string) != "" {
		// If a benchmark has already been selected, the list of bots comes from a different API.
		// TODO(jeffyoon@) need to verify that the list of bots provided by /api/describe
		// is accurate.
		res, err := cp.callDescribeApi(ctx, c, cp.buildDescribeAPIUrl(cp.args["benchmark"].(string)))
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to call describe api")
		}

		// Bots from the list will contain what bots the benchmark can be run on.
		return &ChromeperfConfigurationsResponse{
			Configurations: res.Bots,
		}, nil
	}

	// Otherwise, use /api/config to get the full list.
	reqUrl := fmt.Sprintf("%s%s", cp.Url, ConfigAPI)
	resp, err := httputils.PostWithContext(ctx, c, reqUrl, scomm.ContentType, nil)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to list bot configurations")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to read body of response")
	}

	res := &ChromeperfConfigurationsResponse{}
	err = json.Unmarshal([]byte(respBody), &res)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse chromeperf response body.")
	}

	return res, nil
}

func (cp *ChromeperfClient) ListStories(ctx context.Context, c *http.Client) ([]string, error) {
	res, err := cp.callDescribeApi(ctx, c, cp.buildDescribeAPIUrl(cp.args["benchmark"].(string)))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to invoke describe api")
	}

	return res.Cases, nil
}
