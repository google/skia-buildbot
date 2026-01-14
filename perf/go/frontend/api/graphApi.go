package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/anomalies"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dfbuilder"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/tracecache"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

// graphApi provides a struct to handle api requests related to graph plots.
type graphApi struct {
	loginProvider alogin.Login
	traceCache    *tracecache.TraceCache
	dfBuilder     dataframe.DataFrameBuilder
	perfGit       perfgit.Git
	traceStore    tracestore.TraceStore
	metadataStore tracestore.MetadataStore
	shortcutStore shortcut.Store
	anomalyStore  anomalies.Store
	// progressTracker tracks long running web requests.
	progressTracker progress.Tracker
	// provides access to the ingested files.
	ingestedFS fs.FS

	// numParamSetsForQueries is the number of Tiles to look backwards over when
	// building a ParamSet that is used to present to users for then to build
	// queries.
	//
	// This number needs to be large enough to hit enough Tiles so that no query
	// parameters go missing.
	//
	// For example, let's say "test=foo" only runs once a week, but let's say
	// the incoming data arriving fills one Tile per day, then you'd need
	// numParamSetsForQueries to be at least 7, otherwise "foo" will never show
	// up as a query option in the UI for the "test" key.
	numParamSetsForQueries int

	// The length of the commit window to use when searching for data.
	queryCommitChunkSize int

	maxEmptyTiles int

	frameStartHandlerTimer metrics2.Float64SummaryMetric
	// Individual values of duration/num commits will be whole numbers but there's
	// no Int64SummaryMetric
	frameRequestDurationSeconds metrics2.Float64SummaryMetric
	frameRequestNumCommits      metrics2.Float64SummaryMetric
}

// RegisterHandlers registers the api handlers for their respective routes.
func (api graphApi) RegisterHandlers(router *chi.Mux) {
	router.Post("/_/frame/start", api.frameStartHandler)
	router.Post("/_/cid", api.cidHandler)
	router.Post("/_/details", api.detailsHandler)
	router.Post("/_/links", api.linksHandler)
	router.Post("/_/shift", api.shiftHandler)
	router.Post("/_/cidRange", api.cidRangeHandler)
}

// NewGraphApi returns a new instance of the graphApi struct.
func NewGraphApi(numParamSetsForQueries int, queryCommitChunkSize int, maxEmptyTiles int, loginProvider alogin.Login, dfBuilder dataframe.DataFrameBuilder, perfGit perfgit.Git, traceStore tracestore.TraceStore, metadataStore tracestore.MetadataStore, traceCache *tracecache.TraceCache, shortcutStore shortcut.Store, anomalyStore anomalies.Store, progressTracker progress.Tracker, ingestedFS fs.FS) graphApi {
	return graphApi{
		numParamSetsForQueries:      numParamSetsForQueries,
		queryCommitChunkSize:        queryCommitChunkSize,
		maxEmptyTiles:               maxEmptyTiles,
		loginProvider:               loginProvider,
		dfBuilder:                   dfBuilder,
		perfGit:                     perfGit,
		traceStore:                  traceStore,
		metadataStore:               metadataStore,
		traceCache:                  traceCache,
		shortcutStore:               shortcutStore,
		anomalyStore:                anomalyStore,
		progressTracker:             progressTracker,
		ingestedFS:                  ingestedFS,
		frameStartHandlerTimer:      metrics2.GetFloat64SummaryMetric("perfserver_graphApi_frameStartHandler"),
		frameRequestDurationSeconds: metrics2.GetFloat64SummaryMetric("perfserver_graphApi_frameRequestDurationSeconds"),
		frameRequestNumCommits:      metrics2.GetFloat64SummaryMetric("perfserver_graphApi_frameRequestNumCommits"),
	}
}

