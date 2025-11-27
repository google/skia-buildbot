package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	issuetracker "go.skia.org/infra/go/issuetracker/v1"
	"go.skia.org/infra/go/sklog"
)

func main() {
	r := chi.NewRouter()
	r.Get("/v1/issues", listIssuesHandler)
	r.Post("/v1/issues/{issueId}/comments", createCommentHandler)
	r.Post("/v1/issues", fileBugHandler)

	sklog.Info("Starting mock issuetracker server on :8081")
	if err := http.ListenAndServe(":8081", r); err != nil {
		sklog.Fatalf("Failed to start server: %s", err)
	}
}

func listIssuesHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	sklog.Infof("Mock issuetracker: listIssuesHandler with query: %s", query)

	// A very simple mock response.
	resp := issuetracker.ListIssuesResponse{
		Issues: []*issuetracker.Issue{
			{
				IssueId: 12345,
				IssueState: &issuetracker.IssueState{
					Title: "Mock issue",
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode response: %s", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func createCommentHandler(w http.ResponseWriter, r *http.Request) {
	issueIdStr := chi.URLParam(r, "issueId")
	issueId, err := strconv.ParseInt(issueIdStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid issueId", http.StatusBadRequest)
		return
	}
	sklog.Infof("Mock issuetracker: createCommentHandler for issueId: %d", issueId)

	var comment issuetracker.IssueComment
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		http.Error(w, "Failed to decode request body", http.StatusBadRequest)
		return
	}

	resp := &issuetracker.IssueComment{
		IssueId:       issueId,
		CommentNumber: 1,
		Comment:       comment.Comment,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode response: %s", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func fileBugHandler(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("Mock issuetracker: fileBugHandler")

	var issue issuetracker.Issue
	if err := json.NewDecoder(r.Body).Decode(&issue); err != nil {
		http.Error(w, "Failed to decode request body", http.StatusBadRequest)
		return
	}

	resp := &issuetracker.Issue{
		IssueId: 98765, // A mock issue ID.
		IssueState: &issuetracker.IssueState{
			Title: issue.IssueState.Title,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode response: %s", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
