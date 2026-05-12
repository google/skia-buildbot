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
	"go.skia.org/infra/go/issuetracker/v1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	perf_issuetracker "go.skia.org/infra/perf/go/issuetracker"
)

const (
	defaultRequestProcessTimeout = time.Second * 30
	defaultEditAnomalyTimeout    = time.Second * 5
)

type triageApi struct {
	// TODO(wenbinzhang): add pinpoint client and issuetracker client to complete
	// the triage toolchain when skia backend is ready.
	cpTriageBackend  TriageBackend
	sqlTriageBackend TriageBackend
	loginProvider    alogin.Login
	issueTracker     perf_issuetracker.IssueTracker
}

func (api triageApi) getTriageBackend(r *http.Request) TriageBackend {
	legacyPreference := preferLegacy(r)
	if legacyPreference {
		return api.cpTriageBackend
	}
	return api.sqlTriageBackend
}

// Existing bug request object to asscociate alerts from new bug UI.
type SkiaAssociateBugRequest struct {
	BugId      int      `json:"bug_id"`
	Keys       []string `json:"keys"`
	TraceNames []string `json:"trace_names"`
}

// Response object for Skia UI.
type SkiaFileBugResponse struct {
	BugId int    `json:"bug_id,omitempty"`
	Error string `json:"error,omitempty"`
}

// Existing bug response object for Skia UI.
type SkiaAssociateBugResponse struct {
	BugId int    `json:"bug_id,omitempty"`
	Error string `json:"error,omitempty"`
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
//   - Reset - X button (untriage the anomaly)
//   - Nudge (move the anomaly position to adjacent datapoints)
type EditAnomaliesRequest struct {
	Keys                []string `json:"keys"`
	Action              string   `json:"action"`
	StartRevision       int      `json:"start_revision,omitempty"`
	EndRevision         int      `json:"end_revision,omitempty"`
	DisplayCommitNumber int      `json:"display_commit_number,omitempty"`
	TraceNames          []string `json:"trace_names"`
}

// ListIssuesResponse defines the response object for ListIssues.
type ListIssuesResponse struct {
	// Issues: The current page of issues.
	Issues []*issuetracker.Issue `json:"issues,omitempty"`
}

// Response object from the chromeperf edit anomaly request.
type EditAnomaliesResponse struct {
	Error string `json:"error"`
}

func (api triageApi) RegisterHandlers(router *chi.Mux) {
	router.Post("/_/triage/file_bug", api.FileNewBug)
	router.Post("/_/triage/edit_anomalies", api.EditAnomalies)
	router.Post("/_/triage/associate_alerts", api.AssociateAlerts)
	router.Post("/_/triage/list_issues", api.ListIssues)
}

func NewTriageApi(loginProvider alogin.Login, cpTriageBackend TriageBackend, sqlTriageBackend TriageBackend, issueTracker perf_issuetracker.IssueTracker) triageApi {
	return triageApi{
		loginProvider:    loginProvider,
		cpTriageBackend:  cpTriageBackend,
		sqlTriageBackend: sqlTriageBackend,
		issueTracker:     issueTracker,
	}
}

func cleanErrorMsg(err error) string {
	if wrapper, ok := err.(*skerr.ErrorWithContext); ok {
		if len(wrapper.Context) > 0 {
			return wrapper.Context[len(wrapper.Context)-1]
		}
	}
	return err.Error()
}

func (api triageApi) FileNewBug(w http.ResponseWriter, r *http.Request) {
	if api.loginProvider.LoggedInAs(r) == "" {
		httputils.ReportError(w, errors.New("Not logged in"), "You must be logged in to complete this action.", http.StatusUnauthorized)
		return
	}

	var fileBugRequest perf_issuetracker.FileBugRequest
	if err := json.NewDecoder(r.Body).Decode(&fileBugRequest); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON on new bug request.", http.StatusInternalServerError)
		return
	}
	if fileBugRequest.Host == "" {
		fileBugRequest.Host = config.Config.URL
	}
	fileBugRequest.Host = getOverrideNonProdHost(fileBugRequest.Host)
	sklog.Debugf("[SkiaTriage] File new bug request received from frontend: %s", fileBugRequest)

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultRequestProcessTimeout)
	defer cancel()

	triageBackend := api.getTriageBackend(r)
	if triageBackend == nil {
		httputils.ReportError(w, errors.New("triage backend not configured"), "Triage backend is not configured.", http.StatusInternalServerError)
		return
	}
	resp, err := triageBackend.FileBug(ctx, &fileBugRequest)
	if err != nil {
		sklog.Error("File new bug request failed.", err)
		cleanMsg := cleanErrorMsg(err)
		resp = &SkiaFileBugResponse{Error: cleanMsg}
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httputils.ReportError(w, err, "Failed to encode JSON on file bug response.", http.StatusInternalServerError)
		return
	}
	if resp.Error == "" {
		sklog.Debugf("[SkiaTriage] b/%d is created.", resp.BugId)
	}
}

