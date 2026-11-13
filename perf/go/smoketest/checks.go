// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"go.skia.org/infra/go/skerr"
)

// Check names, as accepted by --checks and recorded in verdicts.
const (
	checkVersion         = "version"
	checkHealth          = "health"
	checkTemplates       = "templates"
	checkParamset        = "paramset"
	checkHistoricalQuery = "historical_query"
	checkRegCount        = "reg_count"
	checkSheriffList     = "sheriff_list"
)

// Check result statuses, as recorded in verdicts.
const (
	statusPass    = "pass"
	statusFail    = "fail"
	statusSkipped = "skipped"
)

// maxErrorBodyBytes keeps the server's error message in a failure without
// copying page content into verdicts.
const maxErrorBodyBytes = 120

// target is one perfserver frontend under test.
type target struct {
	name    string
	baseURL string
	// frameQuery overrides --frame_query for this target when non-empty.
	frameQuery string
	// hostHeader and forwardedHTTPS stand in for the headers the load balancer
	// adds to browser traffic. Without them, perfserver's AllowedHosts and SSL
	// redirect checks reject port-forwarded requests.
	hostHeader     string
	forwardedHTTPS bool
	client         *http.Client
	// appVersion is the window.perf.app_version last seen by the version check.
	appVersion string
}

// newHTTPClient returns a client that never follows redirects. A redirect
// means perfserver's handlers did not answer (login page, Host or HTTPS
// enforcement), so the check must fail and report the Location instead.
func newHTTPClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// do sends one request and returns the body of a 200 response. The stderr
// line it prints for every request is also the audit log of live traffic.
func (t *target) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, t.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if t.hostHeader != "" {
		req.Host = t.hostHeader
	}
	if t.forwardedHTTPS {
		req.Header.Set("X-Forwarded-Proto", "https")
	}
	start := time.Now()
	resp, err := t.client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %s -> error (%dms)\n", method, path, time.Since(start).Milliseconds())
		return nil, skerr.Wrap(err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	fmt.Fprintf(os.Stderr, "%s %s -> %d (%dms)\n", method, path, resp.StatusCode, time.Since(start).Milliseconds())
	if err != nil {
		return nil, skerr.Wrapf(err, "reading %s %s", method, path)
	}
	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("%s %s returned %d", method, path, resp.StatusCode)
		if loc := resp.Header.Get("Location"); loc != "" {
			msg += ", Location " + loc
		}
		if len(respBody) > maxErrorBodyBytes {
			respBody = respBody[:maxErrorBodyBytes]
		}
		return nil, skerr.Fmt("%s: %q", msg, respBody)
	}
	return respBody, nil
}

func (t *target) getJSON(ctx context.Context, path string, v any) error {
	body, err := t.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return skerr.Wrapf(json.Unmarshal(body, v), "decoding GET %s", path)
}

func (t *target) windowPerf(ctx context.Context, page string) (map[string]any, error) {
	body, err := t.do(ctx, http.MethodGet, page, nil)
	if err != nil {
		return nil, err
	}
	perf, err := extractWindowPerf(body)
	if err != nil {
		return nil, skerr.Wrapf(err, "page %s", page)
	}
	return perf, nil
}

var windowPerfAssignment = regexp.MustCompile(`window\.perf\s*=\s*`)

// extractWindowPerf decodes the object that page templates assign to
// window.perf. Templates differ in spacing, so only the assignment is matched
// and json.Decoder finds where the value ends.
func extractWindowPerf(page []byte) (map[string]any, error) {
	loc := windowPerfAssignment.FindIndex(page)
	if loc == nil {
		return nil, skerr.Fmt("window.perf assignment not found")
	}
	var perf map[string]any
	if err := json.NewDecoder(bytes.NewReader(page[loc[1]:])).Decode(&perf); err != nil {
		return nil, skerr.Wrapf(err, "decoding window.perf")
	}
	if perf == nil {
		return nil, skerr.Fmt("window.perf is not a JSON object")
	}
	return perf, nil
}

// Check is one smoke check. Run must return once ctx is done.
type Check struct {
	Name    string
	Timeout time.Duration
	Run     func(ctx context.Context, t *target) error
}

// versionCheck retries until rolloutTimeout because a new build reaches
// autopush some minutes after its hash lands in stages.json.
func versionCheck(expected string, rolloutTimeout, pollInterval time.Duration) Check {
	return Check{Name: checkVersion, Run: func(ctx context.Context, t *target) error {
		giveUp := time.Now().Add(rolloutTimeout)
		for {
			err := t.matchAppVersion(ctx, expected)
			if err == nil || time.Now().Add(pollInterval).After(giveUp) {
				return err
			}
			select {
			case <-ctx.Done():
				return err
			case <-time.After(pollInterval):
			}
		}
	}}
}

func (t *target) matchAppVersion(ctx context.Context, expected string) error {
	perf, err := t.windowPerf(ctx, "/m/")
	if err != nil {
		return err
	}
	t.appVersion, _ = perf["app_version"].(string)
	if t.appVersion != expected {
		return skerr.Fmt("app_version is %q, want %q", t.appVersion, expected)
	}
	return nil
}

func healthCheck(path string) Check {
	return Check{Name: checkHealth, Run: func(ctx context.Context, t *target) error {
		_, err := t.do(ctx, http.MethodGet, path, nil)
		return err
	}}
}

func templatesCheck(pages []string) Check {
	return Check{Name: checkTemplates, Run: func(ctx context.Context, t *target) error {
		for _, page := range pages {
			if _, err := t.windowPerf(ctx, page); err != nil {
				return err
			}
		}
		return nil
	}}
}

