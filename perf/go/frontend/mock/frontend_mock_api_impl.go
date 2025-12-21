package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/perf/go/frontend"
)

/* This file contains essentially a mock for the whole backend.
 * It is used for a demo server. */

// MockFrontend maintains the state of the demo session.
type MockFrontend struct {
	f              *frontend.Frontend
	currentQueries []string
	queryMutex     sync.Mutex
}

// --- Query Builder Logic (Stateless) ---

// Hierarchy defines the navigation order.
var paramHierarchy = []string{"arch", "os"}

func (m *MockFrontend) nextParamListHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"q"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}

	selectedParams, _ := url.ParseQuery(req.Query)

	// Determine next param in hierarchy
	nextParam := ""
	for _, key := range paramHierarchy {
		if _, ok := selectedParams[key]; !ok {
			nextParam = key
			break
		}
	}

	matchCount := 0
	seenValues := make(map[string]bool)

	// Stateless filter against all mock data
	for traceKey := range mockTraceData {
		if isTraceMatch(traceKey, selectedParams) {
			matchCount++
			if nextParam != "" {
				if val := getParamValue(traceKey, nextParam); val != "" {
					seenValues[val] = true
				}
			}
		}
	}

	respParamSet := make(map[string][]string)
	if nextParam != "" && len(seenValues) > 0 {
		var vals []string
		for v := range seenValues {
			vals = append(vals, v)
		}
		respParamSet[nextParam] = vals
	}

	sendJSON(w, map[string]interface{}{
		"paramset": respParamSet,
		"count":    matchCount,
	})
}

func (m *MockFrontend) countHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"q"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	selectedParams, _ := url.ParseQuery(req.Query)
	matchCount := 0

	for traceKey := range mockTraceData {
		if isTraceMatch(traceKey, selectedParams) {
			matchCount++
		}
	}

	sendJSON(w, map[string]interface{}{
		"count":    matchCount,
		"paramset": mockParamSet,
	})
}

// --- Graph & Status Logic ---

func (m *MockFrontend) frameStartHandler(w http.ResponseWriter, r *http.Request) {
	type reqFrame struct {
		Queries []string `json:"queries"`
	}
	var req reqFrame
	_ = json.NewDecoder(r.Body).Decode(&req)

	m.queryMutex.Lock()
	m.currentQueries = req.Queries
	m.queryMutex.Unlock()

	sendJSON(w, map[string]string{
		"status": "Running",
		"msg":    "Running...",
		"url":    "/_/status/demo-req",
	})
}

func (m *MockFrontend) statusHandler(w http.ResponseWriter, r *http.Request) {
	m.queryMutex.Lock()
	queries := m.currentQueries
	m.queryMutex.Unlock()

	filteredTraces := make(map[string][]float64)
	filteredAnomalies := make(map[string]interface{})

	if len(queries) > 0 {
		for traceKey, data := range mockTraceData {
			// MatchesAny checks if the trace matches *at least one* of the query strings
			if matchesAny(traceKey, queries) {
				filteredTraces[traceKey] = data
				if anomalies, ok := mockAnomalyMap[traceKey]; ok {
					filteredAnomalies[traceKey] = anomalies
				}
			}
		}
	}

	sendJSON(w, map[string]interface{}{
		"status": "Finished",
		"results": map[string]interface{}{
			"dataframe": map[string]interface{}{
				"traceset":      filteredTraces,
				"header":        mockHeader,
				"paramset":      mockParamSet,
				"tracemetadata": mockTraceMetadata,
			},
			"anomalymap":   filteredAnomalies,
			"display_mode": "display_plot",
		},
	})
}

// --- General API Handlers ---

func (m *MockFrontend) loginStatusHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]interface{}{"email": "user@google.com", "roles": []string{"admin", "bisecter"}})
}

func (m *MockFrontend) defaultsHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]interface{}{
		"include_params":           []string{"arch", "os"},
		"default_param_selections": map[string][]string{},
		"default_url_values":       map[string]string{"enable_chart_tooltip": "true", "useTestPicker": "true", "show_google_plot": "true"},
		"default_range":            604800,
	})
}

func (m *MockFrontend) initPageHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]interface{}{"dataframe": map[string]interface{}{"paramset": mockParamSet}})
}

func (m *MockFrontend) cidHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]interface{}{
		"cid": map[string]interface{}{
			"author": "dev@example.com",
			"hash":   "7294567890abcdef",
			"ts":     time.Now().Unix(),
		},
	})
}

func (m *MockFrontend) detailsHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]interface{}{"cid": "7294567890abcdef", "log": "Mock commit details."})
}

func (m *MockFrontend) cidRangeHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, []map[string]interface{}{
		{"hash": "abc1234", "ts": 1687855198, "author": "a@google.com"},
		{"hash": "def5678", "ts": 1687857789, "author": "b@google.com"},
	})
}

func (m *MockFrontend) shiftHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]interface{}{"begin": 1687855198, "end": 1687878658})
}

func (m *MockFrontend) linksHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]string{"bug_search": "http://buganizer/search"})
}

