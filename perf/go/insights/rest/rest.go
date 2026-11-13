// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rest serves the perfinsights service as a REST/JSON API under /api/v1, following the
// contracts of the perf.v1 API plan: C1 (error envelope), C2 (opaque page_token) and C4
// (X-Perf-Instance / X-Perf-Visibility headers). All endpoints are read-only.
package rest

import (
	_ "embed"
	"encoding/json"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/insights"
)

//go:embed openapi.json
var openAPISpec []byte

// OpenAPISpec returns a copy of the OpenAPI 3 document served at /openapi.json.
func OpenAPISpec() []byte {
	return append([]byte(nil), openAPISpec...)
}

// maxBodyBytes bounds the size of request bodies.
const maxBodyBytes = 64 * 1024

// validBasePath matches "" and paths like /insights or /a/b: plain segments only.
var validBasePath = regexp.MustCompile(`^(/[A-Za-z0-9_-]+)*$`)

// IsValidBasePath reports whether path can be the path the API and its pages are mounted under:
// empty, or plain segments like /insights.
func IsValidBasePath(path string) bool {
	return validBasePath.MatchString(path)
}

// API is the REST API. It is safe for concurrent use.
type API struct {
	svc *insights.Service

	// spec is the OpenAPI document served at /openapi.json.
	spec []byte
}

// New returns a new API, for a router that serves it at the root of the server.
func New(svc *insights.Service) *API {
	return &API{svc: svc, spec: openAPISpec}
}

// NewWithBasePath returns a new API for a router mounted under basePath, e.g. /insights. The
// OpenAPI document it serves names basePath as its server, so clients resolve the paths under it.
func NewWithBasePath(svc *insights.Service, basePath string) (*API, error) {
	basePath = strings.TrimRight(basePath, "/")
	if basePath == "" {
		return New(svc), nil
	}
	if !IsValidBasePath(basePath) {
		return nil, skerr.Fmt("invalid base path %q: want a path like /insights", basePath)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(openAPISpec, &doc); err != nil {
		return nil, skerr.Wrapf(err, "parsing openapi.json")
	}
	servers, err := json.Marshal([]map[string]string{{"url": basePath}})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	doc["servers"] = servers
	spec, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &API{svc: svc, spec: spec}, nil
}

// RegisterHandlers registers the API routes on r. Only the listed methods are allowed.
func (a *API) RegisterHandlers(r chi.Router) {
	r.Get("/api/v1/instance", a.handle("GetInstance", a.getInstance))
	r.Get("/api/v1/sheriffs", a.handle("ListSheriffs", a.listSheriffs))
	r.Get("/api/v1/anomalies", a.handle("ListAnomalies", a.listAnomalies))
	r.Get("/api/v1/anomalies/{id}", a.handle("GetAnomaly", a.getAnomaly))
	r.Get("/api/v1/anomalies/{id}/trace", a.handle("GetAnomalyTrace", a.getAnomalyTrace))
	r.Post("/api/v1/anomalies:batchGet", a.handle("BatchGetAnomalies", a.batchGetAnomalies))
	r.Get("/api/v1/anomalyGroups/{id}", a.handle("GetAnomalyGroup", a.getAnomalyGroup))
	r.Get("/api/v1/revisions/{rev}/anomalies", a.handle("FindAnomaliesForRevision", a.findAnomaliesForRevision))
	r.Get("/api/v1/insights:summarize", a.handle("SummarizeAnomalies", a.summarize))
	r.Get("/api/v1/insights:suggestTriage", a.handle("SuggestTriage", a.suggestTriage))
	r.Get("/openapi.json", a.openAPI)
}

type handlerFunc func(w http.ResponseWriter, r *http.Request) (interface{}, error)

// handle wraps a handler with error mapping, common headers and metrics.
func (a *API) handle(op string, h handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		resp, err := h(w, r)
		a.setInstanceHeaders(w)
		if err != nil {
			pe := insights.ToPublicError(err)
			if pe.Code == insights.CodeInternal || pe.Code == insights.CodeDeadlineExceeded {
				// The original error may contain SQL; it is only logged, never returned.
				sklog.Errorf("REST %s failed: %s", op, err)
			}
			WriteJSON(w, pe.Code.HTTPStatus(), insights.ErrorEnvelope{Error: pe})
			insights.RecordRequest("rest", op, pe.Code, time.Since(start))
			return
		}
		WriteJSON(w, http.StatusOK, resp)
		insights.RecordRequest("rest", op, insights.CodeOK, time.Since(start))
	}
}

// setInstanceHeaders sets the C4 headers.
func (a *API) setInstanceHeaders(w http.ResponseWriter) {
	info := a.svc.Instance()
	w.Header().Set("X-Perf-Instance", headerSafe(info.Name))
	w.Header().Set("X-Perf-Visibility", headerSafe(info.Visibility))
}

// headerSafe keeps only printable ASCII, so config values can't inject headers.
func headerSafe(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e {
			return -1
		}
		return r
	}, s)
}

