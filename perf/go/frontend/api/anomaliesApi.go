package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/config"
)

const (
	defaultAnomaliesRequestTimeout = time.Second * 30
)

type anomaliesApi struct {
	chromeperfClient chromeperf.ChromePerfClient
	loginProvider    alogin.Login
}

// Response object for the request from sheriff list UI.
type GetSheriffListResponse struct {
	SheriffList []string `json:"sheriff_list"`
	Error       string   `json:"error"`
}

// Request object for the request from the anomaly table UI.
type GetAnomaliesRequest struct {
	Sheriff             string `json:"sheriff"`
	IncludeTriaged      bool   `json:"triaged"`
	IncludeImprovements bool   `json:"improvements"`
	QueryCursor         string `json:"anomaly_cursor"`
	Host                string `json:"host"`
}

// Response object for the request from the anomaly table UI.
type GetAnomaliesResponse struct {
	// The list of anomalies.
	Anomalies []chromeperf.Anomaly `json:"anomaly_list"`
	// The cursor of the current query. It will be used to 'Load More' for the next query.
	QueryCursor string `json:"anomaly_cursor"`
	// Error message if any.
	Error string `json:"error"`
}

// Request object from report page to load the anomalies from Chromeperf
type GetGroupReportRequest struct {
	// A revision number.
	Revison string `json:"rev"`
	// Comma-separated list of urlsafe Anomaly keys.
	AnomalyIDs string `json:"anomalyIDs"`
	// A Buganizer bug number ID.
	BugID string `json:"bugID"`
	// An Anomaly Group ID
	AnomalyGroupID string `json:"anomalyGroupID"`
	// A hash of a group of anomaly keys.
	Sid string `json:"sid"`
}

type GetGroupReportByKeysRequest struct {
	Keys string `json:"keys"`
}

type GetGroupReportResponse struct {
	// The list of anomalies.
	Anomalies []chromeperf.Anomaly `json:"anomaly_list"`
	// The state id (hash of a list of anomaly keys)
	// It is used in a share-able link for a report with multiple keys.
	// This is generated on Chromeperf side and returned on POST call to /alerts_skia_by_keys
	StateId string `json:"sid"`
	// Error message if any.
	Error string `json:"error"`
}

func (api anomaliesApi) RegisterHandlers(router *chi.Mux) {
	router.Get("/_/anomalies/sheriff_list", api.GetSheriffList)
	router.Get("/_/anomalies/anomaly_list", api.GetAnomalyList)
	router.Post("/_/anomalies/group_report", api.GetGroupReport)
}

func NewAnomaliesApi(loginProvider alogin.Login, chromeperfClient chromeperf.ChromePerfClient) anomaliesApi {
	return anomaliesApi{
		loginProvider:    loginProvider,
		chromeperfClient: chromeperfClient,
	}
}