// --- Alerts / Regressions / Triage Handlers ---

func (m *MockFrontend) alertListHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, []map[string]interface{}{{"id": "1", "display_name": "Demo Alert", "query": "source_type=skp", "alert": "email@example.com"}})
}
func (m *MockFrontend) alertNewHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]interface{}{"display_name": "New Alert", "step": "0.5"})
}
func (m *MockFrontend) alertUpdateHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]int{"id": 123})
}
func (m *MockFrontend) alertDeleteHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]string{"status": "ok"})
}
func (m *MockFrontend) alertBugTryHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]string{"url": "http://buganizer/new?template=123"})
}
func (m *MockFrontend) alertNotifyTryHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]string{"status": "sent"})
}
func (m *MockFrontend) subscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, []map[string]interface{}{{"name": "Test Sub", "contact_email": "owner@example.com"}})
}
func (m *MockFrontend) dryrunStartHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]string{"id": "dryrun-123"})
}
func (m *MockFrontend) sheriffListHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]interface{}{"list": []string{"Sheriff 1"}})
}
func (m *MockFrontend) anomalyListHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]interface{}{"anomalies": mockAnomalyMap})
}
func (m *MockFrontend) anomaliesGroupReportHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]interface{}{
		"anomaly_list": []map[string]interface{}{
			{"id": "anomaly-id-1", "test_path": ",arch=arm,os=Android,", "start_revision": 67129, "end_revision": 67130, "state": "triaged"},
		},
	})
}
func (m *MockFrontend) regressionsAlertsHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]int{"count": 5})
}
func (m *MockFrontend) regressionsListHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, []map[string]interface{}{{"id": "r1", "commit_number": 67130, "alert_id": 1, "is_improvement": false, "triaged": map[string]string{"status": "untriaged"}}})
}
func (m *MockFrontend) clusterStartHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]string{"id": "cluster-job-1"})
}
func (m *MockFrontend) triageFileBugHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]int{"bug_id": rand.Intn(100000)})
}
func (m *MockFrontend) triageEditAnomaliesHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]string{"error": ""})
}
func (m *MockFrontend) triageAssociateAlertsHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]int{"bug_id": 54321})
}
func (m *MockFrontend) triageListIssuesHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]interface{}{"issues": []map[string]interface{}{{"issue_id": 54321, "issue_state": map[string]string{"title": "Mock Issue Title"}}}})
}
func (m *MockFrontend) userIssuesHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]interface{}{"UserIssues": []map[string]interface{}{{"user_id": "demo@example.com", "trace_key": ",arch=arm,os=Android,", "issue_id": 999}}})
}
func (m *MockFrontend) userIssueSaveHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}
func (m *MockFrontend) userIssueDeleteHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}
func (m *MockFrontend) favoritesListHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]interface{}{"sections": []map[string]interface{}{{"name": "My Favorites", "links": []map[string]string{{"id": "fav1", "text": "Android Arm", "href": "/e/?queries=arch=arm&os=Android"}}}}})
}
func (m *MockFrontend) favoritesNewHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}
func (m *MockFrontend) shortcutKeysHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]string{"id": "keys-id-123"})
}
func (m *MockFrontend) shortcutHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]interface{}{"graphs": []map[string]interface{}{{"queries": []string{"arch=arm", "os=Android"}}}})
}
func (m *MockFrontend) shortcutUpdateHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]string{"id": "shortcut-id-999"})
}
func (m *MockFrontend) bisectCreateHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]string{"job_id": "pinpoint-job-123", "job_url": "https://pinpoint-mock.example.com/job/123"})
}

// --- Helpers ---

// isTraceMatch checks if a trace key contains ALL the selected parameters.
func isTraceMatch(traceKey string, params url.Values) bool {
	for k, vals := range params {
		if len(vals) == 0 {
			continue
		}
		// Check for the exact token ",key=value,"
		token := fmt.Sprintf(",%s=%s,", k, vals[0])
		if !strings.Contains(traceKey, token) {
			return false
		}
	}
	return true
}

// matchesAny checks if a trace key matches ANY of the query strings (for the plot/status handler)
func matchesAny(traceKey string, queries []string) bool {
	for _, q := range queries {
		parsed, _ := url.ParseQuery(q)
		if isTraceMatch(traceKey, parsed) {
			return true
		}
	}
	return false
}

func getParamValue(traceKey, key string) string {
	prefix := "," + key + "="
	start := strings.Index(traceKey, prefix)
	if start == -1 {
		return ""
	}
	start += len(prefix)
	rest := traceKey[start:]
	end := strings.Index(rest, ",")
	if end == -1 {
		return ""
	}
	return rest[:end]
}

func sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

// --- Test Data ---

var mockParamSet = map[string][]string{
	"arch": {"arm", "arm64", "x86_64"},
	"os":   {"Android", "Debian10", "Ubuntu", "Win2019"},
}

