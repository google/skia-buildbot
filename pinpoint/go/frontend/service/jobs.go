package jobsservice

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/httputils"

	jobstore "go.skia.org/infra/pinpoint/go/sql/jobs_store"
)

const (
	defaultLimt   = 50
	defaultOffset = 0
)

// Service handles the HTTP endpoints for Pinpoint jobs.
type Service struct {
	jobStore jobstore.JobStore
}

// New creates a new Service.
func New(ctx context.Context, js jobstore.JobStore) (*Service, error) {
	return &Service{
		jobStore: js,
	}, nil
}

// ListJobsHandler handles requests for listing jobs.
// It accepts query parameters:
// - search_term: string to filter jobs by name.
// - limit: maximum number of jobs to return.
// - offset: number of jobs to skip for pagination.
func (s *Service) ListJobsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	var limit int = defaultLimt
	var offset int = defaultOffset
	var err error

	retrievedLimit := q.Get("limit")
	// Accept empty limit parameter which will default to 50
	if retrievedLimit != "" {
		limit, err = strconv.Atoi(retrievedLimit)
		if err != nil {
			msg := "Failed to convert limit to an int"
			httputils.ReportError(w, err, msg, http.StatusBadRequest)
			return
		}
	}

	retrievedOffset := q.Get("offset")
	// Accept empty limit parameter which will default to 0
	if retrievedOffset != "" {
		offset, err = strconv.Atoi(retrievedOffset)
		if err != nil {
			msg := "Failed to convert offset to an int"
			httputils.ReportError(w, err, msg, http.StatusBadRequest)
			return
		}
	}

	// Basic validation
	if offset < 0 || limit < 0 {
		msg := "Cannot accept negative numbers as parameters"
		httputils.ReportError(w, err, msg, http.StatusBadRequest)
		return
	}

	opts := jobstore.ListJobsOptions{
		SearchTerm: q.Get("search_term"),
		Limit:      limit,
		Offset:     offset,
	}

	jobs, err := s.jobStore.ListJobs(ctx, opts)
	if err != nil {
		msg := "Failed to list jobs"
		httputils.ReportError(w, err, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(jobs); err != nil {
		msg := "Failed to encode response"
		httputils.ReportError(w, err, msg, http.StatusInternalServerError)
		return
	}
}

// RegisterHandlers registers the service's HTTP handlers with a mux.
func (s *Service) RegisterHandlers(router *chi.Mux) {
	router.Get("/json/jobs/list", s.ListJobsHandler)
}