// EditAnomalies updates data about an anomaly by forwarding the request to Chromeperf's
// edit_anomalies_skia. The "keys", is a required field. They map to ndb Anomaly keys in
// Datastore and are used to fetch the Anomaly object and updated with the new details,
// whether that be the Bug ID due to triage or end revision due to nudging.
// TODO(b/455571863) Update this description after migration is implemented.
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

	if editAnomaliesRequest.StartRevision < 0 || editAnomaliesRequest.EndRevision < 0 {
		http.Error(w, "Invalid start or end revision.", http.StatusBadRequest)
		return
	}
	if editAnomaliesRequest.EndRevision < editAnomaliesRequest.StartRevision {
		http.Error(w, "End revision cannot be less than start revision.", http.StatusBadRequest)
		return
	}
	if len(editAnomaliesRequest.Action) == 0 {
		http.Error(w, "Action must be a nonempty string.", http.StatusBadRequest)
		return
	}
	// "keys" is required by Chromeperf API and will return 400 if not present,
	// but avoid sending request and terminate early if missing.
	if len(editAnomaliesRequest.Keys) < 1 {
		http.Error(w, "Missing anomaly keys.", http.StatusBadRequest)
		return
	}
	triageBackend := api.getTriageBackend(r)
	if triageBackend == nil {
		httputils.ReportError(w, errors.New("triage backend not configured"), "Triage backend is not configured.", http.StatusInternalServerError)
		return
	}
	resp, err := triageBackend.EditAnomalies(ctx, &editAnomaliesRequest)
	if err != nil {
		sklog.Error("Edit anomalies request failed.", err)
		cleanMsg := cleanErrorMsg(err)
		resp = &EditAnomaliesResponse{Error: cleanMsg}
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httputils.ReportError(w, err, "Failed to encode JSON on edit anomalies response.", http.StatusInternalServerError)
		return
	}

	if resp.Error == "" {
		sklog.Debugf("[SkiaTriage] Anomalies (%d) are updated with: action: %s, start_revision: %d, end_revision: %d", editAnomaliesRequest.Keys, editAnomaliesRequest.Action, editAnomaliesRequest.StartRevision, editAnomaliesRequest.EndRevision)
	}
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

	triageBackend := api.getTriageBackend(r)
	if triageBackend == nil {
		httputils.ReportError(w, errors.New("triage backend not configured"), "Triage backend is not configured.", http.StatusInternalServerError)
		return
	}
	resp, err := triageBackend.AssociateAlerts(ctx, &associateBugRequest)
	if err != nil {
		sklog.Error("Associate alerts request failed.", err)
		cleanMsg := cleanErrorMsg(err)
		resp = &SkiaAssociateBugResponse{Error: cleanMsg}
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httputils.ReportError(w, err, "Failed to encode JSON on associate bug response.", http.StatusInternalServerError)
		return
	}
	if resp.Error == "" {
		sklog.Debugf("[SkiaTriage] Alerts are associated with existing bug.")
	}
}

func (api triageApi) ListIssues(w http.ResponseWriter, r *http.Request) {
	if api.issueTracker == nil {
		httputils.ReportError(w, skerr.Fmt("IssueTracker client is not available on this instance"), "IssueTracker client is not available on this instance.", http.StatusForbidden)
	}

	if api.loginProvider.LoggedInAs(r) == "" {
		httputils.ReportError(w, errors.New("not logged in"), fmt.Sprint("You must be logged in to complete this action."), http.StatusUnauthorized)
		return
	}

	var ListIssuesRequest perf_issuetracker.ListIssuesRequest
	if err := json.NewDecoder(r.Body).Decode(&ListIssuesRequest); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON on bug title request.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] ListIssues request received from frontend: %s", ListIssuesRequest)

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultRequestProcessTimeout)
	defer cancel()

	sklog.Debugf("[SkiaTriage] Start sending list issues request to issuetracker.")

	resp, err := api.issueTracker.ListIssues(ctx, ListIssuesRequest)

	if err != nil {
		httputils.ReportError(
			w,
			err,
			"ListIssues request failed due to an internal server error. Please try again.",
			http.StatusInternalServerError)
		return
	}

	if len(resp) > 0 {
		sklog.Debugf("[SkiaTriage] Fetched and returned ListIssue IssueId: %s and IssueState.Title %s", resp[0].IssueId, resp[0].IssueState.Title)
	}

	if err := json.NewEncoder(w).Encode(ListIssuesResponse{Issues: resp}); err != nil {
		httputils.ReportError(w, err, "Failed to write bug id to ListIssuesResponse.", http.StatusInternalServerError)
		return
	}
}