// frameStartHandler starts a FrameRequest running and returns the ID
// of the Go routine doing the work.
//
// Building a DataFrame can take a long time to complete, so we run the request
// in a Go routine and break the building of DataFrames into three separate
// requests:
//   - Start building the DataFrame (_/frame/start), which returns an identifier of the long
//     running request, {id}.
//   - Query the status of the running request (_/frame/status/{id}).
//   - Finally return the constructed DataFrame (_/frame/results/{id}).
func (api graphApi) frameStartHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fr := frame.NewFrameRequest()
	if err := json.NewDecoder(r.Body).Decode(fr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	auditlog.LogWithUser(r, api.loginProvider.LoggedInAs(r).String(), "query", fr)
	// Remove all empty queries.
	q := []string{}
	for _, s := range fr.Queries {
		if strings.TrimSpace(s) != "" {
			q = append(q, s)
		}
	}
	fr.Queries = q

	if len(fr.Formulas) == 0 && len(fr.Queries) == 0 && fr.Keys == "" {
		httputils.ReportError(w, fmt.Errorf("Invalid query."), "Empty queries are not allowed.", http.StatusInternalServerError)
		return
	}

	dfBuilder := api.dfBuilder
	if fr.DoNotFilterParentTraces {
		dfBuilder = dfbuilder.NewDataFrameBuilderFromTraceStore(
			api.perfGit,
			api.traceStore,
			api.traceCache,
			api.numParamSetsForQueries,
			dfbuilder.Filtering(false),
			api.queryCommitChunkSize,
			api.maxEmptyTiles,
			config.Config.Experiments.PreflightSubqueriesForExistingKeys,
			[]string{}) // Empty includedParams for frame requests
	}
	api.progressTracker.Add(fr.Progress)
	go func() {
		// Intentionally using a background context here because the calculation will go on in the background after
		// the request finishes
		ctx, span := trace.StartSpan(context.Background(), "frameStartRequest")
		defer timer.NewWithSummary("perfserver_graphapi_frameProcess", api.frameStartHandlerTimer).Stop()
		if fr.RequestType == frame.REQUEST_COMPACT {
			api.frameRequestNumCommits.Observe(float64(fr.NumCommits))
		} else {
			api.frameRequestDurationSeconds.Observe(float64(fr.End) - float64(fr.Begin))
		}
		timeoutCtx, cancel := context.WithTimeout(ctx, config.QueryMaxRunTime)
		defer cancel()
		defer span.End()
		err := frame.ProcessFrameRequest(timeoutCtx, fr, api.perfGit, dfBuilder, api.traceStore, api.metadataStore, api.shortcutStore, api.anomalyStore, config.Config.GitRepoConfig.CommitNumberRegex == "")
		if err != nil {
			fr.Progress.Error(err.Error())
		} else {
			fr.Progress.Finished()
		}
	}()

	if err := fr.Progress.JSON(w); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

// CIDHandlerResponse is the form of the response from the /_/cid/ endpoint.
type CIDHandlerResponse struct {
	// CommitSlice describes all the commits requested.
	CommitSlice []provider.Commit `json:"commitSlice"`

	// LogEntry is the full git log entry for the first commit in the
	// CommitSlice.
	LogEntry string `json:"logEntry"`
}

// cidHandler takes the POST'd list of dataframe.ColumnHeaders, and returns a
// serialized slice of cid.CommitDetails.
func (api graphApi) cidHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	cids := []types.CommitNumber{}
	if err := json.NewDecoder(r.Body).Decode(&cids); err != nil {
		httputils.ReportError(w, err, "Could not decode POST body.", http.StatusInternalServerError)
		return
	}

	commits, err := api.perfGit.CommitSliceFromCommitNumberSlice(ctx, cids)
	if err != nil {
		httputils.ReportError(w, err, "Failed to lookup all commit ids", http.StatusInternalServerError)
		return
	}
	logEntry, err := api.perfGit.LogEntry(ctx, cids[0])
	if err != nil {
		logEntry = "<<< Failed to load >>>"
		sklog.Errorf("Failed to get log entry: %s", err)
	}

	resp := CIDHandlerResponse{
		CommitSlice: commits,
		LogEntry:    logEntry,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

// CommitDetailsRequest is for deserializing incoming POST requests
// in detailsHandler.
type CommitDetailsRequest struct {
	CommitNumber types.CommitNumber `json:"cid"`
	TraceID      string             `json:"traceid"`
}

// linksHandler returns the links for a trace at a commit number.
func (api graphApi) linksHandler(w http.ResponseWriter, r *http.Request) {
	// With point specific links, we need to read the original json file.
	// Fall back to the process in detailsHandler.
	if config.Config.DataPointConfig.EnablePointSpecificLinks {
		api.detailsHandler(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	dr := &CommitDetailsRequest{}
	if err := json.NewDecoder(r.Body).Decode(dr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	// If the trace is really a calculation then don't provide any details, but
	// also don't generate an error.
	if !query.IsValid(dr.TraceID) {
		ret := format.Format{
			Version: 0, // Specifying an unacceptable version of the format causes the control to be hidden.
		}
		if err := json.NewEncoder(w).Encode(ret); err != nil {
			sklog.Errorf("writing detailsHandler error response: %s", err)
		}
		return
	}

	filename, err := api.traceStore.GetSource(ctx, dr.CommitNumber, dr.TraceID)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load details", http.StatusInternalServerError)
		return
	}

	links, err := api.metadataStore.GetMetadata(ctx, filename)
	if err != nil {
		// This is potentially a data point before Metadata table was populated.
		// Try using the details handler.
		sklog.Warningf("Error ")
		api.detailsHandler(w, r)
		return
	}

	responseObj := format.Format{
		Links: links,
	}

	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(responseObj)
	if err != nil {
		sklog.Errorf("Failed to encode response object to JSON: %v", err)
		return
	}
	if _, err := w.Write(b.Bytes()); err != nil {
		sklog.Errorf("Failed to write JSON response object: %v", err)
		return
	}
}

// detailsHandler returns commit details for the selected data point.
func (api graphApi) detailsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	includeResults := r.FormValue("results") != "false"
	dr := &CommitDetailsRequest{}
	if err := json.NewDecoder(r.Body).Decode(dr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	// If the trace is really a calculation then don't provide any details, but
	// also don't generate an error.
	if !query.IsValid(dr.TraceID) {
		ret := format.Format{
			Version: 0, // Specifying an unacceptable version of the format causes the control to be hidden.
		}
		if err := json.NewEncoder(w).Encode(ret); err != nil {
			sklog.Errorf("writing detailsHandler error response: %s", err)
		}
		return
	}

	name, err := api.traceStore.GetSource(ctx, dr.CommitNumber, dr.TraceID)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load details", http.StatusInternalServerError)
		return
	}

	reader, err := api.ingestedFS.Open(name)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get reader for source file location", http.StatusInternalServerError)
		return
	}
	defer util.Close(reader)
	formattedData, err := format.Parse(reader)
	if err != nil {
		// This is because the file being read is in the legacy format that does not provide links.
		// We can ignore this error since anyway there are no links to return in the file.
		formattedData = format.Format{}
	} else {
		pointLinks := formattedData.GetLinksForMeasurement(dr.TraceID)

		if !includeResults {
			formattedData.Results = nil
		}

		formattedData.Links = pointLinks
	}

	var buff bytes.Buffer
	jsonEncoder := json.NewEncoder(&buff)
	jsonEncoder.SetIndent("", "  ")
	err = jsonEncoder.Encode(formattedData)
	if err != nil {
		httputils.ReportError(w, err, "Failed to re-encode JSON source file", http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(buff.Bytes()); err != nil {
		sklog.Errorf("Failed to write JSON source file: %s", err)
	}
}

// ShiftRequest is a request to find the timestamps of a range of commits.
type ShiftRequest struct {
	// Begin is the commit number at the beginning of the range.
	Begin types.CommitNumber `json:"begin"`

	// End is the commit number at the end of the range.
	End types.CommitNumber `json:"end"`
}

// ShiftResponse are the timestamps from a ShiftRequest.
type ShiftResponse struct {
	Begin int64 `json:"begin"` // In seconds from the epoch.
	End   int64 `json:"end"`   // In seconds from the epoch.
}

// shiftHandler computes a new begin and end timestamp for a dataframe given
// the current begin and end offsets.
func (api graphApi) shiftHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	var sr ShiftRequest
	if err := json.NewDecoder(r.Body).Decode(&sr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	sklog.Infof("ShiftRequest: %#v", &sr)

	var begin time.Time
	var end time.Time
	var err error

	commit, err := api.perfGit.CommitFromCommitNumber(ctx, sr.Begin)
	if err != nil {
		httputils.ReportError(w, err, "Failed to look up begin commit.", http.StatusBadRequest)
		return
	}
	begin = time.Unix(commit.Timestamp, 0)

	commit, err = api.perfGit.CommitFromCommitNumber(ctx, sr.End)
	if err != nil {
		// If sr.End isn't a valid offset then just use the most recent commit.
		lastCommitNumber, err := api.perfGit.CommitNumberFromTime(ctx, time.Time{})
		if err != nil {
			httputils.ReportError(w, err, "Failed to look up last commit.", http.StatusBadRequest)
			return
		}
		commit, err = api.perfGit.CommitFromCommitNumber(ctx, lastCommitNumber)
		if err != nil {
			httputils.ReportError(w, err, "Failed to look up end commit.", http.StatusBadRequest)
			return
		}
	}
	end = time.Unix(commit.Timestamp, 0)

	resp := ShiftResponse{
		Begin: begin.Unix(),
		End:   end.Unix(),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to write JSON response: %s", err)
	}
}

// RangeRequest is used in cidRangeHandler and is used to query for a range of
// cid.CommitIDs that include the range between [begin, end) and include the
// explicit CommitID of "Source, Offset".
type RangeRequest struct {
	Offset types.CommitNumber `json:"offset"`
	Begin  int64              `json:"begin"`
	End    int64              `json:"end"`
}

// cidRangeHandler accepts a POST'd JSON serialized RangeRequest
// and returns a serialized JSON slice of cid.CommitDetails.
func (api graphApi) cidRangeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	var rr RangeRequest
	if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	resp, err := api.perfGit.CommitSliceFromTimeRange(ctx, time.Unix(rr.Begin, 0), time.Unix(rr.End, 0))
	if err != nil {
		httputils.ReportError(w, err, "Failed to look up commits", http.StatusInternalServerError)
		return
	}

	if rr.Offset != types.BadCommitNumber {
		details, err := api.perfGit.CommitFromCommitNumber(ctx, rr.Offset)
		if err != nil {
			httputils.ReportError(w, err, "Failed to look up commit", http.StatusInternalServerError)
			return
		}
		resp = append(resp, details)
	}

	// Filter if we have a restricted set of branches.
	ret := []provider.Commit{}
	if len(config.Config.IngestionConfig.Branches) != 0 {
		for _, details := range resp {
			for _, branch := range config.Config.IngestionConfig.Branches {
				if strings.HasSuffix(details.Subject, branch) {
					ret = append(ret, details)
					continue
				}
			}
		}
	} else {
		ret = resp
	}

	if err := json.NewEncoder(w).Encode(ret); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}
