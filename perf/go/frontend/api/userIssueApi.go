package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/userissue"
)

// userIssueApi provides a struct for handling Buganizer Annotation feature.
type userIssueApi struct {
	loginProvider  alogin.Login
	userIssueStore userissue.Store
}

// NewUserIssueApi returns a new instance of userIssueApi.
func NewUserIssueApi(loginProvider alogin.Login, userIssueStore userissue.Store) userIssueApi {
	return userIssueApi{
		loginProvider:  loginProvider,
		userIssueStore: userIssueStore,
	}
}

// RegisterHandlers registers the api handlers for their respective routes.
func (ui userIssueApi) RegisterHandlers(router *chi.Mux) {
	router.Post("/_/user_issues", ui.userIssuesHandler)
	router.Post("/_/user_issue/save", ui.saveUserIssueHandler)
	router.Post("/_/user_issue/delete", ui.deleteUserIssueHandler)
}

// GetUserIssuesForTraceKeysRequest is the request to fetch all user issues
// corresponding to a list of trace keys and commit position range
type GetUserIssuesForTraceKeysRequest struct {
	TraceKeys           []string `json:"trace_keys"`
	BeginCommitPosition int64    `json:"begin_commit_position"`
	EndCommitPosition   int64    `json:"end_commit_position"`
}

type GetUserIssuesForTraceKeysResponse struct {
	UserIssues []userissue.UserIssue
}

// userIssuesHandler returns list of user reported buganzier issues
// corresponding to the a list of trace keys and commit position range.
func (ui userIssueApi) userIssuesHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	var getIssuesReq GetUserIssuesForTraceKeysRequest
	if err := json.NewDecoder(r.Body).Decode(&getIssuesReq); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	traceKeys := getIssuesReq.TraceKeys
	begin := getIssuesReq.BeginCommitPosition
	end := getIssuesReq.EndCommitPosition

	if len(traceKeys) == 0 || begin == 0 || end == 0 {
		e := fmt.Errorf("Failed to fetch data: ")
		httputils.ReportError(w, e, "Missing Arguments", http.StatusBadRequest)
		return
	}

	userIssues, err := ui.userIssueStore.GetUserIssuesForTraceKeys(ctx, traceKeys, begin, end)
	if err != nil {
		httputils.ReportError(w, err, "Failed to fetch data", http.StatusInternalServerError)
		return
	}

	resp := GetUserIssuesForTraceKeysResponse{
		UserIssues: userIssues,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode response: %s", http.StatusInternalServerError)
	}
}

// SaveUserIssueRequest is the request to create a new User Issue
type SaveUserIssueRequest struct {
	TraceKey       string `json:"trace_key"`
	CommitPosition int64  `json:"commit_position"`
	IssueId        int64  `json:"issue_id"`
}

// saveUserIssueHandler creates a new userissue in the db
func (ui *userIssueApi) saveUserIssueHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	loggedInEmail := ui.loginProvider.LoggedInAs(r)
	if loggedInEmail == "" {
		httputils.ReportError(w, skerr.Fmt("Login Required"), "", http.StatusUnauthorized)
		return
	}

	var saveReq SaveUserIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&saveReq); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusBadRequest)
		return
	}

	if len(saveReq.TraceKey) == 0 {
		httputils.ReportError(w, skerr.Fmt("Invalid Argument: "), "trace_key", http.StatusBadRequest)
	}

	if saveReq.CommitPosition == 0 || saveReq.IssueId == 0 {
		httputils.ReportError(w, skerr.Fmt("Invalid Argument: "), "commit position and issue id", http.StatusBadRequest)
	}

	userIssueObj := userissue.UserIssue{
		UserId:         loggedInEmail.String(),
		TraceKey:       saveReq.TraceKey,
		CommitPosition: saveReq.CommitPosition,
		IssueId:        saveReq.IssueId,
	}
	err := ui.userIssueStore.Save(ctx, &userIssueObj)
	if err != nil {
		httputils.ReportError(w, err, "Failed to save.", http.StatusInternalServerError)
	}
}

// DeleteUserIssueRequest deletes an existing userissue from the db
type DeleteUserIssueRequest struct {
	TraceKey       string `json:"trace_key"`
	CommitPosition int64  `json:"commit_position"`
}

// deleteUserIssueHandler deletes a user issue from the db
func (ui *userIssueApi) deleteUserIssueHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	loggedInEmail := ui.loginProvider.LoggedInAs(r)
	if loggedInEmail == "" {
		httputils.ReportError(w, skerr.Fmt("Login Required"), "", http.StatusUnauthorized)
		return
	}

	var deleteReq DeleteUserIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&deleteReq); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	if len(deleteReq.TraceKey) == 0 || deleteReq.CommitPosition == 0 {
		httputils.ReportError(w, skerr.Fmt("Invalid arguments:"), "Both trace_key and commit_position needs be specified", http.StatusBadRequest)
		return
	}

	err := ui.userIssueStore.Delete(ctx, deleteReq.TraceKey, deleteReq.CommitPosition)
	if err != nil {
		httputils.ReportError(w, skerr.Fmt("Error:"), "Failed to remove bug from this data point.", http.StatusInternalServerError)
	}
}