// WriteJSON writes v as a JSON response that is not cached.
func WriteJSON(w http.ResponseWriter, code int, v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		sklog.Errorf("Failed to encode response: %s", err)
		code = http.StatusInternalServerError
		b = []byte(`{"error":{"code":"INTERNAL","message":"internal error"}}`)
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if _, err := w.Write(b); err != nil {
		sklog.Warningf("Failed to write response: %s", err)
	}
}

// WriteError writes an error envelope.
func WriteError(w http.ResponseWriter, e *insights.Error) {
	WriteJSON(w, e.Code.HTTPStatus(), insights.ErrorEnvelope{Error: e})
}

func intParam(r *http.Request, name string) (int, error) {
	v := strings.TrimSpace(r.URL.Query().Get(name))
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, insights.InvalidArgument("pass an integer", "invalid %s", name)
	}
	return n, nil
}

func floatParam(r *http.Request, name string) (float64, error) {
	v := strings.TrimSpace(r.URL.Query().Get(name))
	if v == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, insights.InvalidArgument("pass a number", "invalid %s", name)
	}
	return f, nil
}

func (a *API) getInstance(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return a.svc.Instance(), nil
}

func (a *API) listSheriffs(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return a.svc.ListSheriffs(r.Context())
}

func (a *API) listAnomalies(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	days, err := intParam(r, "days")
	if err != nil {
		return nil, err
	}
	pageSize, err := intParam(r, "page_size")
	if err != nil {
		return nil, err
	}
	minAbsPct, err := floatParam(r, "min_abs_pct")
	if err != nil {
		return nil, err
	}
	q := r.URL.Query()
	return a.svc.ListAnomalies(r.Context(), insights.ListAnomaliesRequest{
		Sheriff:   q.Get("sheriff"),
		Status:    q.Get("status"),
		Direction: q.Get("direction"),
		Benchmark: q.Get("benchmark"),
		Bot:       q.Get("bot"),
		Days:      days,
		MinAbsPct: minAbsPct,
		PageSize:  pageSize,
		PageToken: q.Get("page_token"),
	})
}

func (a *API) getAnomaly(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	view := insights.View(strings.ToLower(r.URL.Query().Get("view")))
	if view == "" {
		view = insights.ViewFull
	}
	resp, err := a.svc.BatchGetAnomalies(r.Context(), []string{chi.URLParam(r, "id")}, view)
	if err != nil {
		return nil, err
	}
	if len(resp.Anomalies) == 0 {
		return nil, insights.NotFound("anomaly ids come from list_anomalies, summarize_anomalies or the Perf UI", "anomaly not found")
	}
	return resp.Anomalies[0], nil
}

// BatchGetRequest is the body of POST /api/v1/anomalies:batchGet.
type BatchGetRequest struct {
	IDs  []string `json:"ids"`
	View string   `json:"view"`
}

func (a *API) getAnomalyTrace(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	before, err := intParam(r, "commits_before")
	if err != nil {
		return nil, err
	}
	after, err := intParam(r, "commits_after")
	if err != nil {
		return nil, err
	}
	return a.svc.GetAnomalyTrace(r.Context(), insights.AnomalyTraceRequest{
		ID:            chi.URLParam(r, "id"),
		CommitsBefore: before,
		CommitsAfter:  after,
	})
}

func (a *API) batchGetAnomalies(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	// Requiring a JSON content type also forces a CORS preflight for cross-origin browser
	// requests, which this server never approves.
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return nil, insights.InvalidArgument("send the body with Content-Type: application/json", "unsupported content type")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var req BatchGetRequest
	if err := dec.Decode(&req); err != nil {
		return nil, insights.InvalidArgument(`send a JSON body like {"ids": ["<uuid>"], "view": "basic"}`, "invalid request body")
	}
	return a.svc.BatchGetAnomalies(r.Context(), req.IDs, insights.View(strings.ToLower(req.View)))
}

func (a *API) getAnomalyGroup(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return a.svc.GetAnomalyGroup(r.Context(), chi.URLParam(r, "id"))
}

func (a *API) findAnomaliesForRevision(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return a.svc.FindAnomaliesForRevision(r.Context(), chi.URLParam(r, "rev"))
}

func (a *API) summarize(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	days, err := intParam(r, "days")
	if err != nil {
		return nil, err
	}
	return a.svc.Summarize(r.Context(), insights.SummarizeRequest{Sheriff: r.URL.Query().Get("sheriff"), Days: days})
}

func (a *API) suggestTriage(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	days, err := intParam(r, "days")
	if err != nil {
		return nil, err
	}
	return a.svc.SuggestTriage(r.Context(), insights.SuggestTriageRequest{Sheriff: r.URL.Query().Get("sheriff"), Days: days})
}

func (a *API) openAPI(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "max-age=300")
	if _, err := w.Write(a.spec); err != nil {
		sklog.Warningf("Failed to write openapi.json: %s", err)
	}
}
