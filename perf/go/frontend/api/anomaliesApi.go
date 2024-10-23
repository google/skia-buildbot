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

// Request object for the request from new bug UI.
type GetSheriffListResponse struct {
	SheriffList []string `json:"sheriff_list"`
	Error       string   `json:"error"`
}

func (api anomaliesApi) RegisterHandlers(router *chi.Mux) {
	router.Get("/_/anomalies/sheriff_list", api.GetSheriffList)
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
