// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/insights"
	"go.skia.org/infra/perf/go/regression"
	subpb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

const (
	sheriff = "Demo Sheriff"
	goodID  = "abcdef00-0000-4000-8000-000000000001"
)

type fakeRegressions struct {
	err       error
	byIDCalls int
}

func (f *fakeRegressions) GetRegressionsBySubName(context.Context, regression.GetAnomalyListRequest, int) ([]*regression.Regression, error) {
	if f.err != nil {
		return nil, f.err
	}
	r := regression.NewRegression()
	r.Id = goodID
	r.SubscriptionName = sheriff
	r.CreationTime = time.Now()
	r.PrevCommitNumber = 1
	r.CommitNumber = 2
	r.MedianBefore = 10
	r.MedianAfter = 12
	r.High = &clustering2.ClusterSummary{}
	r.HighStatus = regression.TriageStatus{Status: regression.Untriaged}
	return []*regression.Regression{r}, nil
}

func (f *fakeRegressions) GetByIDs(_ context.Context, ids []string) ([]*regression.Regression, error) {
	f.byIDCalls++
	ret := []*regression.Regression{}
	for _, id := range ids {
		if id == goodID {
			r := regression.NewRegression()
			r.Id = goodID
			r.PrevCommitNumber = types.CommitNumber(1)
			r.CommitNumber = types.CommitNumber(2)
			r.Frame = &frame.FrameResponse{DataFrame: &dataframe.DataFrame{TraceSet: types.TraceSet{traceKey: nil}}}
			ret = append(ret, r)
		}
	}
	return ret, nil
}

const traceKey = ",benchmark=8888,bot=x86,master=demo,test=encode,units=ms,"

type fakeTraces struct{}

func (fakeTraces) ReadTrace(_ context.Context, key string, begin, end types.CommitNumber) ([]types.CommitNumber, []float32, error) {
	if key != traceKey {
		return nil, nil, errors.New("unexpected key")
	}
	return []types.CommitNumber{1, 2}, []float32{10, 12}, nil
}

func (f *fakeRegressions) GetByRevision(context.Context, string) ([]*regression.Regression, error) {
	return nil, nil
}

func (f *fakeRegressions) GetBugIdsForRegressions(_ context.Context, regs []*regression.Regression) ([]*regression.Regression, error) {
	return regs, nil
}

type fakeSubs struct{}

func (fakeSubs) GetAllActiveSubscriptions(context.Context) ([]*subpb.Subscription, error) {
	return []*subpb.Subscription{{Name: sheriff}}, nil
}

func newServer(t *testing.T, fr *fakeRegressions) *httptest.Server {
	return newServerWithStores(t, insights.Stores{Regressions: fr, Subscriptions: fakeSubs{}})
}

func newServerWithStores(t *testing.T, stores insights.Stores) *httptest.Server {
	svc, err := insights.New(stores, insights.Options{InstanceName: "demo"})
	require.NoError(t, err)
	r := chi.NewRouter()
	New(svc).RegisterHandlers(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts
}

func TestGetAnomalyTrace_ReturnsPointsAndValidatesInput(t *testing.T) {
	ts := newServerWithStores(t, insights.Stores{Regressions: &fakeRegressions{}, Subscriptions: fakeSubs{}, Traces: fakeTraces{}})

	resp, body := get(t, ts, "/api/v1/anomalies/"+goodID+"/trace?commits_before=10&commits_after=5")
	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	assert.Equal(t, traceKey, body["trace_key"])
	assert.Len(t, body["points"], 2)
	assert.Equal(t, float64(0), body["begin"], "commit range start 2 minus 10, clamped to 0")
	assert.Equal(t, float64(7), body["end"])
	assert.Equal(t, goodID, body["anomaly"].(map[string]interface{})["id"])

	resp, body = get(t, ts, "/api/v1/anomalies/"+goodID+"/trace?commits_before=lots")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "INVALID_ARGUMENT", errorCode(body))

	resp, body = get(t, ts, "/api/v1/anomalies/not-a-uuid/trace")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "INVALID_ARGUMENT", errorCode(body))
}

func get(t *testing.T, ts *httptest.Server, path string) (*http.Response, map[string]interface{}) {
	resp, err := ts.Client().Get(ts.URL + path)
	require.NoError(t, err)
	defer resp.Body.Close()
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return resp, body
}

func errorCode(body map[string]interface{}) string {
	e, _ := body["error"].(map[string]interface{})
	code, _ := e["code"].(string)
	return code
}

func TestListSheriffs_ReturnsJSONWithInstanceHeaders(t *testing.T) {
	ts := newServer(t, &fakeRegressions{})
	resp, body := get(t, ts, "/api/v1/sheriffs")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "demo", resp.Header.Get("X-Perf-Instance"))
	assert.Equal(t, "internal", resp.Header.Get("X-Perf-Visibility"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	sheriffs := body["sheriffs"].([]interface{})
	require.Len(t, sheriffs, 1)
}

func TestListAnomalies_ValidAndInvalidRequests(t *testing.T) {
	ts := newServer(t, &fakeRegressions{})

	resp, body := get(t, ts, "/api/v1/anomalies?sheriff=Demo%20Sheriff")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, body["anomalies"], 1)

	resp, body = get(t, ts, "/api/v1/anomalies?sheriff=nope")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "INVALID_ARGUMENT", errorCode(body))
	assert.Contains(t, body["error"].(map[string]interface{})["hint"], sheriff)

	resp, body = get(t, ts, "/api/v1/anomalies?sheriff=Demo%20Sheriff&page_size=lots")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "INVALID_ARGUMENT", errorCode(body))
}

