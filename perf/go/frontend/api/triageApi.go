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
	"go.skia.org/infra/perf/go/chromeperf"
)

const (
	defaultFileBugTimeout = time.Second * 30
)

type triageApi struct {
	// TODO(wenbinzhang): add pinpoint client and issuetracker client to complete
	// the triage toolchain when skia backend is ready.
	chromeperfClient chromeperf.ChromePerfClient
	loginProvider    alogin.Login
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
}

// Response object for Skia UI.
type SkiaFileBugResponse struct {
	BugId int `json:"bug_id"`
}

// Response object from the chromeperf file bug request.
type ChromeperfFileBugResponse struct {
	BugId int    `json:"bug_id"`
	Error string `json:"error"`
}

func (api triageApi) RegisterHandlers(router *chi.Mux) {
	router.Post("/_/triage/file_bug", api.FileNewBug)
}

func NewTriageApi(loginProvider alogin.Login, chromeperfClient chromeperf.ChromePerfClient) triageApi {
	return triageApi{
		loginProvider:    loginProvider,
		chromeperfClient: chromeperfClient,
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
	sklog.Debugf("[SkiaTriage] File new bug request received from frondend: %s", fileBugRequest)

	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), defaultFileBugTimeout)
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

	return
}