// mockTraceData populated with all 12 combinations (3 Arch * 4 OS)
var mockTraceData = func() map[string][]float64 {
	data := make(map[string][]float64)
	for _, arch := range mockParamSet["arch"] {
		for _, os := range mockParamSet["os"] {
			key := fmt.Sprintf(",arch=%s,os=%s,", arch, os)
			// Generate deterministic random-ish data based on key length
			base := float64(len(key))
			data[key] = []float64{base, base + 1, base - 1, base + 2, base, base + 5, base + 4}
		}
	}
	return data
}()

var mockHeader = []map[string]interface{}{
	{"offset": 67125, "timestamp": 1687855198},
	{"offset": 67126, "timestamp": 1687857789},
	{"offset": 67127, "timestamp": 1687868015},
	{"offset": 67128, "timestamp": 1687868368},
	{"offset": 67129, "timestamp": 1687870256},
	{"offset": 67130, "timestamp": 1687872763},
	{"offset": 67131, "timestamp": 1687877748},
}

var mockAnomalyMap = map[string]interface{}{
	",arch=arm,os=Android,": map[string]interface{}{
		"67130": map[string]interface{}{
			"id": "123", "test_path": ",arch=arm,os=Android,", "state": "untriaged",
			"start_revision": 67129, "end_revision": 67130,
			"median_before_anomaly": 60.8, "median_after_anomaly": 75.2,
		},
	},
	",arch=arm,os=Ubuntu,": map[string]interface{}{
		"67130": map[string]interface{}{
			"id": "456", "test_path": ",arch=arm,os=Ubuntu,", "state": "untriaged",
			"start_revision": 67129, "end_revision": 67130,
			"median_before_anomaly": 13.4, "median_after_anomaly": 18.2,
		},
	},
}

var mockTraceMetadata = []map[string]interface{}{
	{
		"traceid": ",benchmark=jetstream2,bot=win-11-perf,improvement_direction=up,master=ChromiumPerf,stat=value,subtest_1=JetStream2,test=Air,unit=unitless_biggerIsBetter,",
		"commitLinks": map[string]interface{}{
			"1523744": map[string]interface{}{},
			"1523784": map[string]interface{}{},
			"1523789": map[string]interface{}{},
			"1529919": map[string]interface{}{
				"Build Page": map[string]string{
					"Text": "",
					"Href": "https://ci.chromium.org/ui/p/chrome/builders/ci/win-11-perf/15223",
				},
				"V8": map[string]string{
					"Text": "4f387132 (No Change)",
					"Href": "https://chromium.googlesource.com/v8/v8/+/4f38713295d159272dcba0dc433e4c526a2ddd28",
				},
				"WebRTC": map[string]string{
					"Text": "789735e6 (No Change)",
					"Href": "https://chromium.googlesource.com/external/webrtc/+/789735e60b2b00643a2fb70381bf3ffd2be59631",
				},
			},
			"1530122": map[string]interface{}{
				"Build Page": map[string]string{
					"Text": "",
					"Href": "https://ci.chromium.org/ui/p/chrome/builders/ci/win-11-perf/15230",
				},
				"V8": map[string]string{
					"Text": "4f387132 (No Change)",
					"Href": "https://chromium.googlesource.com/v8/v8/+/4f38713295d159272dcba0dc433e4c526a2ddd28",
				},
				"WebRTC": map[string]string{
					"Text": "789735e6 - 6488ee96",
					"Href": "https://chromium.googlesource.com/external/webrtc/+log/789735e60b2b00643a2fb70381bf3ffd2be59631..6488ee961637e6ffdfe22d3052b5e3c6d3a22752",
				},
			},
			"1530341": map[string]interface{}{
				"Build Page": map[string]string{
					"Text": "",
					"Href": "https://ci.chromium.org/ui/p/chrome/builders/ci/win-11-perf/15236",
				},
				"V8": map[string]string{
					"Text": "4f387132 - 9d504525",
					"Href": "https://chromium.googlesource.com/v8/v8/+log/4f38713295d159272dcba0dc433e4c526a2ddd28..9d504525648dc3b74a23eee39de1c68ea80b83f1",
				},
				"WebRTC": map[string]string{
					"Text": "034281d9 (No Change)",
					"Href": "https://chromium.googlesource.com/external/webrtc/+/034281d9a32e43e732c1e7dc215bf55441e916b9",
				},
			},
			"1546707": map[string]interface{}{
				"Build Page": map[string]string{
					"Text": "",
					"Href": "https://ci.chromium.org/ui/p/chrome/builders/ci/win-11-perf/16068",
				},
				"V8": map[string]string{
					"Text": "20aa26b8 - 8cf4312a",
					"Href": "https://chromium.googlesource.com/v8/v8/+log/20aa26b8b4024967addce26b3c1e48d4fdaf623c..8cf4312a8aa059de37a163a267bfbb9bc3238051",
				},
				"WebRTC": map[string]string{
					"Text": "d3dc3341 (No Change)",
					"Href": "https://chromium.googlesource.com/external/webrtc/+/d3dc33415dba12a7b26bae4b667970468fcb256b",
				},
			},
		},
	},
}
