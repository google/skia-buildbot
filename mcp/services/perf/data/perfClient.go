package data

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/dataframe"
)

// PerfClient is an HTTP client for the Perf frontend API.
type PerfClient struct {
	httpClient *http.Client
	baseURL    string
}

// GetParamSetResponse is a partial struct for decoding the response from /_/initpage/.
type GetParamSetResponse struct {
	DataFrame *dataframe.DataFrame `json:"dataframe"`
}

// NewPerfClient creates a new PerfClient.
func NewPerfClient(httpClient *http.Client, baseURL string) *PerfClient {
	return &PerfClient{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

// GetTraceDataRequest represents the parameters for the /mcp/data endpoint.
type GetTraceDataRequest struct {
	Query string
	Begin time.Time
	End   time.Time
}

// GetTraceData fetches trace data from the Perf frontend API.
func (c *PerfClient) GetTraceData(ctx context.Context, req GetTraceDataRequest) (*dataframe.DataFrame, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to parse base URL %q", c.baseURL)
	}
	u.Path = "/mcp/data"

	q := u.Query()
	q.Set("query", req.Query)
	q.Set("begin", strconv.FormatInt(req.Begin.Unix(), 10))
	q.Set("end", strconv.FormatInt(req.End.Unix(), 10))
	u.RawQuery = q.Encode()

	resp, err := httputils.GetWithContext(ctx, c.httpClient, u.String())
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to make GET request to %q", u.String())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, skerr.Wrapf(readErr, "failed to read error response body from %s (status: %d)", u.String(), resp.StatusCode)
		}
		return nil, skerr.Fmt("unexpected status code: %d %s, body: %s", resp.StatusCode, resp.Status, string(bodyBytes))
	}

	var df dataframe.DataFrame
	if err := json.NewDecoder(resp.Body).Decode(&df); err != nil {
		return nil, skerr.Wrapf(err, "failed to decode response body from %s", u.String())
	}

	return &df, nil
}

// GetParamSet fetches the paramset from the Perf frontend API.
func (c *PerfClient) GetParamSet(ctx context.Context) (paramtools.ReadOnlyParamSet, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to parse base URL %q", c.baseURL)
	}
	u.Path = "/_/initpage/"

	resp, err := httputils.GetWithContext(ctx, c.httpClient, u.String())
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to make GET request to %q", u.String())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, skerr.Wrapf(readErr, "failed to read error response body from %s (status: %d)", u.String(), resp.StatusCode)
		}
		return nil, skerr.Fmt("unexpected status code: %d %s, body: %s", resp.StatusCode, resp.Status, string(bodyBytes))
	}

	var initPageResp GetParamSetResponse
	if err := json.NewDecoder(resp.Body).Decode(&initPageResp); err != nil {
		return nil, skerr.Wrapf(err, "failed to decode response body from %s", u.String())
	}

	if initPageResp.DataFrame == nil {
		return paramtools.ReadOnlyParamSet{}, nil
	}

	return initPageResp.DataFrame.ParamSet, nil
}
