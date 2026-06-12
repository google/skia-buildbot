package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/anomalies"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/dataframe"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/types"
)

// TraceValuesApi handles requests for specific trace values.
type TraceValuesApi struct {
	dfBuilder              dataframe.DataFrameBuilder
	perfGit                perfgit.Git
	anomalyStore           anomalies.Store
	chromeperfAnomalyStore anomalies.Store
}

// NewTraceValuesApi returns a new TraceValuesApi.
func NewTraceValuesApi(dfBuilder dataframe.DataFrameBuilder, perfGit perfgit.Git, anomalyStore anomalies.Store, chromeperfAnomalyStore anomalies.Store) *TraceValuesApi {
	return &TraceValuesApi{
		dfBuilder:              dfBuilder,
		perfGit:                perfGit,
		anomalyStore:           anomalyStore,
		chromeperfAnomalyStore: chromeperfAnomalyStore,
	}
}

// RegisterHandlers registers the API handlers.
func (api *TraceValuesApi) RegisterHandlers(router *chi.Mux) {
	router.Post("/_/trace_values", api.traceValuesHandler)
}

// TraceValuesRequest is the request for trace values.
type TraceValuesRequest struct {
	Ids       []string `json:"ids"`
	MinCommit int64    `json:"min_commit"`
	MaxCommit int64    `json:"max_commit"`
	Begin     int64    `json:"begin"`
	End       int64    `json:"end"`
}

// TraceValuesResponse is the response for trace values.
type TraceValuesResponse struct {
	Results    map[string][]TraceRow `json:"results"`
	AnomalyMap chromeperf.AnomalyMap `json:"anomalymap,omitempty"`
}

// TraceRow represents a single data point.
type TraceRow struct {
	CommitNumber int64   `json:"commit_number"`
	CreatedAt    int64   `json:"createdat"`
	Val          float32 `json:"val"`
}

func (api *TraceValuesApi) traceValuesHandler(w http.ResponseWriter, r *http.Request) {
	var req TraceValuesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()

	// Convert commit numbers to times
	var beginTime, endTime time.Time
	if req.Begin > 0 {
		beginTime = time.Unix(req.Begin, 0)
	} else if req.MinCommit > 0 {
		c, err := api.perfGit.CommitFromCommitNumber(ctx, types.CommitNumber(req.MinCommit))
		if err != nil {
			httputils.ReportError(w, err, "Failed to get commit for min_commit", http.StatusInternalServerError)
			return
		}
		beginTime = time.Unix(c.Timestamp, 0)
	}

	if req.End > 0 {
		endTime = time.Unix(req.End, 0)
	} else if req.MaxCommit > 0 {
		mostRecentCommit, err := api.perfGit.CommitNumberFromTime(ctx, time.Time{})
		if err != nil {
			httputils.ReportError(w, err, "Failed to get most recent commit", http.StatusInternalServerError)
			return
		}

		effectiveMaxCommit := types.CommitNumber(req.MaxCommit)
		if effectiveMaxCommit > mostRecentCommit {
			sklog.Warningf("Requested max_commit %d is beyond most recent commit %d. Capping to %d.", req.MaxCommit, mostRecentCommit, mostRecentCommit)
			effectiveMaxCommit = mostRecentCommit
		}

		c, err := api.perfGit.CommitFromCommitNumber(ctx, effectiveMaxCommit)
		if err != nil {
			httputils.ReportError(w, err, "Failed to get commit for max_commit", http.StatusInternalServerError)
			return
		}
		endTime = time.Unix(c.Timestamp, 0)
	} else {
		endTime = time.Now()
	}

	// Fetch data
	df, err := api.dfBuilder.NewFromKeysAndRange(ctx, req.Ids, beginTime, endTime, progress.New())
	if err != nil {
		httputils.ReportError(w, err, "Failed to fetch data", http.StatusInternalServerError)
		return
	}

	// Format response
	resp := TraceValuesResponse{
		Results: map[string][]TraceRow{},
	}

	for id, tr := range df.TraceSet {
		rows := []TraceRow{}
		for i, val := range tr {
			if val != vec32.MissingDataSentinel {
				header := df.Header[i]
				rows = append(rows, TraceRow{
					CommitNumber: int64(header.Offset),
					CreatedAt:    int64(header.Timestamp),
					Val:          val,
				})
			}
		}
		resp.Results[id] = rows
	}

	// Fetch anomalies if store is available
	storeToUse := api.anomalyStore
	if preferLegacy(r) {
		storeToUse = api.chromeperfAnomalyStore
	}
	if storeToUse != nil {
		traceNames := []string{}
		for id := range df.TraceSet {
			traceNames = append(traceNames, id)
		}
		anomalyMap, err := storeToUse.GetAnomaliesInTimeRange(ctx, traceNames, beginTime, endTime)
		if err != nil {
			sklog.Errorf("Failed to fetch anomalies: %v", err)
		} else {
			resp.AnomalyMap = anomalyMap
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode response: %s", err)
	}
}
