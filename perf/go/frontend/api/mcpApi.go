package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/progress"
)

type mcpApi struct {
	dfBuilder dataframe.DataFrameBuilder
}

// NewMcpApi creates a new McpApi.
func NewMcpApi(dfBuilder dataframe.DataFrameBuilder) *mcpApi {
	return &mcpApi{
		dfBuilder: dfBuilder,
	}
}

func (m mcpApi) RegisterHandlers(router *chi.Mux) {
	router.Get("/mcp/data", m.getTraceDataHandler)
}

// getTraceDataHandler handles a request for trace data within a given time range and query.
// It parses the 'query', 'begin', and 'end' parameters from the request URL.
// The 'begin' and 'end' parameters are Unix timestamps.
// It uses the DataFrameBuilder to construct a DataFrame and writes it as a JSON response.
func (m mcpApi) getTraceDataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()

	q := r.URL.Query()
	queryString := q.Get("query")
	beginStr := q.Get("begin")
	endStr := q.Get("end")

	if queryString == "" || beginStr == "" || endStr == "" {
		httputils.ReportError(w, fmt.Errorf("missing required parameters"), "query, begin, and end are required", http.StatusBadRequest)
		return
	}

	beginUnix, err := strconv.ParseInt(beginStr, 10, 64)
	if err != nil {
		httputils.ReportError(w, err, "invalid 'begin' timestamp", http.StatusBadRequest)
		return
	}
	endUnix, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil {
		httputils.ReportError(w, err, "invalid 'end' timestamp", http.StatusBadRequest)
		return
	}

	beginTime := time.Unix(beginUnix, 0)
	endTime := time.Unix(endUnix, 0)

	parsedQuery, err := url.ParseQuery(queryString)
	if err != nil {
		httputils.ReportError(w, err, "invalid 'query' format", http.StatusBadRequest)
		return
	}
	queryObj, err := query.New(parsedQuery)
	if err != nil {
		httputils.ReportError(w, err, "invalid query", http.StatusBadRequest)
		return
	}

	prog := progress.New()

	df, err := m.dfBuilder.NewFromQueryAndRange(ctx, beginTime, endTime, queryObj, prog)
	if err != nil {
		httputils.ReportError(w, err, "Failed to build dataframe.", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(df); err != nil {
		sklog.Errorf("Failed to write JSON response: %s", err)
	}
}

var _ FrontendApi = (*mcpApi)(nil)
