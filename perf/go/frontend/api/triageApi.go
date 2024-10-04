package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/anomalies"
	"go.skia.org/infra/perf/go/chromeperf"
)

const (
	defaultRequestProcessTimeout = time.Second * 30
	defaultEditAnomalyTimeout    = time.Second * 5
)

type triageApi struct {
	// TODO(wenbinzhang): add pinpoint client and issuetracker client to complete
	// the triage toolchain when skia backend is ready.
	chromeperfClient chromeperf.ChromePerfClient
	loginProvider    alogin.Login
	anomalyStore     anomalies.Store
}

// Request object for the request from new bug UI.
type FileBugRequest struct {
	Keys        []int    `json:"keys"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Component   string   `json:"component"`
	Assignee    string   `json:"assignee,omitempty"`
	Ccs         []string `json:"ccs,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	TraceNames  []string `json:"trace_names,omitempty"`
}

// Existing bug request object to asscociate alerts from new bug UI.
type SkiaAssociateBugRequest struct {
	BugId      int      `json:"bug_id"`
	ProjectId  string   `json:"project_id"`
	Keys       []int    `json:"keys"`
	TraceNames []string `json:"trace_names"`
}

// Response object for Skia UI.
type SkiaFileBugResponse struct {
	BugId int `json:"bug_id,omitempty"`
}

// Existing bug response object for Skia UI.
type SkiaAssociateBugResponse struct {
	BugId int `json:"bug_id,omitempty"`
}

// Response object from the chromeperf associate alerts to existing bug response.
type ChromeperfAssociateBugResponse struct {
	Error string `json:"error,omitempty"`
}

// Response object from the chromeperf file bug response.
type ChromeperfFileBugResponse struct {
	BugId int    `json:"bug_id"`
	Error string `json:"error"`
}

// Request object for the request from the following triage actions:
//   - Ignore
//   - X button (untriage the anomaly)
//   - Nudge (move the anomaly position to adjacent datapoints)
type EditAnomaliesRequest struct {
	Keys          []int    `json:"keys"`
	BugId         int      `json:"bug_id,omitempty"`
	StartRevision int      `json:"start_revision,omitempty"`
	EndRevision   int      `json:"end_revision,omitempty"`
	TraceNames    []string `json:"trace_names"`
}

// Response object from the chromeperf edit anomaly request.
type EditAnomaliesResponse struct {
	Error string `json:"error"`
}

func (api triageApi) RegisterHandlers(router *chi.Mux) {
	router.Post("/_/triage/file_bug", api.FileNewBug)
	router.Post("/_/triage/edit_anomalies", api.EditAnomalies)
	router.Post("/_/triage/associate_alerts", api.AssociateAlerts)
}

func NewTriageApi(loginProvider alogin.Login, chromeperfClient chromeperf.ChromePerfClient, anomalyStore anomalies.Store) triageApi {
	return triageApi{
		loginProvider:    loginProvider,
		chromeperfClient: chromeperfClient,
		anomalyStore:     anomalyStore,
	}
}