func paramsetCheck(required []string) Check {
	return Check{Name: checkParamset, Run: func(ctx context.Context, t *target) error {
		var resp struct {
			DataFrame struct {
				ParamSet map[string][]string `json:"paramset"`
			} `json:"dataframe"`
		}
		if err := t.getJSON(ctx, "/_/initpage/", &resp); err != nil {
			return err
		}
		var missing []string
		for _, key := range required {
			if len(resp.DataFrame.ParamSet[key]) == 0 {
				missing = append(missing, key)
			}
		}
		if len(missing) > 0 {
			return skerr.Fmt("paramset has no values for %s", strings.Join(missing, ", "))
		}
		return nil
	}}
}

// frameRequest and progressResponse mirror the JSON of frame.FrameRequest and
// progress.SerializedProgress, because importing those packages would pull
// the perfserver backend into this binary.
type frameRequest struct {
	Begin   int64    `json:"begin"`
	End     int64    `json:"end"`
	Queries []string `json:"queries"`
	TZ      string   `json:"tz"`
}

type progressResponse struct {
	Status   string `json:"status"`
	Messages []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"messages"`
	Results struct {
		DataFrame struct {
			TraceSet map[string]json.RawMessage `json:"traceset"`
		} `json:"dataframe"`
	} `json:"results"`
	URL string `json:"url"`
}

func historicalQueryCheck(defaultQuery string, begin, end time.Time, pollInterval time.Duration) Check {
	return Check{Name: checkHistoricalQuery, Run: func(ctx context.Context, t *target) error {
		query := t.frameQuery
		if query == "" {
			query = defaultQuery
		}
		reqBody, err := json.Marshal(frameRequest{Begin: begin.Unix(), End: end.Unix(), Queries: []string{query}, TZ: "UTC"})
		if err != nil {
			return skerr.Wrap(err)
		}
		respBody, err := t.do(ctx, http.MethodPost, "/_/frame/start", reqBody)
		if err != nil {
			return err
		}
		var p progressResponse
		if err := json.Unmarshal(respBody, &p); err != nil {
			return skerr.Wrapf(err, "decoding POST /_/frame/start")
		}
		// Same contract as startRequest in perf/modules/progress/progress.ts:
		// every Running response names the URL to poll next.
		for p.Status == "Running" {
			if !strings.HasPrefix(p.URL, "/") {
				return skerr.Fmt("progress response has no polling url: %q", p.URL)
			}
			select {
			case <-ctx.Done():
				return skerr.Fmt("frame request still Running at the check deadline: %s", ctx.Err())
			case <-time.After(pollInterval):
			}
			var next progressResponse
			if err := t.getJSON(ctx, p.URL, &next); err != nil {
				return err
			}
			p = next
		}
		switch p.Status {
		case "Finished":
			if len(p.Results.DataFrame.TraceSet) == 0 {
				return skerr.Fmt("frame request finished with an empty traceset")
			}
			return nil
		case "Error":
			msgs := make([]string, 0, len(p.Messages))
			for _, m := range p.Messages {
				msgs = append(msgs, m.Key+": "+m.Value)
			}
			return skerr.Fmt("frame request failed: %s", strings.Join(msgs, "; "))
		default:
			return skerr.Fmt("unexpected progress status %q", p.Status)
		}
	}}
}

func regCountCheck() Check {
	return Check{Name: checkRegCount, Run: func(ctx context.Context, t *target) error {
		// The handler encodes an untagged struct{ Count int }.
		var resp struct{ Count *int }
		if err := t.getJSON(ctx, "/_/reg/count", &resp); err != nil {
			return err
		}
		if resp.Count == nil {
			return skerr.Fmt("GET /_/reg/count has no Count field")
		}
		return nil
	}}
}

// sheriffListCheck is opt-in because every /_/anomalies/* handler answers
// anonymous requests with 401.
func sheriffListCheck() Check {
	return Check{Name: checkSheriffList, Run: func(ctx context.Context, t *target) error {
		body, err := t.do(ctx, http.MethodGet, "/_/anomalies/sheriff_list", nil)
		if err != nil {
			return err
		}
		if !json.Valid(body) {
			return skerr.Fmt("GET /_/anomalies/sheriff_list returned invalid JSON")
		}
		return nil
	}}
}

// CheckResult is the outcome of one check, as recorded in verdicts.
type CheckResult struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// runChecks runs the checks in order, each under its own deadline. After a
// version failure the instance is not serving the expected build, so the
// remaining checks are skipped rather than reported against the wrong build.
func runChecks(ctx context.Context, t *target, checks []Check) []CheckResult {
	results := make([]CheckResult, 0, len(checks))
	versionFailed := false
	for _, c := range checks {
		if versionFailed {
			results = append(results, CheckResult{Name: c.Name, Status: statusSkipped})
			continue
		}
		checkCtx, cancel := context.WithTimeout(ctx, c.Timeout)
		start := time.Now()
		err := c.Run(checkCtx, t)
		cancel()
		result := CheckResult{Name: c.Name, Status: statusPass, LatencyMS: time.Since(start).Milliseconds()}
		if err != nil {
			result.Status = statusFail
			result.Error = errorMessage(err)
			versionFailed = c.Name == checkVersion
		}
		results = append(results, result)
	}
	return results
}

// errorMessage drops the skerr call stack, which would bury the reason in
// verdicts and tables.
func errorMessage(err error) string {
	var withContext *skerr.ErrorWithContext
	if !errors.As(err, &withContext) {
		return err.Error()
	}
	short := *withContext
	short.CallStack = nil
	return short.Error()
}