func TestStoreErrors_ReturnGenericInternalErrorWithoutSQL(t *testing.T) {
	ts := newServer(t, &fakeRegressions{err: errors.New("failed. Query: SELECT * FROM Regressions2 WHERE secret")})
	resp, body := get(t, ts, "/api/v1/anomalies?sheriff=Demo%20Sheriff")
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "INTERNAL", errorCode(body))
	b, err := json.Marshal(body)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "SELECT")
}

func TestGetAnomaly_ValidatesIDBeforeQuerying(t *testing.T) {
	fr := &fakeRegressions{}
	ts := newServer(t, fr)

	resp, body := get(t, ts, "/api/v1/anomalies/'%20OR%201=1")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "INVALID_ARGUMENT", errorCode(body))
	assert.Equal(t, 0, fr.byIDCalls)

	resp, body = get(t, ts, "/api/v1/anomalies/"+goodID)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, goodID, body["id"])

	resp, body = get(t, ts, "/api/v1/anomalies/abcdef00-0000-4000-8000-000000000999")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "NOT_FOUND", errorCode(body))
}

func post(t *testing.T, ts *httptest.Server, path, contentType, body string) (*http.Response, map[string]interface{}) {
	resp, err := ts.Client().Post(ts.URL+path, contentType, strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	var ret map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&ret))
	return resp, ret
}

func TestBatchGet_RequiresJSONAndRejectsUnknownFieldsAndLargeBodies(t *testing.T) {
	ts := newServer(t, &fakeRegressions{})

	resp, body := post(t, ts, "/api/v1/anomalies:batchGet", "application/json", `{"ids":["`+goodID+`","abcdef00-0000-4000-8000-000000000002"]}`)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, body["anomalies"], 1)
	assert.Equal(t, []interface{}{"abcdef00-0000-4000-8000-000000000002"}, body["not_found"])

	resp, _ = post(t, ts, "/api/v1/anomalies:batchGet", "text/plain", `{"ids":["`+goodID+`"]}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, _ = post(t, ts, "/api/v1/anomalies:batchGet", "application/json", `{"ids":["`+goodID+`"],"sql":"DROP"}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, _ = post(t, ts, "/api/v1/anomalies:batchGet", "application/json", `{"ids":["`+strings.Repeat("a", maxBodyBytes)+`"]}`)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestOnlyAllowedMethods(t *testing.T) {
	ts := newServer(t, &fakeRegressions{})
	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/sheriffs", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestOpenAPI_IsValidJSON(t *testing.T) {
	ts := newServer(t, &fakeRegressions{})
	resp, body := get(t, ts, "/openapi.json")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "3.0.3", body["openapi"])
	assert.NotContains(t, body, "servers", "served at the root, the default server / is right")
	paths := body["paths"].(map[string]interface{})
	for _, p := range []string{"/api/v1/sheriffs", "/api/v1/anomalies", "/api/v1/anomalies/{id}", "/api/v1/anomalies/{id}/trace", "/api/v1/anomalies:batchGet", "/api/v1/anomalyGroups/{id}", "/api/v1/revisions/{rev}/anomalies", "/api/v1/insights:summarize", "/api/v1/insights:suggestTriage"} {
		assert.Contains(t, paths, p)
	}
}

func TestNewWithBasePath_OpenAPINamesTheBasePathAsServer(t *testing.T) {
	svc, err := insights.New(insights.Stores{Regressions: &fakeRegressions{}, Subscriptions: fakeSubs{}}, insights.Options{InstanceName: "demo"})
	require.NoError(t, err)
	api, err := NewWithBasePath(svc, "/insights/")
	require.NoError(t, err)
	r := chi.NewRouter()
	r.Route("/insights", api.RegisterHandlers)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	resp, body := get(t, ts, "/insights/openapi.json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []interface{}{map[string]interface{}{"url": "/insights"}}, body["servers"])
	assert.Equal(t, "3.0.3", body["openapi"])
	assert.Contains(t, body["paths"].(map[string]interface{}), "/api/v1/sheriffs")
	resp, _ = get(t, ts, "/insights/api/v1/sheriffs")
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	api, err = NewWithBasePath(svc, "")
	require.NoError(t, err)
	assert.Equal(t, OpenAPISpec(), api.spec, "no base path: the document is served as is")
	for _, bad := range []string{"insights", "//evil.example.com", "/a b", "/x?y", "/../x", `/"x`} {
		_, err := NewWithBasePath(svc, bad)
		assert.Error(t, err, bad)
	}
}

func TestSuggestTriage_ReturnsAPlanAndValidatesInput(t *testing.T) {
	ts := newServer(t, &fakeRegressions{})

	resp, body := get(t, ts, "/api/v1/insights:suggestTriage?sheriff=Demo%20Sheriff&days=2")
	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	assert.Equal(t, float64(1), body["untriaged"])
	suggestions := body["suggestions"].([]interface{})
	require.Len(t, suggestions, 1)
	assert.Equal(t, "review", suggestions[0].(map[string]interface{})["action"], "a step without stored data to judge it")
	assert.Contains(t, body["note"], "never changes triage")

	resp, body = get(t, ts, "/api/v1/insights:suggestTriage?sheriff=nope")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "INVALID_ARGUMENT", errorCode(body))

	resp, body = get(t, ts, "/api/v1/insights:suggestTriage?sheriff=Demo%20Sheriff&days=many")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "INVALID_ARGUMENT", errorCode(body))
}
