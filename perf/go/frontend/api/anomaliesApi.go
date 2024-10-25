package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/chromeperf"
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
}

// Request object for the request from the anomaly table UI.
type GetAnomaliesResponse struct {
	// The list of anomalies.
	Anomalies []chromeperf.Anomaly `json:"anomaly_list"`
	// The cursor of the current query. It will be used to 'Load More' for the next query.
	QueryCursor string `json:"anomaly_cursor"`
	// Error message if any.
	Error string `json:"error"`
}

func (api anomaliesApi) RegisterHandlers(router *chi.Mux) {
	router.Get("/_/anomalies/sheriff_list", api.GetSheriffList)
	router.Get("/_/anomalies/anomaly_list", api.GetAnomalyList)
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

	var getAnoamliesRequest GetAnomaliesRequest
	if err := json.NewDecoder(r.Body).Decode(&getAnoamliesRequest); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON on get anomalies request.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("[SkiaTriage] Get anomalies request received from frontend: %s", getAnoamliesRequest)

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultAnomaliesRequestTimeout)
	defer cancel()
	getAnoamliesResponse := &GetAnomaliesResponse{}

	err := api.chromeperfClient.SendPostRequest(ctx, "alerts_skia", "", getAnoamliesRequest, getAnoamliesResponse, []int{200, 400})
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
