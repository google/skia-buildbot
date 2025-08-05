package jobsservice

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	pinpoint_service "go.skia.org/infra/pinpoint/go/service"
	jobstore "go.skia.org/infra/pinpoint/go/sql/jobs_store"
	pb "go.skia.org/infra/pinpoint/proto/v1"
	tpr_client "go.skia.org/infra/temporal/go/client"
	"golang.org/x/time/rate"
)

const (
	defaultLimit  = 50
	defaultOffset = 0
)

//go:embed benchmarks.json
var benchmarksJSON []byte

type BenchmarkConfig struct {
	BenchmarkName string   `json:"benchmark"`
	Stories       []string `json:"stories"`
	Bots          []string `json:"bots"`
}

// Service handles the HTTP endpoints for Pinpoint jobs.
type Service struct {
	jobStore         jobstore.JobStore
	templates        *template.Template
	benchmarkConfigs []BenchmarkConfig
	pinpointServer   pb.PinpointServer
	pinpointHandler  http.Handler
}

// New creates a new Service.
func New(ctx context.Context, js jobstore.JobStore, t tpr_client.TemporalProvider, l *rate.Limiter, resourceDir string) (*Service, error) {
	s := &Service{
		jobStore:       js,
		pinpointServer: pinpoint_service.New(t, l), // gRPC service to handle temporal interaction
	}

	handler, err := pinpoint_service.NewJSONHandler(context.Background(), s.pinpointServer)
	if err != nil {
		return nil, skerr.Fmt("failed to initalize pinpoint service %s.", err)
	}
	s.pinpointHandler = handler

	err = s.loadConfigs()
	if err != nil {
		return nil, skerr.Fmt("failed to retreive config contents: %s", err)
	}

	s.loadTemplates(resourceDir)
	return s, nil
}

// ListJobsHandler handles requests for listing jobs.
// It accepts query parameters:
// - search_term: string to filter jobs by name.
// - limit: maximum number of jobs to return.
// - offset: number of jobs to skip for pagination.
func (s *Service) ListJobsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	var limit int = defaultLimit
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
		Benchmark:  q.Get("benchmark"),
		BotName:    q.Get("bot_name"),
		User:       q.Get("user"),
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

func (s *Service) GetJobHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobID := chi.URLParam(r, "jobID")
	if jobID == "" {
		msg := "No Job ID was provided"
		httputils.ReportError(w, skerr.Fmt("no job id was provided"), msg, http.StatusBadRequest)
		return
	}

	job, err := s.jobStore.GetJob(ctx, jobID)
	if err != nil {
		msg := "Failed to receive Job info"
		httputils.ReportError(w, err, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(job); err != nil {
		msg := "Failed to encode response"
		httputils.ReportError(w, err, msg, http.StatusInternalServerError)
		return
	}

}

// templateHandler returns an http.HandlerFunc that executes the named template.
func (s *Service) templateHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		var data any // Currently we dont define Cookies or any other data to be stored in the HTML
		if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
			httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
			return
		}
	}
}

// RegisterHandlers registers the service's HTTP handlers with a mux.
func (s *Service) RegisterHandlers(router *chi.Mux) {
	router.Get("/json/jobs/list", s.ListJobsHandler)
	router.Get("/json/job/{jobID}", s.GetJobHandler)
	router.Get("/benchmarks", s.ListBenchmarksHandler)
	router.Get("/bots", s.ListBotConfigurationsHandler)
	router.Get("/stories", s.ListStoriesHandler)
	router.HandleFunc("/", s.templateHandler("landing-page.html"))
	router.HandleFunc("/results/jobid/{jobID}", s.templateHandler("results-page.html"))
	router.HandleFunc("/pinpoint/*", s.pinpointHandler.ServeHTTP)
}

// ListBenchmarksHandler handles requests for listing chromeperf benchmarks.
func (s *Service) ListBenchmarksHandler(w http.ResponseWriter, r *http.Request) {
	benchmarks := make([]string, 0, len(s.benchmarkConfigs))
	for _, config := range s.benchmarkConfigs {
		benchmarks = append(benchmarks, config.BenchmarkName)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(benchmarks); err != nil {
		msg := "Failed to encode response"
		httputils.ReportError(w, err, msg, http.StatusInternalServerError)
		return
	}
}

// ListBotConfigurationsHandler handles requests for listing available chromeperf bots based on a given benchmark.
func (s *Service) ListBotConfigurationsHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	benchmarkName := q.Get("benchmark")

	// If no benchmark is specified, return all unique bots.
	if benchmarkName == "" {
		allBots := make(map[string]struct{})
		for _, config := range s.benchmarkConfigs {
			for _, bot := range config.Bots {
				allBots[bot] = struct{}{}
			}
		}
		bots := make([]string, 0, len(allBots))
		for bot := range allBots {
			bots = append(bots, bot)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(bots); err != nil {
			httputils.ReportError(w, err, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	// If a benchmark is specified, find it and return its bots.
	for _, config := range s.benchmarkConfigs {
		if config.BenchmarkName == benchmarkName {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(config.Bots); err != nil {
				httputils.ReportError(w, err, "Failed to encode response", http.StatusInternalServerError)
			}
			return
		}
	}
}

// ListStoriesHandler handles requests for listing chromeperf stories based on provided benchmark.
func (s *Service) ListStoriesHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	benchmark := q.Get("benchmark")
	var stories []string
	found := false
	for _, config := range s.benchmarkConfigs {
		if config.BenchmarkName == benchmark {
			found = true
			stories = config.Stories
			break
		}
	}

	if !found {
		msg := "Failed to find bot configurations"
		httputils.ReportError(w, fmt.Errorf("story values were not found"), msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stories); err != nil {
		msg := "Failed to encode response"
		httputils.ReportError(w, err, msg, http.StatusInternalServerError)
		return
	}
}

// loadTemplates loads the HTML templates from the given directory.
func (s *Service) loadTemplates(resourcesDir string) {
	s.templates = template.Must(template.New("").Delims("{%", "%}").ParseGlob(filepath.Join(resourcesDir, "*.html")))
}

// loadConfigs loads the values retrived from the JSON file defined in configPath.
func (s *Service) loadConfigs() error {
	decoder := json.NewDecoder(bytes.NewReader(benchmarksJSON))
	err := decoder.Decode(&s.benchmarkConfigs)
	if err != nil {
		return skerr.Fmt("failed to decode from embedded benchmarks.json: %s", err)
	}
	return nil
}