func (api triageApi) FileNewBug(w http.ResponseWriter, r *http.Request) {
	if api.loginProvider.LoggedInAs(r) == "" {
		httputils.ReportError(w, errors.New("Not logged in"), fmt.Sprintf("You must be logged in to complete this action."), http.StatusUnauthorized)
		return
	}

	var fileBugRequest FileBugRequest
	if err := json.NewDecoder(r.Body).Decode(&fileBugRequest); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON on new bug request.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] File new bug request received from frontend: %s", fileBugRequest)

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultRequestProcessTimeout)
	defer cancel()
	chromeperfResponse := &ChromeperfFileBugResponse{}

	err := api.chromeperfClient.SendPostRequest(ctx, "file_bug_skia", "", fileBugRequest, chromeperfResponse, []int{200, 400, 401, 500})
	if err != nil {
		httputils.ReportError(w, err, "Failed to finish new bug request.", http.StatusInternalServerError)
		return
	}

	if chromeperfResponse.Error != "" {
		httputils.ReportError(w, errors.New(chromeperfResponse.Error), "New bug request returned error message.", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(SkiaFileBugResponse{BugId: chromeperfResponse.BugId}); err != nil {
		httputils.ReportError(w, err, "Failed to write bug id to SkiaFileBugResponse.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] b/%s is created.", chromeperfResponse.BugId)

	api.markTracesForCacheInvalidation(ctx, fileBugRequest.TraceNames)

	return
}

func (api triageApi) EditAnomalies(w http.ResponseWriter, r *http.Request) {
	if api.loginProvider.LoggedInAs(r) == "" {
		httputils.ReportError(w, errors.New("Not logged in"), fmt.Sprintf("You must be logged in to complete this action."), http.StatusUnauthorized)
		return
	}

	var editAnomaliesRequest EditAnomaliesRequest
	if err := json.NewDecoder(r.Body).Decode(&editAnomaliesRequest); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON on edit anomalies request.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] Edit anomalies request received from frontend: %s", editAnomaliesRequest)

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultEditAnomalyTimeout)
	defer cancel()

	editAnomalyResponse := &EditAnomaliesResponse{}

	err := api.chromeperfClient.SendPostRequest(ctx, "edit_anomalies_skia", "", editAnomaliesRequest, editAnomalyResponse, []int{200, 400, 401, 500})
	if err != nil {
		httputils.ReportError(w, err, "Failed to finish edit anomalies request.", http.StatusInternalServerError)
		return
	}

	if editAnomalyResponse.Error != "" {
		// TODO(wenbinzhang): Should update all end-user facing messages to be more informative and actionable. b/369622563.
		httputils.ReportError(w, errors.New(editAnomalyResponse.Error), "Edit anomalies request returned error message.", http.StatusInternalServerError)
		return
	}

	if error := json.NewEncoder(w).Encode(editAnomalyResponse); error != nil {
		httputils.ReportError(w, error, "Failed to enode JSON on edit anomalies response.", http.StatusInternalServerError)
		return
	}

	sklog.Debugf("[SkiaTriage] Anomalies (%d) are updated with: bug_id: %d, start_revision: %d, end_revision: %d", editAnomaliesRequest.Keys, editAnomaliesRequest.BugId, editAnomaliesRequest.StartRevision, editAnomaliesRequest.EndRevision)

	api.markTracesForCacheInvalidation(ctx, editAnomaliesRequest.TraceNames)

	return
}

func (api triageApi) AssociateAlerts(w http.ResponseWriter, r *http.Request) {
	if api.loginProvider.LoggedInAs(r) == "" {
		httputils.ReportError(w, errors.New("not logged in"), fmt.Sprint("You must be logged in to complete this action."), http.StatusUnauthorized)
		return
	}

	var associateBugRequest SkiaAssociateBugRequest
	if err := json.NewDecoder(r.Body).Decode(&associateBugRequest); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON on associate bug request.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] Associate bug request received from frontend: %s", associateBugRequest)

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultRequestProcessTimeout)
	defer cancel()

	skiaExistingBugResponse := &ChromeperfAssociateBugResponse{}
	err := api.chromeperfClient.SendPostRequest(ctx, "associate_alerts_skia", "", associateBugRequest, skiaExistingBugResponse, []int{200, 400, 401, 500})
	if err != nil {
		httputils.ReportError(w, err, "Failed to send request to associate_alerts_skia.", http.StatusInternalServerError)
		return
	}
	if error := json.NewEncoder(w).Encode(skiaExistingBugResponse); error != nil {
		httputils.ReportError(w, error, "Failed to enode JSON on associate bug response.", http.StatusInternalServerError)
		return
	}

	if skiaExistingBugResponse.Error != "" {
		httputils.ReportError(w, errors.New(skiaExistingBugResponse.Error), "Associate alerts with existing bug request returned error message.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] Alerts are associated with existing bug.")

	api.markTracesForCacheInvalidation(ctx, associateBugRequest.TraceNames)
}

// For each trace name, mark it as invalidated in the anomalystore's tests cache.
func (api triageApi) markTracesForCacheInvalidation(ctx context.Context, traceNames []string) {
	for _, traceName := range traceNames {
		api.anomalyStore.InvalidateTestsCacheForTraceName(ctx, traceName)
	}
	sklog.Debugf("[SkiaTriage] The following traces in cache are marked invalidated: %s", traceNames)
}