func (api anomaliesApi) GetSheriffList(w http.ResponseWriter, r *http.Request) {
	if api.loginProvider.LoggedInAs(r) == "" {
		httputils.ReportError(w, errors.New("Not logged in"), fmt.Sprintf("You must be logged in to complete this action."), http.StatusUnauthorized)
		return
	}

	sklog.Debug("[SkiaTriage] Get sheriff config list request received from frontend.")

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultAnomaliesRequestTimeout)
	defer cancel()

	getSheriffListResponse := &GetSheriffListResponse{}
	err := api.chromeperfClient.SendGetRequest(ctx, "sheriff_configs_skia", "", url.Values{}, getSheriffListResponse)
	if err != nil {
		httputils.ReportError(w, err, "Failed to finish get sheriff list request.", http.StatusInternalServerError)
		return
	}

	if getSheriffListResponse.Error != "" {
		httputils.ReportError(w, errors.New(getSheriffListResponse.Error), "Load sheriff list request returned error.", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(getSheriffListResponse); err != nil {
		httputils.ReportError(w, err, "Failed to write sheriff list to UI response.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] sheriff config list is loaded: %s", getSheriffListResponse.SheriffList)

	return
}

func (api anomaliesApi) GetAnomalyList(w http.ResponseWriter, r *http.Request) {
	if api.loginProvider.LoggedInAs(r) == "" {
		httputils.ReportError(w, errors.New("Not logged in"), fmt.Sprintf("You must be logged in to complete this action."), http.StatusUnauthorized)
		return
	}

	query_values := r.URL.Query()
	sklog.Debugf("[SkiaTriage] Get anomalies request received from frontend: %s", query_values)
	if query_values.Get("host") == "" {
		query_values["host"] = []string{config.Config.URL}
	}

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultAnomaliesRequestTimeout)
	defer cancel()
	getAnoamliesResponse := &GetAnomaliesResponse{}

	err := api.chromeperfClient.SendGetRequest(ctx, "alerts_skia", "", query_values, getAnoamliesResponse)
	if err != nil {
		httputils.ReportError(w, err, "Get anomalies request failed due to an internal server error. Please try again.", http.StatusInternalServerError)
		return
	}

	if getAnoamliesResponse.Error != "" {
		httputils.ReportError(w, errors.New(getAnoamliesResponse.Error), fmt.Sprintf("Error when getting the anomaly list. Please double check each request parameter, and try again: %v", getAnoamliesResponse.Error), http.StatusBadRequest)
		return
	}

	if err := json.NewEncoder(w).Encode(getAnoamliesResponse); err != nil {
		httputils.ReportError(w, err, "Failed to write get anoamlies response.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] %d anomalies are received.", len(getAnoamliesResponse.Anomalies))

	return
}

// This function is to redirect the report page request to the group_report
// endpoint in Chromeperf.
func (api anomaliesApi) GetGroupReport(w http.ResponseWriter, r *http.Request) {
	if api.loginProvider.LoggedInAs(r) == "" {
		httputils.ReportError(w, errors.New("Not logged in"), fmt.Sprintf("You must be logged in to complete this action."), http.StatusUnauthorized)
		return
	}

	var err error
	var groupReportRequest GetGroupReportRequest
	if err = json.NewDecoder(r.Body).Decode(&groupReportRequest); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON on anomaly group report request.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] Anomaly group report request received from frontend: %s", groupReportRequest)

	if !IsGroupReportRequestValid(groupReportRequest) {
		httputils.ReportError(w, err, "Group report request is invalid.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultAnomaliesRequestTimeout)
	defer cancel()
	groupReportResponse := &GetGroupReportResponse{}

	if groupReportRequest.AnomalyIDs != "" {
		if len(strings.Split(groupReportRequest.AnomalyIDs, ",")) == 1 {
			err = api.chromeperfClient.SendGetRequest(ctx, "alerts_skia_by_key", "", url.Values{"key": {groupReportRequest.AnomalyIDs}}, groupReportResponse)
		} else {
			groupReportByKeysRequest := &GetGroupReportByKeysRequest{
				Keys: groupReportRequest.AnomalyIDs,
			}
			err = api.chromeperfClient.SendPostRequest(ctx, "alerts_skia_by_keys", "", groupReportByKeysRequest, groupReportResponse, []int{200, 400, 500})
		}
	} else if groupReportRequest.BugID != "" {
		err = api.chromeperfClient.SendGetRequest(ctx, "alerts_skia_by_bug_id", "", url.Values{"bug_id": {groupReportRequest.BugID}}, groupReportResponse)
	} else if groupReportRequest.Sid != "" {
		err = api.chromeperfClient.SendGetRequest(ctx, "alerts_skia_by_sid", "", url.Values{"sid": {groupReportRequest.Sid}}, groupReportResponse)
	} else {
		httputils.ReportError(w, errors.New("Invalid Request"), fmt.Sprintf("Group report request does not have valid parameters, or the parameter provided is not yet supported: %s", groupReportRequest), http.StatusBadRequest)
	}

	if err != nil {
		httputils.ReportError(w, err, "Anomaly group report request failed due to an internal server error. Please try again.", http.StatusInternalServerError)
		return
	}

	if groupReportResponse.Error != "" {
		httputils.ReportError(w, errors.New(groupReportResponse.Error), fmt.Sprintf("Error when getting the anomaly report group. Please double check each request parameter, and try again: %v", groupReportResponse.Error), http.StatusBadRequest)
		return
	}

	if err := json.NewEncoder(w).Encode(groupReportResponse); err != nil {
		httputils.ReportError(w, err, "Failed to write anomaly report response.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] %d anomalies are received from anomaly report group.", len(groupReportResponse.Anomalies))

	return
}

func IsGroupReportRequestValid(req GetGroupReportRequest) bool {
	valid_param_count := 0
	if req.AnomalyIDs != "" {
		valid_param_count += 1
	}
	if req.BugID != "" {
		valid_param_count += 1
	}
	if req.Sid != "" {
		valid_param_count += 1
	}
	return valid_param_count == 1
}
