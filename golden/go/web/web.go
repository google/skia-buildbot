package web

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	ttlcache "github.com/patrickmn/go-cache"
	"go.opencensus.io/trace"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/sqlutil"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/search"
	search_query "go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/validation"
	"go.skia.org/infra/golden/go/web/frontend"
)

const (
	// pageSize is the default page size used for pagination.
	pageSize = 20

	// maxPageSize is the maximum page size used for pagination.
	maxPageSize = 100

	// These params limit how anonymous (not logged-in) users can hit various endpoints.
	// We have two buckets of requests - cheap and expensive. Expensive stuff hits a database
	// or similar, where as cheap stuff is cached. These limits are shared by *all* endpoints
	// in a given bucket. See skbug.com/9476 for more.
	maxAnonQPSExpensive   = rate.Limit(0.01)
	maxAnonBurstExpensive = 50
	maxAnonQPSCheap       = rate.Limit(5.0)
	maxAnonBurstCheap     = 50
	// Special settings for RPCs serving the gerrit plugin. See skbug.com/10768 for more.
	maxAnonQPSGerritPlugin   = rate.Limit(200.0)
	maxAnonBurstGerritPlugin = 1000

	changelistSummaryCacheSize = 10000

	// RPCCallCounterMetric is the metric that should be used when counting how many times a given
	// RPC route is called from clients.
	RPCCallCounterMetric = "gold_rpc_call_counter"

	baselineCachePrimaryBranchEntryTTL   = 10 * time.Second
	baselineCacheSecondaryBranchEntryTTL = time.Minute
	baselineCacheCleanupInterval         = 10 * time.Minute
)

type validateFields int

const (
	// FullFrontEnd means all fields should be set
	FullFrontEnd validateFields = iota
	// BaselineSubset means just the fields needed for BaselineV2Response Server should be set.
	BaselineSubset
)

// HandlersConfig holds the environment needed by the various http handler functions.
type HandlersConfig struct {
	DB                        *pgxpool.Pool
	GCSClient                 storage.GCSClient
	IgnoreStore               ignore.Store
	ReviewSystems             []clstore.ReviewSystem
	Search2API                search.API
	WindowSize                int
	GroupingParamKeysByCorpus map[string][]string
}

// Handlers represents all the handlers (e.g. JSON endpoints) of Gold.
// It should be created by clients using NewHandlers.
type Handlers struct {
	HandlersConfig

	anonymousExpensiveQuota *rate.Limiter
	anonymousCheapQuota     *rate.Limiter
	anonymousGerritQuota    *rate.Limiter

	clSummaryCache *lru.Cache
	baselineCache  *ttlcache.Cache

	statusCache      frontend.GUIStatus
	statusCacheMutex sync.RWMutex

	ignoredTracesCache      []ignoredTrace
	ignoredTracesCacheMutex sync.RWMutex

	knownHashesMutex sync.RWMutex
	knownHashesCache string

	// These can be set for unit tests to simplify the testing.
	testingAuthAs string
}

// NewHandlers returns a new instance of Handlers.
func NewHandlers(conf HandlersConfig, val validateFields) (*Handlers, error) {
	// These fields are required by all types.
	if conf.DB == nil {
		return nil, skerr.Fmt("Baseliner cannot be nil")
	}
	if conf.GCSClient == nil {
		return nil, skerr.Fmt("GCSClient cannot be nil")
	}

	if val == FullFrontEnd {
		if conf.IgnoreStore == nil {
			return nil, skerr.Fmt("IgnoreStore cannot be nil")
		}
		if conf.Search2API == nil {
			return nil, skerr.Fmt("Search2API cannot be nil")
		}
	}

	clcache, err := lru.New(changelistSummaryCacheSize)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &Handlers{
		HandlersConfig:          conf,
		anonymousExpensiveQuota: rate.NewLimiter(maxAnonQPSExpensive, maxAnonBurstExpensive),
		anonymousCheapQuota:     rate.NewLimiter(maxAnonQPSCheap, maxAnonBurstCheap),
		anonymousGerritQuota:    rate.NewLimiter(maxAnonQPSGerritPlugin, maxAnonBurstGerritPlugin),
		clSummaryCache:          clcache,
		baselineCache:           ttlcache.New(baselineCachePrimaryBranchEntryTTL, baselineCacheCleanupInterval),
		testingAuthAs:           "", // Just to be explicit that we do *not* bypass Auth.
	}, nil
}

// limitForAnonUsers blocks using the configured rate.Limiter for expensive queries.
func (wh *Handlers) limitForAnonUsers(r *http.Request) error {
	if login.LoggedInAs(r) != "" {
		return nil
	}
	return wh.anonymousExpensiveQuota.Wait(r.Context())
}

// cheapLimitForAnonUsers blocks using the configured rate.Limiter for cheap queries.
func (wh *Handlers) cheapLimitForAnonUsers(r *http.Request) error {
	if login.LoggedInAs(r) != "" {
		return nil
	}
	return wh.anonymousCheapQuota.Wait(r.Context())
}

// cheapLimitForGerritPlugin blocks using the configured rate.Limiter for queries for the
// Gerrit Plugin.
func (wh *Handlers) cheapLimitForGerritPlugin(r *http.Request) error {
	if login.LoggedInAs(r) != "" {
		return nil
	}
	return wh.anonymousGerritQuota.Wait(r.Context())
}

// ByBlameHandler takes the response from the SQL backend's GetBlamesForUntriagedDigests and
// converts it into the same format that the legacy version (v1) produced.
func (wh *Handlers) ByBlameHandler(w http.ResponseWriter, r *http.Request) {
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	ctx, span := trace.StartSpan(r.Context(), "web_ByBlameHandler", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()

	// Extract the corpus from the query parameters.
	corpus := ""
	if v := r.FormValue("query"); v != "" {
		if qp, err := url.ParseQuery(v); err != nil {
			httputils.ReportError(w, err, "invalid input", http.StatusBadRequest)
			return
		} else if corpus = qp.Get(types.CorpusField); corpus == "" {
			// If no corpus specified report an error.
			http.Error(w, "did not receive value for corpus", http.StatusBadRequest)
			return
		}
	} else {
		// If no corpus specified report an error.
		http.Error(w, "did not receive value for search query", http.StatusBadRequest)
		return
	}
	summary, err := wh.Search2API.GetBlamesForUntriagedDigests(ctx, corpus)
	if err != nil {
		httputils.ReportError(w, err, "Could not compute blames", http.StatusInternalServerError)
		return
	}
	result := frontend.ByBlameResponse{}
	for _, sr := range summary.Ranges {
		entry := frontend.ByBlameEntry{
			GroupID:  sr.CommitRange,
			NDigests: sr.TotalUntriagedDigests,
			NTests:   len(sr.AffectedGroupings),
			Commits:  sr.Commits,
		}
		var groupings []frontend.TestRollup
		for _, gr := range sr.AffectedGroupings {
			groupings = append(groupings, frontend.TestRollup{
				Grouping:     gr.Grouping,
				Num:          gr.UntriagedDigests,
				SampleDigest: gr.SampleDigest,
			})
		}
		entry.AffectedTests = groupings
		result.Data = append(result.Data, entry)
	}
	sendJSONResponse(w, result)
}

// ChangelistsHandler returns the list of code_review.Changelists that have
// uploaded results to Gold (via TryJobs).
func (wh *Handlers) ChangelistsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_ChangelistsHandler", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	values := r.URL.Query()
	offset, size, err := httputils.PaginationParams(values, 0, pageSize, maxPageSize)
	if err != nil {
		httputils.ReportError(w, err, "Invalid pagination params.", http.StatusInternalServerError)
		return
	}

	_, activeOnly := values["active"]
	cls, pagination, err := wh.getIngestedChangelists2(ctx, offset, size, activeOnly)

	if err != nil {
		httputils.ReportError(w, err, "Retrieving changelists results failed.", http.StatusInternalServerError)
		return
	}

	response := frontend.ChangelistsResponse{
		Changelists:        cls,
		ResponsePagination: pagination,
	}

	sendJSONResponse(w, response)
}

func (wh *Handlers) getIngestedChangelists2(ctx context.Context, offset, size int, activeOnly bool) ([]frontend.Changelist, httputils.ResponsePagination, error) {
	ctx, span := trace.StartSpan(ctx, "web_getIngestedChangelists2")
	defer span.End()

	statement := `SELECT changelist_id, system, status, owner_email, subject, last_ingested_data
FROM Changelists AS OF SYSTEM TIME '-0.1s'`
	if activeOnly {
		statement += " WHERE status = 'open'"
	} else {
		// This lets us use the same statusIngestedIndex
		statement += " WHERE status = ANY('open', 'landed', 'abandoned')"
	}
	statement += ` ORDER BY last_ingested_data DESC OFFSET $1 LIMIT $2`
	rows, err := wh.DB.Query(ctx, statement, offset, size)
	if err != nil {
		return nil, httputils.ResponsePagination{}, skerr.Wrap(err)
	}
	defer rows.Close()
	var rv []frontend.Changelist
	for rows.Next() {
		var cl frontend.Changelist
		var qCLID string
		if err := rows.Scan(&qCLID, &cl.System, &cl.Status, &cl.Owner, &cl.Subject, &cl.Updated); err != nil {
			return nil, httputils.ResponsePagination{}, skerr.Wrap(err)
		}
		cl.Updated = cl.Updated.UTC()
		cl.SystemID = sql.Unqualify(qCLID)
		urlTempl := ""
		for _, system := range wh.ReviewSystems {
			if system.ID == cl.System {
				urlTempl = system.URLTemplate
				break
			}
		}
		cl.URL = strings.Replace(urlTempl, "%s", cl.SystemID, 1)
		rv = append(rv, cl)
	}

	pagination := httputils.ResponsePagination{
		Offset: offset,
		Size:   size,
		Total:  clstore.CountMany, // exact count not important for most day-to-day work.
	}
	return rv, pagination, nil
}

// A list of CI systems we support. So far, the mapping of task ID to link is project agnostic. If
// that stops being the case, then we'll need to supply this mapping on a per-instance basis.
var cisTemplates = map[string]string{
	"cirrus":               "https://cirrus-ci.com/task/%s",
	"buildbucket":          "https://cr-buildbucket.appspot.com/build/%s",
	"buildbucket-internal": "https://cr-buildbucket.appspot.com/build/%s",
}

// PatchsetsAndTryjobsForCL2 returns a summary of the data we have collected
// for a given Changelist, specifically any TryJobs that have uploaded data
// to Gold belonging to various patchsets in it.
func (wh *Handlers) PatchsetsAndTryjobsForCL2(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_PatchsetsAndTryjobsForCL2", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	clID, ok := mux.Vars(r)["id"]
	if !ok {
		http.Error(w, "Must specify 'id' of Changelist.", http.StatusBadRequest)
		return
	}
	crs, ok := mux.Vars(r)["system"]
	if !ok {
		http.Error(w, "Must specify 'system' of Changelist.", http.StatusBadRequest)
		return
	}
	rv, err := wh.getPatchsetsAndTryjobs(ctx, crs, clID)
	if err != nil {
		httputils.ReportError(w, err, "could not retrieve data for the specified CL.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, rv)
}

// getPatchsetsAndTryjobs returns a summary of the patchsets and tryjobs that belong to a given
// CL.
func (wh *Handlers) getPatchsetsAndTryjobs(ctx context.Context, crs, clID string) (frontend.ChangelistSummary, error) {
	ctx, span := trace.StartSpan(ctx, "getPatchsetsAndTryjobs")
	defer span.End()

	system, ok := wh.getCodeReviewSystem(crs)
	if !ok {
		return frontend.ChangelistSummary{}, skerr.Fmt("Invalid Code Review System %q", crs)
	}

	qCLID := sql.Qualify(crs, clID)
	row := wh.DB.QueryRow(ctx, `SELECT status, owner_email, subject, last_ingested_data FROM Changelists
WHERE changelist_id = $1`, qCLID)
	var cl frontend.Changelist
	if err := row.Scan(&cl.Status, &cl.Owner, &cl.Subject, &cl.Updated); err != nil {
		return frontend.ChangelistSummary{}, skerr.Wrapf(err, "checking if CL %q exists", qCLID)
	}
	cl.Updated = cl.Updated.UTC()
	cl.SystemID = clID
	cl.System = crs
	cl.URL = strings.Replace(system.URLTemplate, "%s", cl.SystemID, 1)
	rv := frontend.ChangelistSummary{CL: cl}

	const statement = `SELECT Patchsets.patchset_id, Patchsets.ps_order,
tryjob_id, display_name, Tryjobs.last_ingested_data, Tryjobs.system FROM
Tryjobs JOIN Patchsets ON Tryjobs.patchset_id = Patchsets.patchset_id
WHERE Tryjobs.changelist_id = $1
ORDER BY Patchsets.patchset_id
`
	rows, err := wh.DB.Query(ctx, statement, qCLID)
	if err != nil {
		return frontend.ChangelistSummary{}, skerr.Wrap(err)
	}
	defer rows.Close()
	var patchsets []*frontend.Patchset
	var currentPS *frontend.Patchset
	for rows.Next() {
		var psID string
		var order int
		var tj frontend.TryJob
		if err := rows.Scan(&psID, &order, &tj.SystemID, &tj.DisplayName, &tj.Updated, &tj.System); err != nil {
			return frontend.ChangelistSummary{}, skerr.Wrap(err)
		}
		tj.Updated = tj.Updated.UTC()
		urlTempl, ok := cisTemplates[tj.System]
		if !ok {
			return frontend.ChangelistSummary{}, skerr.Fmt("Unrecognized CIS system: %q", tj.System)
		}
		tj.URL = strings.Replace(urlTempl, "%s", sql.Unqualify(tj.SystemID), 1)
		if currentPS == nil || currentPS.SystemID != psID {
			currentPS = &frontend.Patchset{
				SystemID: psID,
				Order:    order,
			}
			patchsets = append(patchsets, currentPS)
		}
		currentPS.TryJobs = append(currentPS.TryJobs, tj)
	}

	rv.Patchsets = make([]frontend.Patchset, 0, len(patchsets)) // ensure non-nil slice
	for _, ps := range patchsets {
		rv.Patchsets = append(rv.Patchsets, *ps)
	}
	rv.NumTotalPatchsets = len(rv.Patchsets)

	sort.Slice(rv.Patchsets, func(i, j int) bool {
		return rv.Patchsets[i].Order > rv.Patchsets[j].Order
	})
	return rv, nil
}

// SearchHandler searches the data in the new SQL backend. It times out after 3 minutes, to prevent
// outstanding requests from growing unbounded.
func (wh *Handlers) SearchHandler(w http.ResponseWriter, r *http.Request) {
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	q, ok := parseSearchQuery(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	ctx, span := trace.StartSpan(ctx, "web_SearchHandler", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()

	searchResponse, err := wh.Search2API.Search(ctx, q)
	if err != nil {
		httputils.ReportError(w, err, "Search for digests failed in the SQL backend.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, searchResponse)
}

// parseSearchQuery extracts the search query from request.
func parseSearchQuery(w http.ResponseWriter, r *http.Request) (*search_query.Search, bool) {
	q := search_query.Search{Limit: 50}
	if err := search_query.ParseSearch(r, &q); err != nil {
		httputils.ReportError(w, err, "Search for digests failed.", http.StatusInternalServerError)
		return nil, false
	}
	// Currently, the frontend includes the corpus as a right trace value. That's really a no-op
	// because that info (and the test name) are specified in the grouping. As such, we delete
	// those so they don't cause us to go into a slow path accounting for keys when we do not
	// need to.
	// TODO(kjlubick) Make the frontend not supply these.
	delete(q.RightTraceValues, types.CorpusField)
	delete(q.RightTraceValues, types.PrimaryKeyField)
	return &q, true
}

// DetailsHandler returns the details about a single digest.
func (wh *Handlers) DetailsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_DetailsHandler", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	req := frontend.DetailsRequest{}
	if err := parseJSON(r, &req); err != nil {
		httputils.ReportError(w, err, "Failed to parse JSON request.", http.StatusBadRequest)
		return
	}
	sklog.Infof("Details request: %#v", req)

	if len(req.Grouping) == 0 {
		http.Error(w, "Grouping cannot be empty.", http.StatusBadRequest)
		return
	}
	if !validation.IsValidDigest(string(req.Digest)) {
		http.Error(w, "Invalid digest.", http.StatusBadRequest)
		return
	}
	if req.CodeReviewSystem != "" && req.ChangelistID != "" {
		if _, ok := wh.getCodeReviewSystem(req.CodeReviewSystem); !ok {
			http.Error(w, "Invalid code review system.", http.StatusBadRequest)
			return
		}
	}

	ret, err := wh.Search2API.GetDigestDetails(ctx, req.Grouping, types.Digest(req.Digest), req.ChangelistID, req.CodeReviewSystem)
	if err != nil {
		httputils.ReportError(w, err, "Unable to get digest details.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, ret)
}

// GroupingForTestHandler looks up and returns the grouping corresponding to a test. This RPC acts
// as a bridge for clients that do not have access to grouping information (only Gold's details
// page at the time of writing.)
//
// TODO(lovisolo): Delete this RPC once the details page, and all links to it, include the
//                 necessary grouping information.
func (wh *Handlers) GroupingForTestHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_GroupingForTestHandler", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()

	req := frontend.GroupingForTestRequest{}
	if err := parseJSON(r, &req); err != nil {
		httputils.ReportError(w, err, "Failed to parse JSON request.", http.StatusBadRequest)
		return
	}

	if req.TestName == "" {
		http.Error(w, "Test name cannot be empty.", http.StatusBadRequest)
		return
	}

	grouping, err := wh.getGroupingForTest(ctx, req.TestName)
	if err != nil {
		if skerr.Unwrap(err) == pgx.ErrNoRows {
			http.Error(w, "Test not found.", http.StatusNotFound)
			return
		}
		httputils.ReportError(w, err, "Unable to get grouping for test.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, frontend.GroupingForTestResponse{Grouping: grouping})
}

// getGroupingForTest acts as a bridge for RPCs that only take in a test name, when they should
// be taking in a grouping. It looks up the grouping by test name and returns it.
// TODO(kjlubick) Migrate all RPCs and remove this function.
func (wh *Handlers) getGroupingForTest(ctx context.Context, testName string) (paramtools.Params, error) {
	ctx, span := trace.StartSpan(ctx, "getGroupingForTest")
	defer span.End()

	const statement = `SELECT keys FROM Groupings WHERE keys->'name' = $1 LIMIT 1`
	// Need to wrap testName with quotes to make it "valid JSON", so we can use the inverted index
	// on keys.
	row := wh.DB.QueryRow(ctx, statement, `"`+testName+`"`)
	var ps paramtools.Params
	if err := row.Scan(&ps); err != nil {
		return nil, skerr.Wrapf(err, "looking up grouping for test name %q", testName)
	}
	return ps, nil
}

// DiffHandler compares two digests and returns that information along with triage data.
func (wh *Handlers) DiffHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_DiffHandler", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()

	req := frontend.DiffRequest{}
	if err := parseJSON(r, &req); err != nil {
		httputils.ReportError(w, err, "Failed to parse JSON request.", http.StatusBadRequest)
		return
	}
	sklog.Infof("Diff request: %#v", req)

	if len(req.Grouping) == 0 {
		http.Error(w, "Grouping cannot be empty.", http.StatusBadRequest)
		return
	}
	if !validation.IsValidDigest(string(req.LeftDigest)) {
		http.Error(w, "Invalid left digest.", http.StatusBadRequest)
		return
	}
	if !validation.IsValidDigest(string(req.RightDigest)) {
		http.Error(w, "Invalid right digest.", http.StatusBadRequest)
		return
	}
	if req.CodeReviewSystem != "" && req.ChangelistID != "" {
		if _, ok := wh.getCodeReviewSystem(req.CodeReviewSystem); !ok {
			http.Error(w, "Invalid code review system.", http.StatusBadRequest)
			return
		}
	}

	ret, err := wh.Search2API.GetDigestsDiff(ctx, req.Grouping, req.LeftDigest, req.RightDigest, req.ChangelistID, req.CodeReviewSystem)
	if err != nil {
		httputils.ReportError(w, err, "Unable to get diff for digests.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, ret)
}

// ListIgnoreRules2 returns the current ignore rules in JSON format and the counts of
// how many traces they affect.
func (wh *Handlers) ListIgnoreRules2(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_ListIgnoreRules2", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()

	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	ignores, err := wh.getIgnores2(ctx)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve ignore rules, there may be none.", http.StatusInternalServerError)
		return
	}

	response := frontend.IgnoresResponse{
		Rules: ignores,
	}

	sendJSONResponse(w, response)
}

// getIgnores2 fetches all ignore rules and converts them into the frontend format. It will add the
// trace counts for each rule.
func (wh *Handlers) getIgnores2(ctx context.Context) ([]frontend.IgnoreRule, error) {
	ctx, span := trace.StartSpan(ctx, "getIgnores2")
	defer span.End()
	rules, err := wh.IgnoreStore.List(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching ignores from store")
	}

	ret := make([]frontend.IgnoreRule, 0, len(rules))
	for _, r := range rules {
		fr, err := frontend.ConvertIgnoreRule(r)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		ret = append(ret, fr)
	}
	// addIgnoreCounts updates the values of ret directly
	if err := wh.addIgnoreCounts2(ctx, ret); err != nil {
		return nil, skerr.Wrapf(err, "adding ignore counts to %d rules", len(ret))
	}
	return ret, nil
}

// addIgnoreCounts2 fetches all ignored traces from the SQL DB and then goes through all the ignore
// rules and figures out which rules applied to each of those traces. This allows us to count how
// many traces each rule affects and how many are exclusively impacted by a given rule.
func (wh *Handlers) addIgnoreCounts2(ctx context.Context, rules []frontend.IgnoreRule) error {
	ctx, span := trace.StartSpan(ctx, "addIgnoreCounts2")
	defer span.End()

	type counts struct {
		Count                   int
		UntriagedCount          int
		ExclusiveCount          int
		ExclusiveUntriagedCount int
	}

	ruleCounts := make([]counts, len(rules))
	wh.ignoredTracesCacheMutex.RLock()
	defer wh.ignoredTracesCacheMutex.RUnlock()
	for _, tr := range wh.ignoredTracesCache {
		idxMatched, untIdxMatched := -1, -1
		numMatched, untMatched := 0, 0
		for i, r := range rules {
			if paramtools.ParamSet(r.ParsedQuery).MatchesParams(tr.Keys) {
				numMatched++
				ruleCounts[i].Count++
				idxMatched = i

				// Check to see if the digest is untriaged at head
				if tr.Label == expectations.Untriaged {
					ruleCounts[i].UntriagedCount++
					untMatched++
					untIdxMatched = i
				}
			}
		}
		// Check for any exclusive matches
		if numMatched == 1 {
			ruleCounts[idxMatched].ExclusiveCount++
		}
		if untMatched == 1 {
			ruleCounts[untIdxMatched].ExclusiveUntriagedCount++
		}
	}
	for i := range rules {
		(&rules[i]).Count += ruleCounts[i].Count
		(&rules[i]).UntriagedCount += ruleCounts[i].UntriagedCount
		(&rules[i]).ExclusiveCount += ruleCounts[i].ExclusiveCount
		(&rules[i]).ExclusiveUntriagedCount += ruleCounts[i].ExclusiveUntriagedCount
	}
	return nil
}

// UpdateIgnoreRule updates an existing ignores rule.
func (wh *Handlers) UpdateIgnoreRule(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_UpdateIgnoreRule", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	user := wh.loggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to update an ignore rule.", http.StatusUnauthorized)
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		http.Error(w, "ID must be non-empty.", http.StatusBadRequest)
		return
	}
	expiresInterval, irb, err := getValidatedIgnoreRule(r)
	if err != nil {
		httputils.ReportError(w, err, "invalid ignore rule input", http.StatusBadRequest)
		return
	}
	ts := now.Now(ctx)
	ignoreRule := ignore.NewRule(user, ts.Add(expiresInterval), irb.Filter, irb.Note)
	ignoreRule.ID = id
	if err := wh.IgnoreStore.Update(ctx, ignoreRule); err != nil {
		httputils.ReportError(w, err, "Unable to update ignore rule", http.StatusInternalServerError)
		return
	}

	sklog.Infof("Successfully updated ignore with id %s", id)
	sendJSONResponse(w, map[string]string{"updated": "true"})
}

// getValidatedIgnoreRule parses the JSON from the given request into an IgnoreRuleBody. As a
// convenience, the duration as a time.Duration is returned.
func getValidatedIgnoreRule(r *http.Request) (time.Duration, frontend.IgnoreRuleBody, error) {
	irb := frontend.IgnoreRuleBody{}
	if err := parseJSON(r, &irb); err != nil {
		return 0, irb, skerr.Wrapf(err, "reading request JSON")
	}
	if irb.Filter == "" {
		return 0, irb, skerr.Fmt("must supply a filter")
	}
	// If a user accidentally includes a huge amount of text, we'd like to catch that here.
	if len(irb.Filter) >= 10*1024 {
		return 0, irb, skerr.Fmt("Filter must be < 10 KB")
	}
	if len(irb.Note) >= 1024 {
		return 0, irb, skerr.Fmt("Note must be < 1 KB")
	}
	d, err := human.ParseDuration(irb.Duration)
	if err != nil {
		return 0, irb, skerr.Wrapf(err, "invalid duration")
	}
	return d, irb, nil
}

// DeleteIgnoreRule deletes an existing ignores rule.
func (wh *Handlers) DeleteIgnoreRule(w http.ResponseWriter, r *http.Request) {
	user := wh.loggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to delete an ignore rule", http.StatusUnauthorized)
		return
	}
	ctx, span := trace.StartSpan(r.Context(), "web_DeleteIgnoreRule", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	id := mux.Vars(r)["id"]
	if id == "" {
		http.Error(w, "ID must be non-empty.", http.StatusBadRequest)
		return
	}

	if err := wh.IgnoreStore.Delete(ctx, id); err != nil {
		httputils.ReportError(w, err, "Unable to delete ignore rule", http.StatusInternalServerError)
		return
	}
	sklog.Infof("Successfully deleted ignore with id %s", id)
	sendJSONResponse(w, map[string]string{"deleted": "true"})
}

// AddIgnoreRule is for adding a new ignore rule.
func (wh *Handlers) AddIgnoreRule(w http.ResponseWriter, r *http.Request) {
	user := wh.loggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to add an ignore rule", http.StatusUnauthorized)
		return
	}
	ctx, span := trace.StartSpan(r.Context(), "web_AddIgnoreRule", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()

	expiresInterval, irb, err := getValidatedIgnoreRule(r)
	if err != nil {
		httputils.ReportError(w, err, "invalid ignore rule input", http.StatusBadRequest)
		return
	}
	ts := now.Now(ctx)
	ignoreRule := ignore.NewRule(user, ts.Add(expiresInterval), irb.Filter, irb.Note)
	if err := wh.IgnoreStore.Create(ctx, ignoreRule); err != nil {
		httputils.ReportError(w, err, "Failed to create ignore rule", http.StatusInternalServerError)
		return
	}

	sklog.Infof("Successfully added ignore from %s", user)
	sendJSONResponse(w, map[string]string{"added": "true"})
}

// TriageHandlerV2 handles a request to change the triage status of one or more
// digests of one test.
//
// It accepts a POST'd JSON serialization of TriageRequest and updates
// the expectations.
// TODO(kjlubick) In V3, this should take groupings, not test names. Additionally, to avoid race
//   conditions where users triage the same thing at the same time, the request should include
//   before and after. Finally, to avoid confusion on CLs, we should fail to apply changes
//   on closed CLs (skbug.com/12122)
func (wh *Handlers) TriageHandlerV2(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_TriageHandlerV2", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	user := login.LoggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to triage.", http.StatusUnauthorized)
		return
	}

	req := frontend.TriageRequestV2{}
	if err := parseJSON(r, &req); err != nil {
		httputils.ReportError(w, err, "Failed to parse JSON request.", http.StatusBadRequest)
		return
	}
	sklog.Infof("Triage v2 request: %#v", req)

	if err := wh.triage2(ctx, user, req); err != nil {
		httputils.ReportError(w, err, "Could not triage", http.StatusInternalServerError)
		return
	}
	// Nothing to return, so just set 200
	w.WriteHeader(http.StatusOK)
}

func (wh *Handlers) triage2(ctx context.Context, userID string, req frontend.TriageRequestV2) error {
	ctx, span := trace.StartSpan(ctx, "triage2")
	defer span.End()
	branch := ""
	if req.ChangelistID != "" && req.CodeReviewSystem != "" {
		branch = sql.Qualify(req.CodeReviewSystem, req.ChangelistID)
	}
	// If set, use the image matching algorithm's name as the author of this change.
	if req.ImageMatchingAlgorithm != "" {
		userID = req.ImageMatchingAlgorithm
	}

	deltas, err := wh.convertToDeltas(ctx, req)
	if err != nil {
		return skerr.Wrapf(err, "getting groupings")
	}
	if len(deltas) == 0 {
		return nil
	}
	span.AddAttributes(trace.Int64Attribute("num_changes", int64(len(deltas))))

	err = crdbpgx.ExecuteTx(ctx, wh.DB, pgx.TxOptions{}, func(tx pgx.Tx) error {
		newRecordID, err := writeRecord(ctx, tx, userID, len(deltas), branch)
		if err != nil {
			return err
		}
		err = fillPreviousLabel(ctx, tx, deltas, newRecordID)
		if err != nil {
			return err
		}
		err = writeDeltas(ctx, tx, deltas)
		if err != nil {
			return err
		}
		if branch == "" {
			return applyDeltasToPrimary(ctx, tx, deltas)
		}
		return applyDeltasToBranch(ctx, tx, deltas, branch)
	})
	if err != nil {
		return skerr.Wrapf(err, "writing %d expectations from %s to branch %q", len(deltas), userID, branch)
	}
	return nil
}

// convertToDeltas converts in triage request (a map) into a slice of deltas. These deltas are
// partially filled out, with only the
func (wh *Handlers) convertToDeltas(ctx context.Context, req frontend.TriageRequestV2) ([]schema.ExpectationDeltaRow, error) {
	rv := make([]schema.ExpectationDeltaRow, 0, len(req.TestDigestStatus))
	for test, digests := range req.TestDigestStatus {
		for d, label := range digests {
			if label == "" {
				// Empty string means the frontend didn't have a closest digest to use when making a
				// "bulk triage to the closest digest" request. It's easier to catch this on the
				// server side than make the JS check for empty string and mutate the POST body.
				continue
			}
			if !expectations.ValidLabel(label) {
				return nil, skerr.Fmt("invalid label %q in triage request", label)
			}
			labelAfter := schema.FromExpectationLabel(label)
			grouping, err := wh.getGroupingForTest(ctx, string(test))
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			_, groupingID := sql.SerializeMap(grouping)
			digestBytes, err := sql.DigestToBytes(d)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			rv = append(rv, schema.ExpectationDeltaRow{
				GroupingID: groupingID,
				Digest:     digestBytes,
				LabelAfter: labelAfter,
			})
		}
	}
	return rv, nil
}

// fillPreviousLabel looks up all the expectations for the partially filled-out deltas passed in
// and updates those in-place. It only pulls labels from the primary branch, as this is not meant
// for long term use (see notes for getting to V3 triage).
func fillPreviousLabel(ctx context.Context, tx pgx.Tx, deltas []schema.ExpectationDeltaRow, newRecordID uuid.UUID) error {
	ctx, span := trace.StartSpan(ctx, "fillPreviousLabel")
	defer span.End()
	type expectationKey struct {
		groupingID schema.MD5Hash
		digest     schema.MD5Hash
	}
	toUpdate := map[expectationKey]*schema.ExpectationDeltaRow{}
	for i := range deltas {
		deltas[i].ExpectationRecordID = newRecordID
		deltas[i].LabelBefore = schema.LabelUntriaged
		toUpdate[expectationKey{
			groupingID: sql.AsMD5Hash(deltas[i].GroupingID),
			digest:     sql.AsMD5Hash(deltas[i].Digest),
		}] = &deltas[i]
	}

	statement := `SELECT grouping_id, digest, label FROM Expectations WHERE `
	// We should be safe from injection attacks because we are hex encoding known valid byte arrays.
	// I couldn't find a better way to match multiple composite keys using our usual techniques
	// involving placeholders.
	for i, d := range deltas {
		if i != 0 {
			statement += " OR "
		}
		statement += fmt.Sprintf(`(grouping_id = x'%x' AND digest = x'%x')`, d.GroupingID, d.Digest)
	}
	rows, err := tx.Query(ctx, statement)
	if err != nil {
		return err // don't wrap, could be retried
	}
	defer rows.Close()
	for rows.Next() {
		var gID schema.GroupingID
		var d schema.DigestBytes
		var label schema.ExpectationLabel
		if err := rows.Scan(&gID, &d, &label); err != nil {
			return skerr.Wrap(err) // probably not retryable
		}
		ek := expectationKey{
			groupingID: sql.AsMD5Hash(gID),
			digest:     sql.AsMD5Hash(d),
		}
		row := toUpdate[ek]
		if row == nil {
			sklog.Warningf("Unmatched row with grouping %x and digest %x", gID, d)
			continue // should never happen
		}
		row.LabelBefore = label
	}
	return nil
}

// TriageHandlerV3 handles a request to change the triage status of one or more digests.
func (wh *Handlers) TriageHandlerV3(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_TriageHandlerV3", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	user := login.LoggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to triage.", http.StatusUnauthorized)
		return
	}

	req := frontend.TriageRequestV3{}
	if err := parseJSON(r, &req); err != nil {
		httputils.ReportError(w, err, "Failed to parse JSON request.", http.StatusBadRequest)
		return
	}
	sklog.Infof("Triage v3 request: %#v", req)

	res, err := wh.triage3(ctx, user, req)
	if err != nil {
		httputils.ReportError(w, err, "Could not triage", http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, res)
}

func (wh *Handlers) triage3(ctx context.Context, userID string, req frontend.TriageRequestV3) (frontend.TriageResponse, error) {
	ctx, span := trace.StartSpan(ctx, "triage3")
	defer span.End()

	branch := ""
	if req.ChangelistID != "" && req.CodeReviewSystem != "" {
		branch = sql.Qualify(req.CodeReviewSystem, req.ChangelistID)

		// We disallow changes on closed CLs to avoid confusion (skbug.com/12122).
		const statement = "SELECT status FROM Changelists WHERE changelist_id = $1"
		row := wh.DB.QueryRow(ctx, statement, branch)
		var cl schema.ChangelistRow
		if err := row.Scan(&cl.Status); err != nil {
			return frontend.TriageResponse{}, skerr.Wrapf(err, "querying status of changelist (changelist ID %q, CRS %q)", req.ChangelistID, req.CodeReviewSystem)
		}
		if cl.Status != schema.StatusOpen {
			return frontend.TriageResponse{}, skerr.Fmt("triaging digests from non-open changelists is not allowed (changelist ID %q, CRS %q, status %q)", req.ChangelistID, req.CodeReviewSystem, cl.Status)
		}
	}

	// If set, use the image matching algorithm's name as the author of this change.
	if req.ImageMatchingAlgorithm != "" {
		userID = req.ImageMatchingAlgorithm
	}

	deltas, err := convertTriageDeltasToExpectationDeltaRows(req.Deltas)
	if err != nil {
		return frontend.TriageResponse{}, skerr.Wrapf(err, "converting TriageDeltas to ExpectationDeltaRows")
	}
	if len(deltas) == 0 {
		return frontend.TriageResponse{Status: frontend.TriageResponseStatusOK}, nil
	}

	span.AddAttributes(trace.Int64Attribute("num_changes", int64(len(deltas))))

	err = crdbpgx.ExecuteTx(ctx, wh.DB, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := verifyExpectationDeltaRowsLabelBefore(ctx, tx, deltas, branch); err != nil {
			// Could be a triageConflictError if any of the LabelBefore fields do not match their
			// expected value. This error is handled outside of the transaction.
			return err
		}
		newRecordID, err := writeRecord(ctx, tx, userID, len(deltas), branch)
		if err != nil {
			return err
		}
		for i := range deltas {
			deltas[i].ExpectationRecordID = newRecordID
		}
		err = writeDeltas(ctx, tx, deltas)
		if err != nil {
			return err
		}
		if branch == "" {
			return applyDeltasToPrimary(ctx, tx, deltas)
		}
		return applyDeltasToBranch(ctx, tx, deltas, branch)
	})
	if err != nil {
		// If any of the deltas' LabelBefore do not match the corresponding entries in the
		// Expectations or SecondaryBranchExpectations tables, we send a meaningful error response
		// to the frontend so that we can properly report the triage conflict in the UI.
		var tce *triageConflictError
		if errors.As(err, &tce) {
			grouping, err := wh.lookupGrouping(ctx, tce.GroupingID)
			if err != nil {
				return frontend.TriageResponse{}, skerr.Wrap(err)
			}
			return frontend.TriageResponse{
				Status: frontend.TriageResponseStatusConflict,
				Conflict: frontend.TriageConflict{
					Grouping:            grouping,
					Digest:              types.Digest(hex.EncodeToString(tce.Digest)),
					ExpectedLabelBefore: tce.ExpectedLabelBefore.ToExpectation(),
					ActualLabelBefore:   tce.ActualLabelBefore.ToExpectation(),
				},
			}, nil
		}
		return frontend.TriageResponse{}, skerr.Wrapf(err, "writing %d expectations from %s to branch %q", len(deltas), userID, branch)
	}
	return frontend.TriageResponse{Status: frontend.TriageResponseStatusOK}, nil
}

// convertTriageDeltasToExpectationDeltaRows converts frontend.TriageDelta structs to
// schema.ExpectationDeltaRow structs.
func convertTriageDeltasToExpectationDeltaRows(deltas []frontend.TriageDelta) ([]schema.ExpectationDeltaRow, error) {
	rv := make([]schema.ExpectationDeltaRow, 0, len(deltas))
	for _, delta := range deltas {
		if !expectations.ValidLabel(delta.LabelBefore) {
			return nil, skerr.Fmt("invalid LabelBefore %q in triage request", delta.LabelBefore)
		}
		if !expectations.ValidLabel(delta.LabelAfter) {
			return nil, skerr.Fmt("invalid LabelAfter %q in triage request", delta.LabelAfter)
		}
		labelBefore := schema.FromExpectationLabel(delta.LabelBefore)
		labelAfter := schema.FromExpectationLabel(delta.LabelAfter)
		_, groupingID := sql.SerializeMap(delta.Grouping)
		digestBytes, err := sql.DigestToBytes(delta.Digest)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, schema.ExpectationDeltaRow{
			GroupingID:  groupingID,
			Digest:      digestBytes,
			LabelBefore: labelBefore,
			LabelAfter:  labelAfter,
		})
	}
	return rv, nil
}

// triageConflictError is an error returned by the verifyExpectationDeltaRowsLabelBefore method. It
// contains the necessary information to construct a meaningful error response to return to the
// frontend.
type triageConflictError struct {
	GroupingID          schema.GroupingID
	Digest              schema.DigestBytes
	ExpectedLabelBefore schema.ExpectationLabel
	ActualLabelBefore   schema.ExpectationLabel
}

func (e *triageConflictError) Error() string {
	return fmt.Sprintf("expected LabelBefore for grouping %x and digest %x to be %s, was %s", e.GroupingID, e.Digest, e.ExpectedLabelBefore, e.ActualLabelBefore)
}

// groupingIDAndDigest is a (grouping ID, digest) pair.
type groupingIDAndDigest struct {
	groupingID schema.MD5Hash
	digest     schema.MD5Hash
}

// verifyExpectationDeltaRowsLabelBefore verifies that the LabelBefore column of each passed in
// schema.ExpectationDeltaRow matches the Label column of the corresponding entry in the
// Expectations or SecondaryBranchExpectations table. If no entry is found, we check that the
// LabelBefore is untriaged. This function prevents race conditions where multiple Gold users might
// attempt to triage the same digest.
//
// If branchName is empty, we only check against the Expectations table.
//
// If branchName is not empty (e.g. when triaging digests from a CL), we first check the
// SecondaryBranchExpectations table, and if there is no corresponding entry, we check against the
// Expectations table.
//
// If the LabelBefore of a schema.ExpectationDeltaRow does not match the expected label, a
// triageConflictError is returned.
func verifyExpectationDeltaRowsLabelBefore(ctx context.Context, tx pgx.Tx, deltaRows []schema.ExpectationDeltaRow, branchName string) error {
	ctx, span := trace.StartSpan(ctx, "verifyExpectationDeltaRowsLabelBefore")
	defer span.End()

	// Put the deltaRows in a map keyed by grouping ID and digest for easier querying.
	deltaRowsMap := map[groupingIDAndDigest]*schema.ExpectationDeltaRow{}
	for i := range deltaRows {
		key := groupingIDAndDigest{
			groupingID: sql.AsMD5Hash(deltaRows[i].GroupingID),
			digest:     sql.AsMD5Hash(deltaRows[i].Digest),
		}
		deltaRowsMap[key] = &deltaRows[i]
	}

	// Check the deltaRows' LabelBefore columns against the corresponding table.
	var (
		verifiedDeltaRows map[groupingIDAndDigest]bool
		err               error
	)
	if branchName == "" {
		verifiedDeltaRows, err = verifyPrimaryBranchLabelBefore(ctx, tx, deltaRowsMap)
	} else {
		verifiedDeltaRows, err = verifySecondaryBranchLabelBefore(ctx, tx, branchName, deltaRowsMap)
	}
	if err != nil {
		return err // Don't wrap - crdbpgx might retry
	}

	// If any of the deltaRows did not have a matching entry in the Expectations or
	// SecondaryBranchExpectations tables, check that their LabelBefore columns are set to
	// "untriaged".
	for key, deltaRow := range deltaRowsMap {
		if !verifiedDeltaRows[key] && deltaRow.LabelBefore != schema.LabelUntriaged {
			return &triageConflictError{
				GroupingID:          deltaRow.GroupingID,
				Digest:              deltaRow.Digest,
				ExpectedLabelBefore: schema.LabelUntriaged,
				ActualLabelBefore:   deltaRow.LabelBefore,
			}
		}
	}

	return nil
}

// makeGroupingAndDigestWhereClause builds the part of a "WHERE" clause that filters by grouping ID
// and digest. It returns the SQL clause and a list of parameter values.
func makeGroupingAndDigestWhereClause(deltaRows map[groupingIDAndDigest]*schema.ExpectationDeltaRow, startingPlaceholderNum int) (string, []interface{}) {
	var parts []string
	args := make([]interface{}, 0, 2*len(deltaRows))
	placeholderNum := startingPlaceholderNum
	for _, deltaRow := range deltaRows {
		parts = append(parts, fmt.Sprintf("(grouping_id = $%d AND digest = $%d)", placeholderNum, placeholderNum+1))
		args = append(args, deltaRow.GroupingID, deltaRow.Digest)
		placeholderNum += 2
	}
	sort.Strings(parts) // Make the query string deterministic for easier debugging.
	return strings.Join(parts, " OR "), args
}

// verifyPrimaryBranchLabelBefore verifies that the LabelBefore of each given ExpectationDeltaRow
// matches the label of the corresponding row in the Expectations table, if any. If the labels
// do not match, it returns a triageConflictError.
//
// It returns a set with one (grouping ID, digest) pair for each ExpectationDeltaRow it was able to
// verify, i.e. those with a corresponding row in the Expectations table.
func verifyPrimaryBranchLabelBefore(ctx context.Context, tx pgx.Tx, deltaRows map[groupingIDAndDigest]*schema.ExpectationDeltaRow) (map[groupingIDAndDigest]bool, error) {
	whereClause, whereArgs := makeGroupingAndDigestWhereClause(deltaRows, 1)
	statement := "SELECT grouping_id, digest, label FROM Expectations WHERE " + whereClause
	rows, err := tx.Query(ctx, statement, whereArgs...)
	if err != nil {
		return nil, err // Don't wrap - crdbpgx might retry
	}
	defer rows.Close()

	// Check that the LabelBefore of each ExpectationDeltaRow matches the label of the
	// corresponding row in the Expectations table, if any.
	verifiedDeltaRows := map[groupingIDAndDigest]bool{}
	for rows.Next() {
		var groupingID schema.GroupingID
		var digest schema.DigestBytes
		var label schema.ExpectationLabel
		if err := rows.Scan(&groupingID, &digest, &label); err != nil {
			return nil, err
		}

		key := groupingIDAndDigest{
			groupingID: sql.AsMD5Hash(groupingID),
			digest:     sql.AsMD5Hash(digest),
		}
		deltaRow := deltaRows[key]
		if deltaRow == nil {
			sklog.Warningf("Unmatched row with grouping %x and digest %x.", groupingID, digest)
			continue // Should never happen.
		}

		if label != deltaRow.LabelBefore {
			return nil, &triageConflictError{
				GroupingID:          groupingID,
				Digest:              digest,
				ExpectedLabelBefore: label,
				ActualLabelBefore:   deltaRow.LabelBefore,
			}
		}
		verifiedDeltaRows[key] = true
	}

	return verifiedDeltaRows, nil
}

// verifySecondaryBranchLabelBefore verifies that the LabelBefore of each given ExpectationDeltaRow
// matches the label of the corresponding row in the SecondaryBranchExpectations table. If there is
// no such row, it does the same against the corresponding row in the Expectations table, if any.
// If the LabelBefore does not match the label of the corresponding row in either table, it returns
// a triageConflictError.
//
// It returns a set with one (grouping ID, digest) pair for each ExpectationDeltaRow it was able to
// verify, i.e. those with a corresponding row in the SecondaryBranchExpectations or Expectations
// table.
func verifySecondaryBranchLabelBefore(ctx context.Context, tx pgx.Tx, branchName string, deltaRows map[groupingIDAndDigest]*schema.ExpectationDeltaRow) (map[groupingIDAndDigest]bool, error) {
	// Gather the relevant labels from the Expectations table.
	primaryBranchLabels := map[groupingIDAndDigest]schema.ExpectationLabel{}
	whereClause, whereArgs := makeGroupingAndDigestWhereClause(deltaRows, 1)
	statement := "SELECT grouping_id, digest, label FROM Expectations WHERE " + whereClause
	rows, err := tx.Query(ctx, statement, whereArgs...)
	if err != nil {
		return nil, err // Don't wrap - crdbpgx might retry
	}
	defer rows.Close()
	for rows.Next() {
		var groupingID schema.GroupingID
		var digest schema.DigestBytes
		var label schema.ExpectationLabel
		if err := rows.Scan(&groupingID, &digest, &label); err != nil {
			return nil, err
		}
		primaryBranchLabels[groupingIDAndDigest{
			groupingID: sql.AsMD5Hash(groupingID),
			digest:     sql.AsMD5Hash(digest),
		}] = label
	}

	// Gather the relevant labels from the SecondaryBranchExpectations table.
	secondaryBranchLabels := map[groupingIDAndDigest]schema.ExpectationLabel{}
	whereClause, whereArgs = makeGroupingAndDigestWhereClause(deltaRows, 2)
	statement = `
		SELECT grouping_id,
		       digest,
			   label
		  FROM SecondaryBranchExpectations
		 WHERE branch_name = $1 AND (` + whereClause + ")"
	rows, err = tx.Query(ctx, statement, append([]interface{}{branchName}, whereArgs...)...)
	if err != nil {
		return nil, err // Don't wrap - crdbpgx might retry
	}
	defer rows.Close()
	for rows.Next() {
		var groupingID schema.GroupingID
		var digest schema.DigestBytes
		var label schema.ExpectationLabel
		if err := rows.Scan(&groupingID, &digest, &label); err != nil {
			return nil, err
		}
		secondaryBranchLabels[groupingIDAndDigest{
			groupingID: sql.AsMD5Hash(groupingID),
			digest:     sql.AsMD5Hash(digest),
		}] = label
	}

	// Check that the LabelBefore of each ExpectationDeltaRow matches the label of the
	// corresponding row in the SecondaryBranchExpectations or Expectations table, if any.
	verifiedDeltaRows := map[groupingIDAndDigest]bool{}
	for key, deltaRow := range deltaRows {
		label, ok := secondaryBranchLabels[key]
		if !ok {
			label, ok = primaryBranchLabels[key]
		}
		if ok {
			if label != deltaRow.LabelBefore {
				return nil, &triageConflictError{
					GroupingID:          deltaRow.GroupingID,
					Digest:              deltaRow.Digest,
					ExpectedLabelBefore: label,
					ActualLabelBefore:   deltaRow.LabelBefore,
				}
			}
			verifiedDeltaRows[key] = true
		}
	}

	return verifiedDeltaRows, nil
}

// StatusHandler returns information about the most recently ingested data and the triage status
// of the various corpora.
func (wh *Handlers) StatusHandler(w http.ResponseWriter, r *http.Request) {
	_, span := trace.StartSpan(r.Context(), "web_StatusHandler")
	defer span.End()
	wh.statusCacheMutex.RLock()
	defer wh.statusCacheMutex.RUnlock()
	// This should be an incredibly cheap call and therefore does not count against any quota.
	sendJSONResponse(w, wh.statusCache)
}

// GroupingsHandler returns a map from corpus name to the list of keys that comprise the corpus
// grouping.
//
// If this Gold instance's JSON5 config includes a dictionary of grouping param keys by corpus,
// this method returns it. Otherwise, this method reads the grouping param keys by corpus from the
// status cache.
//
// For large Gold instances (e.g. Skia, Chrome) it is important to provide a dictionary of grouping
// param keys by corpus in its JSON5 config because:
//
//   - The status cache is periodically populated by a goroutine that runs a slow SQL query (~13
//     minutes in the case of the Skia instance).
//   - Upon launching an instance, the status cache remains empty for several minutes until said
//     goroutine finishes running the aforementioned slow SQL query for the first time.
//   - During that time, this RPC (/json/v1/groupings) returns an empty dictionary if the JSON5
//     config does not include a dictionary of grouping param keys by corpus.
//   - The "goldctl imgtest add" command hits this RPC to validate that the test being added
//     includes all the params required by its corpus' grouping.
//   - If the RPC returns an empty map, goldctl reports "grouping params for corpus X are unknown",
//     which causes spurious test failures in the associated CI system.
//
// Some possible alternatives:
//
//   - Write a fast SQL query specifically for /json/v1/groupings, but that's probably hard with
//     the current schema. It might require factoring the corpora out into their own table.
//   - Delay starting the webserver until the status cache is populated, but that would be at the
//     expense of a much longer startup time for large instances.
func (wh *Handlers) GroupingsHandler(w http.ResponseWriter, r *http.Request) {
	_, span := trace.StartSpan(r.Context(), "web_GroupingsHandler")
	defer span.End()

	// We will read the grouping param keys by corpus from the status cache. This should be an
	// incredibly cheap call and therefore does not count against any quota.
	wh.statusCacheMutex.RLock()
	defer wh.statusCacheMutex.RUnlock()

	// If the status cache's corpora status list is empty, and if this Gold instance's JSON5 config
	// includes a dictionary of grouping param keys by corpus, return it.
	if len(wh.statusCache.CorpStatus) == 0 && len(wh.GroupingParamKeysByCorpus) != 0 {
		res := frontend.GroupingsResponse{
			GroupingParamKeysByCorpus: wh.GroupingParamKeysByCorpus,
		}
		sendJSONResponse(w, res)
		return
	}

	res := frontend.GroupingsResponse{
		GroupingParamKeysByCorpus: map[string][]string{},
	}
	for _, cs := range wh.statusCache.CorpStatus {
		// For now, all corpora are grouped by corpus ("source_type") and test name ("name"), so
		// the groupings are hardcoded.
		//
		// If we ever want to support different groupings, these keys could be read in from the
		// Gold instance's JSON5 config file.
		res.GroupingParamKeysByCorpus[cs.Name] = []string{
			// Sorted lexicographically.
			types.PrimaryKeyField,
			types.CorpusField,
		}
	}

	sendJSONResponse(w, res)
}

// ClusterDiffRequest contains the options that the frontend provides to the clusterdiff RPC.
type ClusterDiffRequest struct {
	Corpus                  string
	Filters                 paramtools.ParamSet
	IncludePositiveDigests  bool
	IncludeNegativeDigests  bool
	IncludeUntriagedDigests bool
	// TODO(kjlubick) the frontend does not yet support these yet.
	ChangelistID       string
	CodeReviewSystemID string
	PatchsetID         string
}

func parseClusterDiffQuery(r *http.Request) (ClusterDiffRequest, error) {
	if err := r.ParseForm(); err != nil {
		return ClusterDiffRequest{}, skerr.Wrap(err)
	}
	var rv ClusterDiffRequest
	// TODO(kjlubick) rename this field on the UI side
	if corpus := r.FormValue("source_type"); corpus == "" {
		return ClusterDiffRequest{}, skerr.Fmt("Must include corpus")
	} else {
		rv.Corpus = corpus
	}
	if q := r.FormValue("query"); q == "" {
		return ClusterDiffRequest{}, skerr.Fmt("Must include query")
	} else {
		filters, err := url.ParseQuery(q)
		if err != nil {
			return ClusterDiffRequest{}, skerr.Wrapf(err, "invalid query %q", q)
		}
		rv.Filters = paramtools.ParamSet(filters)
	}
	rv.IncludePositiveDigests = r.FormValue("pos") == "true"
	rv.IncludeNegativeDigests = r.FormValue("neg") == "true"
	rv.IncludeUntriagedDigests = r.FormValue("unt") == "true"

	rv.CodeReviewSystemID = r.FormValue("crs")
	rv.ChangelistID = r.FormValue("cl_id")
	rv.PatchsetID = r.FormValue("ps_id")
	return rv, nil
}

// ClusterDiffHandler computes the diffs between all digests that match the filters and
// returns them in a way that is convenient for rendering via d3.js
func (wh *Handlers) ClusterDiffHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_ClusterDiffHandler", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	q, err := parseClusterDiffQuery(r)
	if err != nil {
		httputils.ReportError(w, err, "Invalid requrest", http.StatusBadRequest)
		return
	}

	testNames, ok := q.Filters[types.PrimaryKeyField]
	if !ok || len(testNames) == 0 {
		http.Error(w, "Must include test name", http.StatusBadRequest)
		return
	}
	leftGrouping := paramtools.Params{
		types.CorpusField:     q.Corpus,
		types.PrimaryKeyField: testNames[0],
	}
	delete(q.Filters, types.PrimaryKeyField)
	clusterOpts := search.ClusterOptions{
		Grouping:                leftGrouping,
		Filters:                 q.Filters,
		IncludePositiveDigests:  q.IncludePositiveDigests,
		IncludeNegativeDigests:  q.IncludeNegativeDigests,
		IncludeUntriagedDigests: q.IncludeUntriagedDigests,

		CodeReviewSystem: q.CodeReviewSystemID,
		ChangelistID:     q.ChangelistID,
		PatchsetID:       q.PatchsetID,
	}
	clusterResp, err := wh.Search2API.GetCluster(ctx, clusterOpts)
	if err != nil {
		httputils.ReportError(w, err, "Unable to compute cluster.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, clusterResp)
}

// ListTestsHandler returns all the tests in the given corpus and a count of how many digests
// have been seen for that.
func (wh *Handlers) ListTestsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_ListTestsHandler", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	// Inputs: (head, ignored, corpus, keys)
	q, err := frontend.ParseListTestsQuery(r)
	if err != nil {
		httputils.ReportError(w, err, "Failed to parse form data.", http.StatusBadRequest)
		return
	}

	counts, err := wh.Search2API.CountDigestsByTest(ctx, q)
	if err != nil {
		httputils.ReportError(w, err, "Could not compute query.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, counts)
}

// TriageLogHandler returns what has been triaged recently.
func (wh *Handlers) TriageLogHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_TriageLogHandler", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	// Get the pagination params.
	q := r.URL.Query()
	offset, size, err := httputils.PaginationParams(q, 0, pageSize, maxPageSize)
	if err != nil {
		httputils.ReportError(w, err, "Invalid Pagination params", http.StatusBadRequest)
		return
	}

	clID := q.Get("changelist_id")
	crs := q.Get("crs")
	if clID != "" {
		if _, ok := wh.getCodeReviewSystem(crs); !ok {
			http.Error(w, "Invalid Code Review System; did you include crs?", http.StatusBadRequest)
			return
		}
	} else {
		crs = ""
	}

	logEntries, total, err := wh.getTriageLog(ctx, crs, clID, offset, size)
	if err != nil {
		httputils.ReportError(w, err, "Unable to retrieve triage logs", http.StatusInternalServerError)
		return
	}

	response := frontend.TriageLogResponse{
		Entries: logEntries,
		ResponsePagination: httputils.ResponsePagination{
			Offset: offset,
			Size:   size,
			Total:  total,
		},
	}

	sendJSONResponse(w, response)
}

// getTriageLog returns the specified entries and the total count of expectation records.
func (wh *Handlers) getTriageLog(ctx context.Context, crs, clid string, offset, size int) ([]frontend.TriageLogEntry, int, error) {
	ctx, span := trace.StartSpan(ctx, "getTriageLog2")
	defer span.End()

	total, err := wh.getTotalTriageRecords(ctx, crs, clid)
	if err != nil {
		return nil, 0, skerr.Wrap(err)
	}
	if total == 0 {
		return []frontend.TriageLogEntry{}, 0, nil // We don't want null in our JSON response.
	}

	// Default to the primary branch, which is associated with branch_name (i.e. CL) as NULL.
	branchStatement := "WHERE branch_name IS NULL"
	if crs != "" {
		branchStatement = "WHERE branch_name = $3"
	}

	statement := `WITH
RecentRecords AS (
	SELECT expectation_record_id, user_name, triage_time
	FROM ExpectationRecords ` + branchStatement + `
	ORDER BY triage_time DESC, expectation_record_id
	OFFSET $1 LIMIT $2
)
SELECT RecentRecords.*, Groupings.keys, digest, label_before, label_after
FROM RecentRecords
	JOIN ExpectationDeltas ON RecentRecords.expectation_record_id = ExpectationDeltas.expectation_record_id
JOIN Groupings ON ExpectationDeltas.grouping_id = Groupings.grouping_id
ORDER BY triage_time DESC, expectation_record_id, digest
`
	args := []interface{}{offset, size}
	if crs != "" {
		args = append(args, sql.Qualify(crs, clid))
	}
	rows, err := wh.DB.Query(ctx, statement, args...)
	if err != nil {
		return nil, 0, skerr.Wrap(err)
	}
	defer rows.Close()
	var currentEntry *frontend.TriageLogEntry
	var rv []frontend.TriageLogEntry
	for rows.Next() {
		var record schema.ExpectationRecordRow
		var delta schema.ExpectationDeltaRow
		var grouping paramtools.Params
		if err := rows.Scan(&record.ExpectationRecordID, &record.UserName, &record.TriageTime,
			&grouping, &delta.Digest, &delta.LabelBefore, &delta.LabelAfter); err != nil {
			return nil, 0, skerr.Wrap(err)
		}
		if currentEntry == nil || currentEntry.ID != record.ExpectationRecordID.String() {
			rv = append(rv, frontend.TriageLogEntry{
				ID:   record.ExpectationRecordID.String(),
				User: record.UserName,
				// Multiply by 1000 to convert seconds to milliseconds
				TS: record.TriageTime.UTC().Unix() * 1000,
			})
			currentEntry = &rv[len(rv)-1]
		}
		currentEntry.Details = append(currentEntry.Details, frontend.TriageDelta{
			Grouping:    grouping,
			Digest:      types.Digest(hex.EncodeToString(delta.Digest)),
			LabelBefore: delta.LabelBefore.ToExpectation(),
			LabelAfter:  delta.LabelAfter.ToExpectation(),
		})
	}
	return rv, total, nil
}

// getTotalTriageRecords returns the total number of triage records for the CL (or the primary
// branch)
func (wh *Handlers) getTotalTriageRecords(ctx context.Context, crs, clid string) (int, error) {
	ctx, span := trace.StartSpan(ctx, "getTotalTriageRecords")
	defer span.End()

	branchStatement := "WHERE branch_name IS NULL"
	if crs != "" {
		branchStatement = "WHERE branch_name = $1"
	}

	statement := `SELECT COUNT(*) FROM ExpectationRecords ` + branchStatement
	var args []interface{}
	if crs != "" {
		args = append(args, sql.Qualify(crs, clid))
	}
	row := wh.DB.QueryRow(ctx, statement, args...)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, skerr.Wrap(err)
	}
	return count, nil
}

// TriageUndoHandler performs an "undo" for a given id. This id corresponds to the record id of the
// set of changes in the DB.
// If successful it returns the same result as a call to TriageLogHandler to reflect the changes.
func (wh *Handlers) TriageUndoHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_TriageUndoHandler", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	// Get the user and make sure they are logged in.
	user := login.LoggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to change expectations", http.StatusUnauthorized)
		return
	}

	// Extract the id to undo.
	changeID := r.URL.Query().Get("id")

	// Do the undo procedure.
	if err := wh.undoExpectationChanges(ctx, changeID, user); err != nil {
		httputils.ReportError(w, err, "Unable to undo.", http.StatusInternalServerError)
		return
	}

	// Send the same response as a query for the first page.
	wh.TriageLogHandler(w, r)
}

// undoExpectationChanges will look up all ExpectationDeltas associated with the record that has
// the given ID. It will set the current expectations for those digests/groupings to be the
// label_before value. This will all be done in a transaction.
func (wh *Handlers) undoExpectationChanges(ctx context.Context, recordID, userID string) error {
	ctx, span := trace.StartSpan(ctx, "undoExpectationChanges")
	defer span.End()

	err := crdbpgx.ExecuteTx(ctx, wh.DB, pgx.TxOptions{}, func(tx pgx.Tx) error {
		deltas, err := getDeltasForRecord(ctx, tx, recordID)
		if err != nil {
			return err // Don't wrap - crdbpgx might retry
		}
		if len(deltas) == 0 {
			return skerr.Fmt("no expectation deltas found for record %s", recordID)
		}
		branchNameRow := tx.QueryRow(ctx, `SELECT branch_name FROM ExpectationRecords WHERE expectation_record_id = $1`, recordID)
		var branchOfOriginal pgtype.Text
		if err := branchNameRow.Scan(&branchOfOriginal); err != nil {
			return err
		}

		newRecordID, err := writeRecord(ctx, tx, userID, len(deltas), branchOfOriginal.String)
		if err != nil {
			return err
		}

		invertedDeltas := invertDeltas(deltas, newRecordID)
		if err := writeDeltas(ctx, tx, invertedDeltas); err != nil {
			return err
		}

		if branchOfOriginal.Status != pgtype.Present {
			err = applyDeltasToPrimary(ctx, tx, invertedDeltas)
		} else {
			err = applyDeltasToBranch(ctx, tx, invertedDeltas, branchOfOriginal.String)
		}
		return err
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// writeRecord writes a new ExpectationRecord to the DB.
func writeRecord(ctx context.Context, tx pgx.Tx, userID string, numChanges int, branch string) (uuid.UUID, error) {
	ctx, span := trace.StartSpan(ctx, "writeRecord")
	defer span.End()

	var br *string
	if branch != "" {
		br = &branch
	}
	const statement = `INSERT INTO ExpectationRecords
(user_name, triage_time, num_changes, branch_name) VALUES ($1, $2, $3, $4) RETURNING expectation_record_id`
	row := tx.QueryRow(ctx, statement, userID, now.Now(ctx), numChanges, br)
	var recordUUID uuid.UUID
	err := row.Scan(&recordUUID)
	if err != nil {
		return uuid.UUID{}, err
	}
	return recordUUID, nil
}

// invertDeltas returns a slice of deltas corresponding to the same grouping+digest pairs as the
// original slice, but with inverted before/after labels and a new record ID.
func invertDeltas(deltas []schema.ExpectationDeltaRow, newRecordID uuid.UUID) []schema.ExpectationDeltaRow {
	var rv []schema.ExpectationDeltaRow
	for _, d := range deltas {
		rv = append(rv, schema.ExpectationDeltaRow{
			ExpectationRecordID: newRecordID,
			GroupingID:          d.GroupingID,
			Digest:              d.Digest,
			LabelBefore:         d.LabelAfter, // Intentionally flipped around
			LabelAfter:          d.LabelBefore,
		})
	}
	return rv
}

// getDeltasForRecord returns all ExpectationDeltaRows for the given record ID.
func getDeltasForRecord(ctx context.Context, tx pgx.Tx, recordID string) ([]schema.ExpectationDeltaRow, error) {
	ctx, span := trace.StartSpan(ctx, "getDeltasForRecord")
	defer span.End()
	const statement = `SELECT grouping_id, digest, label_before, label_after
FROM ExpectationDeltas WHERE expectation_record_id = $1`
	rows, err := tx.Query(ctx, statement, recordID)
	if err != nil {
		return nil, err // Don't wrap - crdbpgx might retry
	}
	defer rows.Close()
	var deltas []schema.ExpectationDeltaRow
	for rows.Next() {
		var row schema.ExpectationDeltaRow
		if err := rows.Scan(&row.GroupingID, &row.Digest, &row.LabelBefore, &row.LabelAfter); err != nil {
			return nil, skerr.Wrap(err) // probably not retriable
		}
		deltas = append(deltas, row)
	}
	return deltas, nil
}

// writeDeltas writes the given rows to the SQL DB.
func writeDeltas(ctx context.Context, tx pgx.Tx, deltas []schema.ExpectationDeltaRow) error {
	ctx, span := trace.StartSpan(ctx, "writeDeltas")
	defer span.End()

	const statement = `INSERT INTO ExpectationDeltas
(expectation_record_id, grouping_id, digest, label_before, label_after) VALUES `
	const valuesPerRow = 5
	vp := sqlutil.ValuesPlaceholders(valuesPerRow, len(deltas))
	arguments := make([]interface{}, 0, len(deltas)*valuesPerRow)
	for _, d := range deltas {
		arguments = append(arguments, d.ExpectationRecordID, d.GroupingID, d.Digest, d.LabelBefore, d.LabelAfter)
	}
	_, err := tx.Exec(ctx, statement+vp, arguments...)
	return err // don't wrap, could be retryable
}

// applyDeltasToPrimary applies the given deltas to the primary branch expectations.
func applyDeltasToPrimary(ctx context.Context, tx pgx.Tx, deltas []schema.ExpectationDeltaRow) error {
	ctx, span := trace.StartSpan(ctx, "applyDeltasToPrimary")
	defer span.End()

	const statement = `UPSERT INTO Expectations
(grouping_id, digest, label, expectation_record_id) VALUES `
	const valuesPerRow = 4
	vp := sqlutil.ValuesPlaceholders(valuesPerRow, len(deltas))
	arguments := make([]interface{}, 0, len(deltas)*valuesPerRow)
	for _, d := range deltas {
		arguments = append(arguments, d.GroupingID, d.Digest, d.LabelAfter, d.ExpectationRecordID)
	}
	_, err := tx.Exec(ctx, statement+vp, arguments...)
	return err // don't wrap, could be retryable
}

// applyDeltasToBranch applies the given deltas to the given branch (i.e. CL).
func applyDeltasToBranch(ctx context.Context, tx pgx.Tx, deltas []schema.ExpectationDeltaRow, branch string) error {
	ctx, span := trace.StartSpan(ctx, "applyInvertedDeltasToBranch")
	defer span.End()

	const statement = `UPSERT INTO SecondaryBranchExpectations
(branch_name, grouping_id, digest, label, expectation_record_id) VALUES `
	const valuesPerRow = 5
	vp := sqlutil.ValuesPlaceholders(valuesPerRow, len(deltas))
	arguments := make([]interface{}, 0, len(deltas)*valuesPerRow)
	for _, d := range deltas {
		arguments = append(arguments, branch, d.GroupingID, d.Digest, d.LabelAfter, d.ExpectationRecordID)
	}
	_, err := tx.Exec(ctx, statement+vp, arguments...)
	return err // don't wrap, could be retryable
}

// ParamsHandler returns all Params that could be searched over. It uses the SQL Backend and
// returns *only* the keys, not the options.
func (wh *Handlers) ParamsHandler(w http.ResponseWriter, r *http.Request) {
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	ctx, span := trace.StartSpan(r.Context(), "web_ParamsHandler", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()

	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, err, "Invalid form headers", http.StatusBadRequest)
		return
	}
	clID := r.Form.Get("changelist_id")
	crs := r.Form.Get("crs")

	if clID == "" {
		ps, err := wh.Search2API.GetPrimaryBranchParamset(ctx)
		if err != nil {
			httputils.ReportError(w, err, "Could not get paramset for primary branch", http.StatusInternalServerError)
			return
		}
		sendJSONResponse(w, ps)
		return
	}

	if _, ok := wh.getCodeReviewSystem(crs); !ok {
		http.Error(w, "Invalid Code Review System; did you include crs?", http.StatusBadRequest)
		return
	}
	ps, err := wh.Search2API.GetChangelistParamset(ctx, crs, clID)
	if err != nil {
		httputils.ReportError(w, err, "Could not get paramset for given CL", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, ps)
}

// CommitsHandler returns the last n commits with data that make up the sliding window.
func (wh *Handlers) CommitsHandler(w http.ResponseWriter, r *http.Request) {
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	ctx, span := trace.StartSpan(r.Context(), "web_CommitsHandler", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()

	commits, err := wh.Search2API.GetCommitsInWindow(ctx)
	if err != nil {
		httputils.ReportError(w, err, "Could not get commits", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, commits)
}

// KnownHashesHandler returns known hashes that have been written to GCS in the background
// Each line contains a single digest for an image. Bots will then only upload images which
// have a hash not found on this list, avoiding significant amounts of unnecessary uploads.
func (wh *Handlers) KnownHashesHandler(w http.ResponseWriter, r *http.Request) {
	// No limit for anon users - this is an endpoint backed up by baseline servers, and
	// should be able to handle a large load.
	_, span := trace.StartSpan(r.Context(), "web_TextKnownHashesProxy")
	defer span.End()
	w.Header().Set("Content-Type", "text/plain")
	wh.knownHashesMutex.RLock()
	defer wh.knownHashesMutex.RUnlock()
	if _, err := w.Write([]byte(wh.knownHashesCache)); err != nil {
		sklog.Errorf("Failed to write the known hashes", err)
		return
	}
}

// BaselineHandlerV2 returns a JSON representation of that baseline including
// baselines for a options issue. It can respond to requests like these:
//
//    /json/expectations
//    /json/expectations?issue=123456
//
// The "issue" parameter indicates the changelist ID for which we would like to
// retrieve the baseline. In that case the returned options will be a blend of
// the master baseline and the baseline defined for the changelist (usually
// based on tryjob results).
func (wh *Handlers) BaselineHandlerV2(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "frontend_BaselineHandlerV2")
	defer span.End()
	// No limit for anon users - this is an endpoint backed up by baseline servers, and
	// should be able to handle a large load.

	q := r.URL.Query()
	clID := q.Get("issue")
	crs := q.Get("crs")

	if clID != "" {
		if _, ok := wh.getCodeReviewSystem(crs); !ok {
			http.Error(w, "Invalid CRS provided.", http.StatusBadRequest)
			return
		}
	} else {
		crs = ""
	}

	bl, err := wh.fetchBaseline(ctx, crs, clID)
	if err != nil {
		httputils.ReportError(w, err, "Fetching baseline failed.", http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, bl)
}

// fetchBaseline returns an object that contains all the positive and negatively triaged digests
// for either the primary branch or the primary branch and the CL. As per usual, the triage status
// on a CL overrides the triage status on the primary branch.
func (wh *Handlers) fetchBaseline(ctx context.Context, crs, clID string) (frontend.BaselineV2Response, error) {
	ctx, span := trace.StartSpan(ctx, "fetchBaseline")
	defer span.End()

	// Return the baseline from the cache if possible.
	baselineCacheKey := "primary"
	if clID != "" {
		baselineCacheKey = fmt.Sprintf("%s_%s", crs, clID)
	}
	if val, ok := wh.baselineCache.Get(baselineCacheKey); ok {
		return val.(frontend.BaselineV2Response), nil
	}

	statement := `WITH
PrimaryBranchExps AS (
	SELECT grouping_id, digest, label FROM Expectations
	AS OF SYSTEM TIME '-0.1s'
	WHERE label = 'n' OR label = 'p'
)`
	var args []interface{}
	if crs == "" {
		span.AddAttributes(trace.StringAttribute("type", "primary"))
		statement += `
SELECT Groupings.keys ->> 'name', encode(digest, 'hex'), label FROM PrimaryBranchExps
JOIN Groupings ON PrimaryBranchExps.grouping_id = Groupings.grouping_id
AS OF SYSTEM TIME '-0.1s'`
	} else {
		span.AddAttributes(
			trace.StringAttribute("type", "changelist"),
			trace.StringAttribute("crs", crs),
			trace.StringAttribute("clID", clID))
		qCLID := sql.Qualify(crs, clID)
		statement += `,
CLExps AS (
	SELECT grouping_id, digest, label FROM SecondaryBranchExpectations
	AS OF SYSTEM TIME '-0.1s'
	WHERE branch_name = $1
),
JoinedExps AS (
	SELECT COALESCE(CLExps.grouping_id, PrimaryBranchExps.grouping_id) as grouping_id,
		COALESCE(CLExps.digest, PrimaryBranchExps.digest) as digest,
		COALESCE(CLExps.label, PrimaryBranchExps.label, 'u') as label
    FROM CLExps FULL OUTER JOIN PrimaryBranchExps ON
		CLExps.grouping_id = PrimaryBranchExps.grouping_id
		AND CLExps.digest = PrimaryBranchExps.digest
	AS OF SYSTEM TIME '-0.1s'
)
SELECT Groupings.keys ->> 'name', encode(digest, 'hex'), label FROM JoinedExps
JOIN Groupings ON JoinedExps.grouping_id = Groupings.grouping_id
AS OF SYSTEM TIME '-0.1s'
WHERE label = 'n' OR label = 'p'`
		args = append(args, qCLID)
	}
	rows, err := wh.DB.Query(ctx, statement, args...)
	if err != nil {
		return frontend.BaselineV2Response{}, skerr.Wrap(err)
	}
	defer rows.Close()
	baseline := expectations.Baseline{}
	for rows.Next() {
		var testName types.TestName
		var digest types.Digest
		var label schema.ExpectationLabel
		if err := rows.Scan(&testName, &digest, &label); err != nil {
			return frontend.BaselineV2Response{}, skerr.Wrap(err)
		}
		byDigest, ok := baseline[testName]
		if !ok {
			byDigest = map[types.Digest]expectations.Label{}
			baseline[testName] = byDigest
		}
		byDigest[digest] = label.ToExpectation()
	}

	response := frontend.BaselineV2Response{
		CodeReviewSystem: crs,
		ChangelistID:     clID,
		Expectations:     baseline,
	}

	// Cache the computed baseline.
	baselineCacheEntryTTL := baselineCachePrimaryBranchEntryTTL
	if clID != "" {
		baselineCacheEntryTTL = baselineCacheSecondaryBranchEntryTTL
	}
	wh.baselineCache.Set(baselineCacheKey, response, baselineCacheEntryTTL)

	return response, nil
}

// DigestListHandler returns a list of digests for a given test. This is used by goldctl's
// local diff tech.
func (wh *Handlers) DigestListHandler(w http.ResponseWriter, r *http.Request) {
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	ctx, span := trace.StartSpan(r.Context(), "web_DigestListHandler")
	defer span.End()

	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, err, "Failed to parse form values", http.StatusInternalServerError)
		return
	}

	encodedGrouping := r.Form.Get("grouping")
	if encodedGrouping == "" {
		http.Error(w, "You must include 'grouping'", http.StatusBadRequest)
		return
	}
	groupingSet, err := url.ParseQuery(encodedGrouping)
	if err != nil {
		httputils.ReportError(w, skerr.Wrapf(err, "bad grouping %s", encodedGrouping), "Invalid grouping", http.StatusBadRequest)
		return
	}
	grouping := make(paramtools.Params, len(groupingSet))
	for key, values := range groupingSet {
		if len(values) == 0 {
			continue
		}
		grouping[key] = values[0]
	}

	// If needed, we could add a TTL cache here.
	out, err := wh.Search2API.GetDigestsForGrouping(ctx, grouping)
	if err != nil {
		httputils.ReportError(w, err, "Could not retrieve digests", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, out)
}

// Whoami returns the email address of the user or service account used to authenticate the
// request. For debugging purposes only.
func (wh *Handlers) Whoami(w http.ResponseWriter, r *http.Request) {
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	_, span := trace.StartSpan(r.Context(), "web_Whoami")
	defer span.End()

	user := wh.loggedInAs(r)
	sendJSONResponse(w, map[string]string{"whoami": user})
}

// LatestPositiveDigestHandler returns the most recent positive digest for the given trace.
// Starting at the tip of tree, it will skip over any missing data, untriaged digests or digests
// triaged negative until it finds a positive digest.
func (wh *Handlers) LatestPositiveDigestHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_LatestPositiveDigestHandler")
	defer span.End()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	tID, ok := mux.Vars(r)["traceID"]
	if !ok {
		http.Error(w, "Must specify traceID.", http.StatusBadRequest)
		return
	}
	traceID, err := hex.DecodeString(tID)
	if err != nil {
		httputils.ReportError(w, err, "Invalid traceID - must be an MD5 hash", http.StatusBadRequest)
		return
	}
	digest, err := wh.getLatestPositiveDigest(ctx, traceID)
	if err != nil {
		httputils.ReportError(w, err, "Could not complete query.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, frontend.MostRecentPositiveDigestResponse{Digest: digest})
}

func (wh *Handlers) getLatestPositiveDigest(ctx context.Context, traceID schema.TraceID) (types.Digest, error) {
	ctx, span := trace.StartSpan(ctx, "getLatestPositiveDigest")
	defer span.End()

	const statement = `WITH
RecentDigests AS (
	SELECT digest, commit_id, grouping_id FROM TraceValues WHERE trace_id = $1
	ORDER BY commit_id DESC LIMIT 1000 -- arbitrary limit
)
SELECT encode(RecentDigests.digest, 'hex') FROM RecentDigests
JOIN Expectations ON Expectations.grouping_id = RecentDigests.grouping_id AND
	Expectations.digest = RecentDigests.digest
WHERE label = 'p'
ORDER BY commit_id DESC LIMIT 1
`
	row := wh.DB.QueryRow(ctx, statement, traceID)
	var digest types.Digest
	if err := row.Scan(&digest); err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", skerr.Wrap(err)
	}
	return digest, nil
}

// ChangelistSearchRedirect redirects the user to a search page showing the search results
// for a given CL. It will do a (hopefully) quick scan of the untriaged digests - if it finds some,
// it will include the corpus containing some of those untriaged digests in the search query so the
// user will see results (instead of getting directed to a corpus with no results).
func (wh *Handlers) ChangelistSearchRedirect(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_ChangelistSearchRedirect")
	defer span.End()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
	}

	requestVars := mux.Vars(r)
	crs, ok := requestVars["system"]
	if !ok {
		http.Error(w, "Must specify 'system' of Changelist.", http.StatusBadRequest)
		return
	}
	clID, ok := requestVars["id"]
	if !ok {
		http.Error(w, "Must specify 'id' of Changelist.", http.StatusBadRequest)
		return
	}
	_, ok = wh.getCodeReviewSystem(crs)
	if !ok {
		http.Error(w, "Invalid Code Review System", http.StatusBadRequest)
		return
	}
	// This allows users to link to something like:
	// https://gold.skia.org/cl/gerrit/3213712&master=true
	// And the page loads showing the all the results.
	extraQueryParam := ""
	if strings.Contains(clID, "?") {
		s := strings.Split(clID, "?")
		clID, extraQueryParam = s[0], s[1]
	} else if strings.Contains(clID, "&") {
		s := strings.Split(clID, "&")
		clID, extraQueryParam = s[0], s[1]
	}

	qualifiedPSID, psOrder, err := wh.getLatestPatchset(ctx, crs, clID)
	if err != nil {
		httputils.ReportError(w, err, "Could not find latest patchset", http.StatusNotFound)
		return
	}
	// TODO(kjlubick) when we change the patchsets arg to not be a list of orders, we should
	//   update it here too (probably specify the ps id).
	baseURL := fmt.Sprintf("/search?issue=%s&crs=%s&patchsets=%d", clID, crs, psOrder)
	if extraQueryParam != "" {
		baseURL += "&" + extraQueryParam
	}

	corporaWithUntriagedUnignoredDigests, err := wh.getActionableDigests(ctx, crs, clID, qualifiedPSID)
	if err != nil {
		sklog.Errorf("Error getting digests for CL %s from CRS %s with PS %s: %s", clID, crs, qualifiedPSID, err)
		http.Redirect(w, r, baseURL, http.StatusTemporaryRedirect)
		return
	}
	if len(corporaWithUntriagedUnignoredDigests) == 0 {
		http.Redirect(w, r, baseURL, http.StatusTemporaryRedirect)
		return
	}
	http.Redirect(w, r, baseURL+"&corpus="+corporaWithUntriagedUnignoredDigests[0].Corpus, http.StatusTemporaryRedirect)
}

// getLatestPatchset returns the latest patchset for a given CL. It goes off of created_ts, due
// to the fact that (for GitHub) rebases can happen and potentially cause ps_order to be off.
func (wh *Handlers) getLatestPatchset(ctx context.Context, crs, clID string) (string, int, error) {
	ctx, span := trace.StartSpan(ctx, "getLatestPatchset")
	defer span.End()
	const statement = `SELECT patchset_id, ps_order FROM Patchsets
WHERE changelist_id = $1
ORDER BY created_ts DESC, ps_order DESC
LIMIT 1`
	row := wh.DB.QueryRow(ctx, statement, sql.Qualify(crs, clID))
	var qualifiedID string
	var order int
	if err := row.Scan(&qualifiedID, &order); err != nil {
		return "", 0, skerr.Wrap(err)
	}
	return qualifiedID, order, nil
}

type corpusAndCount struct {
	Corpus string
	Count  int
}

// getActionableDigests returns a list of corpus and the number of untriaged, not-ignored digests
// that have been seen in the data for the given PS. We choose *not* to strip out digests that
// are already on the primary branch because that additional join makes this query too slow.
// As is, it can take 3-4 seconds on a large instance like Skia. The return value will be sorted
// by count, with the corpus name being the tie-breaker.
func (wh *Handlers) getActionableDigests(ctx context.Context, crs, clID, qPSID string) ([]corpusAndCount, error) {
	ctx, span := trace.StartSpan(ctx, "getActionableDigests")
	defer span.End()

	const statement = `WITH
DataFromCL AS (
    SELECT secondary_branch_trace_id, SecondaryBranchValues.grouping_id, digest
    FROM SecondaryBranchValues WHERE branch_name = $1 AND version_name = $2
),
ExpectationsForCL AS (
    SELECT grouping_id, digest, label
    FROM SecondaryBranchExpectations
    WHERE branch_name = $1
),
JoinedExpectations AS (
    SELECT COALESCE(ExpectationsForCL.grouping_id, Expectations.grouping_id) AS grouping_id,
        COALESCE(ExpectationsForCL.digest, Expectations.digest) AS digest,
        COALESCE(ExpectationsForCL.label, Expectations.label, 'u') AS label
    FROM ExpectationsForCL FULL OUTER JOIN Expectations ON
    ExpectationsForCL.grouping_id = Expectations.grouping_id
        AND ExpectationsForCL.digest = Expectations.digest
),
UntriagedData AS (
    SELECT secondary_branch_trace_id, DataFromCL.grouping_id, DataFromCL.digest FROM DataFromCL
    LEFT JOIN JoinedExpectations ON DataFromCL.grouping_id = JoinedExpectations.grouping_id
        AND DataFromCL.digest = JoinedExpectations.digest
    WHERE label = 'u' OR label IS NULL
),
UnignoredUntriagedData AS (
    SELECT DISTINCT UntriagedData.grouping_id, digest FROM UntriagedData
    JOIN Traces ON UntriagedData.secondary_branch_trace_id = Traces.trace_id
    AND matches_any_ignore_rule = FALSE
)
SELECT keys->>'source_type', COUNT(*) FROM Groupings JOIN UnignoredUntriagedData
    ON Groupings.grouping_id = UnignoredUntriagedData.grouping_id
GROUP BY 1
ORDER BY 2 DESC, 1 ASC`

	rows, err := wh.DB.Query(ctx, statement, sql.Qualify(crs, clID), qPSID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var rv []corpusAndCount
	for rows.Next() {
		var c corpusAndCount
		if err := rows.Scan(&c.Corpus, &c.Count); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, c)
	}
	return rv, nil
}

func (wh *Handlers) loggedInAs(r *http.Request) string {
	if wh.testingAuthAs != "" {
		return wh.testingAuthAs
	}
	return login.LoggedInAs(r)
}

func (wh *Handlers) getCodeReviewSystem(crs string) (clstore.ReviewSystem, bool) {
	var system clstore.ReviewSystem
	found := false
	for _, rs := range wh.ReviewSystems {
		if rs.ID == crs {
			system = rs
			found = true
		}
	}
	return system, found
}

const (
	validDigestLength = 2 * md5.Size
	dotPNG            = ".png"
)

// ImageHandler returns either a single image or a diff between two images identified by their
// respective digests.
func (wh *Handlers) ImageHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_ImageHandler")
	defer span.End()
	// No rate limit, as this should be quite fast.
	_, imgFile := path.Split(r.URL.Path)
	// Get the file that was requested and verify that it's a valid PNG file.
	if !strings.HasSuffix(imgFile, dotPNG) {
		noCacheNotFound(w)
		return
	}

	// Trim the image extension to get the image or diff ID.
	imgID := imgFile[:len(imgFile)-len(dotPNG)]
	// Cache images for 12 hours.
	w.Header().Set("Cache-Control", "public, max-age=43200")
	if len(imgID) == validDigestLength {
		// Example request:
		// https://skia-infra-gold.skia.org/img/images/8588cad6f3821b948468df35b67778ef.png
		wh.serveImageWithDigest(ctx, w, types.Digest(imgID))
	} else if len(imgID) == validDigestLength*2+1 {
		// Example request:
		// https://skia-infra-gold.skia.org/img/diffs/81c4d3a64cf32143ff6c1fbf4cbbec2d-d20731492287002a3f046eae4bd4ce7d.png
		left := types.Digest(imgID[:validDigestLength])
		// + 1 for the dash
		right := types.Digest(imgID[validDigestLength+1:])
		wh.serveImageDiff(ctx, w, left, right)
	} else {
		noCacheNotFound(w)
		return
	}
}

// serveImageWithDigest downloads the image from GCS and returns it. If there is an error, a 404
// or 500 error is returned, as appropriate.
func (wh *Handlers) serveImageWithDigest(ctx context.Context, w http.ResponseWriter, digest types.Digest) {
	ctx, span := trace.StartSpan(ctx, "serveImageWithDigest")
	defer span.End()
	// Go's image package has no color profile support and we convert to 8-bit NRGBA to diff,
	// but our source images may have embedded color profiles and be up to 16-bit. So we must
	// at least take care to serve the original .pngs unaltered.
	b, err := wh.GCSClient.GetImage(ctx, digest)
	if err != nil {
		sklog.Warningf("Could not get image with digest %s: %s", digest, err)
		noCacheNotFound(w)
		return
	}
	if _, err := w.Write(b); err != nil {
		httputils.ReportError(w, err, "Could not load image. Try again later.", http.StatusInternalServerError)
		return
	}
}

// serveImageDiff downloads the left and right images, computes the diff between them, encodes
// the diff as a PNG image and writes it to the provided ResponseWriter. If there is an error, it
// returns a 404 or 500 error as appropriate.
func (wh *Handlers) serveImageDiff(ctx context.Context, w http.ResponseWriter, left types.Digest, right types.Digest) {
	ctx, span := trace.StartSpan(ctx, "serveImageDiff")
	defer span.End()
	// TODO(lovisolo): Diff in NRGBA64?
	// TODO(lovisolo): Make sure each pair of images is in the same color space before diffing?
	//                 (They probably are today but it'd be a good correctness check to make sure.)
	eg, eCtx := errgroup.WithContext(ctx)
	var leftImg *image.NRGBA
	var rightImg *image.NRGBA
	eg.Go(func() error {
		b, err := wh.GCSClient.GetImage(eCtx, left)
		if err != nil {
			return skerr.Wrap(err)
		}
		leftImg, err = decode(b)
		return skerr.Wrap(err)
	})
	eg.Go(func() error {
		b, err := wh.GCSClient.GetImage(eCtx, right)
		if err != nil {
			return skerr.Wrap(err)
		}
		rightImg, err = decode(b)
		return skerr.Wrap(err)
	})
	if err := eg.Wait(); err != nil {
		sklog.Warningf("Could not get diff for images %q and %q: %s", left, right, err)
		noCacheNotFound(w)
		return
	}
	// Compute the diff image.
	_, diffImg := diff.PixelDiff(leftImg, rightImg)

	// Write output image to the http.ResponseWriter. Content-Type is set automatically
	// based on the first 512 bytes of written data. See docs for ResponseWriter.Write()
	// for details.
	//
	// The encoding step below does not take color profiles into account. This is fine since
	// both the left and right images used to compute the diff are in the same color space,
	// and also because the resulting diff image is just a visual approximation of the
	// differences between the left and right images.
	if err := encodeImg(w, diffImg); err != nil {
		httputils.ReportError(w, err, "could not serve diff image", http.StatusInternalServerError)
		return
	}
}

// decode decodes the provided bytes as a PNG and returns them as an *image.NRGBA.
func decode(b []byte) (*image.NRGBA, error) {
	im, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return diff.GetNRGBA(im), nil
}

// noCacheNotFound disables caching and returns a 404.
func noCacheNotFound(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	http.NotFound(w, nil)
}

// ChangelistSummaryHandler returns a summary of the new and untriaged digests produced by this
// CL across all Patchsets.
func (wh *Handlers) ChangelistSummaryHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_ChangelistSummaryHandler", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	if err := wh.cheapLimitForGerritPlugin(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	clID, ok := mux.Vars(r)["id"]
	if !ok {
		http.Error(w, "Must specify 'id' of Changelist.", http.StatusBadRequest)
		return
	}
	crs, ok := mux.Vars(r)["system"]
	if !ok {
		http.Error(w, "Must specify 'system' of Changelist.", http.StatusBadRequest)
		return
	}
	system, ok := wh.getCodeReviewSystem(crs)
	if !ok {
		http.Error(w, "Invalid Code Review System", http.StatusBadRequest)
		return
	}

	qCLID := sql.Qualify(system.ID, clID)
	sum, err := wh.getCLSummary2(ctx, qCLID)
	if err != nil {
		httputils.ReportError(w, err, "Could not get summary", http.StatusInternalServerError)
		return
	}
	rv := convertChangelistSummaryResponseV1(sum)
	sendJSONResponse(w, rv)
}

// getCLSummary2 fetches, caches, and returns the summary for a given CL. If the result has already
// been cached, it will return that cached value with a flag if the value is still up to date or
// not. If the cached data is stale, it will spawn a goroutine to update the cached value.
func (wh *Handlers) getCLSummary2(ctx context.Context, qCLID string) (search.NewAndUntriagedSummary, error) {
	ts, err := wh.Search2API.ChangelistLastUpdated(ctx, qCLID)
	if err != nil {
		return search.NewAndUntriagedSummary{}, skerr.Wrap(err)
	}
	if ts.IsZero() { // A Zero time means we have no data for this CL.
		return search.NewAndUntriagedSummary{}, nil
	}

	cached, ok := wh.clSummaryCache.Get(qCLID)
	if ok {
		sum, ok := cached.(search.NewAndUntriagedSummary)
		if ok {
			if ts.Before(sum.LastUpdated) || sum.LastUpdated.Equal(ts) {
				sum.Outdated = false
				return sum, nil
			}
			// Result is stale. Start a goroutine to fetch it again.
			done := make(chan struct{})
			go func() {
				// We intentionally use context.Background() and not the request's context because
				// if we return a result, we want the fetching in the background to continue so
				// if/when the client tries again, we can serve that updated result.
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				newValue, err := wh.Search2API.NewAndUntriagedSummaryForCL(ctx, qCLID)
				if err != nil {
					sklog.Warningf("Could not fetch out of date summary for cl %s in background: %s", qCLID, err)
					return
				}
				wh.clSummaryCache.Add(qCLID, newValue)
				done <- struct{}{}
			}()
			// Wait up to 500ms to return the latest value quickly if available
			timer := time.NewTimer(500 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-done:
			case <-timer.C:
			}
			cached, ok = wh.clSummaryCache.Get(qCLID)
			if ok {
				if possiblyUpdated, ok := cached.(search.NewAndUntriagedSummary); ok {
					if ts.Before(possiblyUpdated.LastUpdated) || possiblyUpdated.LastUpdated.Equal(ts) {
						// We were able to fetch new data quickly, so return it now.
						possiblyUpdated.Outdated = false
						return possiblyUpdated, nil
					}
				}
			}
			// The cached data is still stale or invalid, so return what we have marked as outdated.
			sum.Outdated = true
			return sum, nil
		}
	}
	// Invalid or missing cache entry. We must fetch because we have nothing to give the user.
	sum, err := wh.Search2API.NewAndUntriagedSummaryForCL(ctx, qCLID)
	if err != nil {
		return search.NewAndUntriagedSummary{}, skerr.Wrap(err)
	}
	wh.clSummaryCache.Add(qCLID, sum)
	return sum, nil
}

// convertChangelistSummaryResponseV1 converts the search2 version of a Changelist summary into
// the version expected by the frontend.
func convertChangelistSummaryResponseV1(summary search.NewAndUntriagedSummary) frontend.ChangelistSummaryResponseV1 {
	xps := make([]frontend.PatchsetNewAndUntriagedSummaryV1, 0, len(summary.PatchsetSummaries))
	for _, ps := range summary.PatchsetSummaries {
		xps = append(xps, frontend.PatchsetNewAndUntriagedSummaryV1{
			NewImages:            ps.NewImages,
			NewUntriagedImages:   ps.NewUntriagedImages,
			TotalUntriagedImages: ps.TotalUntriagedImages,
			PatchsetID:           ps.PatchsetID,
			PatchsetOrder:        ps.PatchsetOrder,
		})
	}
	// It is convenient for the UI to have these sorted with the latest patchset first.
	sort.Slice(xps, func(i, j int) bool {
		return xps[i].PatchsetOrder > xps[j].PatchsetOrder
	})
	return frontend.ChangelistSummaryResponseV1{
		ChangelistID:      summary.ChangelistID,
		PatchsetSummaries: xps,
		Outdated:          summary.Outdated,
	}
}

// StartCacheWarming starts warming the caches for data we want to serve quickly. It starts
// goroutines that will run in the background (until the provided context is cancelled).
func (wh *Handlers) StartCacheWarming(ctx context.Context) {
	wh.startCLCacheProcess(ctx)
	wh.startStatusCacheProcess(ctx)
	wh.startIgnoredTraceCacheProcess(ctx)
	wh.StartKnownHashesCacheProcess(ctx)
}

// startCLCacheProcess starts a go routine to warm the CL Summary cache. This way, most
// summaries are responsive, even on big instances.
func (wh *Handlers) startCLCacheProcess(ctx context.Context) {
	// We warm every CL that was open and produced data or saw triage activity in the last 5 days.
	// After the first cycle, we will incrementally update the cache.
	lastCheck := now.Now(ctx).Add(-5 * 24 * time.Hour)
	go util.RepeatCtx(ctx, time.Minute, func(ctx context.Context) {
		ctx, span := trace.StartSpan(ctx, "web_warmCLCacheCycle", trace.WithSampler(trace.AlwaysSample()))
		defer span.End()
		newTS := now.Now(ctx)
		rows, err := wh.DB.Query(ctx, `WITH
ChangelistsWithNewData AS (
	SELECT changelist_id FROM Changelists
	WHERE status = 'open' and last_ingested_data > $1
),
ChangelistsWithTriageActivity AS (
	SELECT DISTINCT branch_name AS changelist_id FROM ExpectationRecords
	WHERE branch_name IS NOT NULL AND triage_time > $1
)
SELECT changelist_id FROM ChangelistsWithNewData
UNION
SELECT changelist_id FROM ChangelistsWithTriageActivity
`, lastCheck)
		if err != nil {
			if err == pgx.ErrNoRows {
				sklog.Infof("No CLS updated since %s", lastCheck)
				lastCheck = newTS
				return
			}
			sklog.Errorf("Could not fetch updated CLs to warm cache: %s", err)
			return
		}
		defer rows.Close()
		var qualifiedIDS []string
		for rows.Next() {
			var qID string
			if err := rows.Scan(&qID); err != nil {
				sklog.Errorf("Could not scan: %s", err)
			}
			qualifiedIDS = append(qualifiedIDS, qID)
		}
		sklog.Infof("Warming cache for %d CLs", len(qualifiedIDS))
		span.AddAttributes(trace.Int64Attribute("num_cls", int64(len(qualifiedIDS))))
		// warm cache 3 at a time. This number of goroutines was chosen arbitrarily.
		_ = util.ChunkIterParallel(ctx, len(qualifiedIDS), len(qualifiedIDS)/3+1, func(ctx context.Context, startIdx int, endIdx int) error {
			if err := ctx.Err(); err != nil {
				return nil
			}
			for _, qCLID := range qualifiedIDS[startIdx:endIdx] {
				_, err := wh.getCLSummary2(ctx, qCLID)
				if err != nil {
					sklog.Warningf("Ignoring error while warming CL Cache for %s: %s", qCLID, err)
				}
			}
			return nil
		})
		lastCheck = newTS
		sklog.Infof("Done warming cache")
	})
}

// startStatusCacheProcess will compute the GUI Status on a timer and save it to the cache.
func (wh *Handlers) startStatusCacheProcess(ctx context.Context) {
	go util.RepeatCtx(ctx, time.Minute, func(ctx context.Context) {
		ctx, span := trace.StartSpan(ctx, "web_warmStatusCacheCycle", trace.WithSampler(trace.AlwaysSample()))
		defer span.End()

		gs, err := wh.Search2API.ComputeGUIStatus(ctx)
		if err != nil {
			sklog.Errorf("Could not compute GUI Status: %s", err)
			return
		}

		wh.statusCacheMutex.Lock()
		defer wh.statusCacheMutex.Unlock()
		wh.statusCache = gs
	})
}

// StartKnownHashesCacheProcess will fetch the known hashes on a timer and save it to the cache.
func (wh *Handlers) StartKnownHashesCacheProcess(ctx context.Context) {
	go util.RepeatCtx(ctx, time.Minute, func(ctx context.Context) {
		ctx, span := trace.StartSpan(ctx, "web_warmKnownHashesCacheCycle", trace.WithSampler(trace.AlwaysSample()))
		defer span.End()

		var buf bytes.Buffer
		if err := wh.GCSClient.LoadKnownDigests(ctx, &buf); err != nil {
			sklog.Errorf("Could not fetch known digests: %s", err)
			return
		}

		wh.knownHashesMutex.Lock()
		defer wh.knownHashesMutex.Unlock()
		wh.knownHashesCache = buf.String()
	})
}

type ignoredTrace struct {
	Keys  paramtools.Params
	Label expectations.Label
}

//startIgnoredTraceCacheProcess will periodically update the cache of ignored traces.
func (wh *Handlers) startIgnoredTraceCacheProcess(ctx context.Context) {
	go util.RepeatCtx(ctx, time.Minute, func(ctx context.Context) {
		ctx, span := trace.StartSpan(ctx, "web_warmIgnoredTraceCacheCycle", trace.WithSampler(trace.AlwaysSample()))
		defer span.End()

		if err := wh.updateIgnoredTracesCache(ctx); err != nil {
			sklog.Errorf("Could not get all ignored traces: %s", err)
		}
	})
}

// updateIgnoredTracesCache fetches all ignored traces that have recent data and returns both
// the trace keys and the triage status of the digest at ToT.
func (wh *Handlers) updateIgnoredTracesCache(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "updateIgnoredTracesCache")
	defer span.End()

	const statement = `WITH
RecentCommits AS (
	SELECT commit_id FROM CommitsWithData
	ORDER BY commit_id DESC LIMIT $1
),
OldestCommitInWindow AS (
	SELECT commit_id FROM RecentCommits
	ORDER BY commit_id ASC LIMIT 1
)
SELECT keys, label FROM ValuesAtHead
JOIN OldestCommitInWindow ON ValuesAtHead.most_recent_commit_id >= OldestCommitInWindow.commit_id
	AND matches_any_ignore_rule = TRUE
JOIN Expectations ON ValuesAtHead.grouping_id = Expectations.grouping_id
	AND ValuesAtHead.digest = Expectations.digest
`

	rows, err := wh.DB.Query(ctx, statement, wh.WindowSize)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer rows.Close()

	var ignoredTraces []ignoredTrace
	for rows.Next() {
		var ps paramtools.Params
		var label schema.ExpectationLabel
		if err := rows.Scan(&ps, &label); err != nil {
			return skerr.Wrap(err)
		}
		ignoredTraces = append(ignoredTraces, ignoredTrace{
			Keys:  ps,
			Label: label.ToExpectation(),
		})
	}

	wh.ignoredTracesCacheMutex.Lock()
	defer wh.ignoredTracesCacheMutex.Unlock()
	wh.ignoredTracesCache = ignoredTraces
	return nil
}

// PositiveDigestsByGroupingIDHandler returns all positively triaged digests seen in the sliding
// window for a given grouping, split up by trace.
// Used by https://source.chromium.org/chromium/chromium/src/+/main:content/test/gpu/gold_inexact_matching/base_parameter_optimizer.py
func (wh *Handlers) PositiveDigestsByGroupingIDHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_PositiveDigestsByGroupingIDHandler", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	gID := mux.Vars(r)["groupingID"]
	if len(gID) != 2*md5.Size {
		http.Error(w, "Must specify 'groupingID', which is a hex-encoded MD5 hash of the JSON encoded group keys (e.g. source_type and name)", http.StatusBadRequest)
		return
	}
	groupingID, err := hex.DecodeString(gID)
	if err != nil {
		httputils.ReportError(w, err, "Invalid 'groupingID', which is a hex-encoded MD5 hash of the JSON encoded group keys (e.g. source_type and name)", http.StatusBadRequest)
		return
	}

	groupingKeys, err := wh.lookupGrouping(ctx, groupingID)
	if err != nil {
		httputils.ReportError(w, err, "Unknown groupingID", http.StatusBadRequest)
		return
	}

	beginTile, endTile, err := wh.getTilesInWindow(ctx)
	if err != nil {
		httputils.ReportError(w, err, "Error while finding commits with data", http.StatusInternalServerError)
		return
	}

	resp, err := wh.getPositiveDigests(ctx, beginTile, endTile, groupingID)
	if err != nil {
		httputils.ReportError(w, err, "Error while finding positive traces for grouping", http.StatusInternalServerError)
		return
	}
	resp.GroupingID = gID
	resp.GroupingKeys = groupingKeys

	sendJSONResponse(w, resp)
}

// lookupGrouping returns the keys associated with the provided grouping id.
func (wh *Handlers) lookupGrouping(ctx context.Context, id schema.GroupingID) (paramtools.Params, error) {
	ctx, span := trace.StartSpan(ctx, "lookupGrouping")
	defer span.End()

	row := wh.DB.QueryRow(ctx, `SELECT keys FROM Groupings WHERE grouping_id = $1`, id)
	var keys paramtools.Params
	err := row.Scan(&keys)
	if err != nil {
		return nil, skerr.Wrap(err) // likely the grouping was not found
	}
	return keys, nil
}

// getTilesInWindow returns the start and end tile of the given window.
func (wh *Handlers) getTilesInWindow(ctx context.Context) (schema.TileID, schema.TileID, error) {
	ctx, span := trace.StartSpan(ctx, "getTilesInWindow")
	defer span.End()
	const statement = `WITH
RecentCommits AS (
	SELECT tile_id, commit_id FROM CommitsWithData
	AS OF SYSTEM TIME '-0.1s'
	ORDER BY commit_id DESC LIMIT $1
)
SELECT MIN(tile_id), MAX(tile_id) FROM RecentCommits
AS OF SYSTEM TIME '-0.1s'
`
	row := wh.DB.QueryRow(ctx, statement, wh.WindowSize)
	var lc pgtype.Int4
	var mc pgtype.Int4
	if err := row.Scan(&lc, &mc); err != nil {
		if err == pgx.ErrNoRows {
			return 0, 0, nil // not enough commits seen, so start at tile 0.
		}
		return 0, 0, skerr.Wrapf(err, "getting latest commit")
	}
	if lc.Status == pgtype.Null || mc.Status == pgtype.Null {
		// There are no commits with data, so start at tile 0.
		return 0, 0, nil
	}
	return schema.TileID(lc.Int), schema.TileID(mc.Int), nil
}

// getPositiveDigests returns all digests which are triaged as positive in the given tiles that
// belong to the provided grouping. These digests are split according to the traces that made them.
func (wh *Handlers) getPositiveDigests(ctx context.Context, beginTile, endTile schema.TileID, groupingID schema.GroupingID) (frontend.PositiveDigestsByGroupingIDResponse, error) {
	ctx, span := trace.StartSpan(ctx, "getPositiveDigests")
	defer span.End()

	tracesForGroup, err := wh.getTracesForGroup(ctx, groupingID)
	if err != nil {
		return frontend.PositiveDigestsByGroupingIDResponse{}, skerr.Wrap(err)
	}

	tilesInRange := make([]schema.TileID, 0, endTile-beginTile+1)
	for i := beginTile; i <= endTile; i++ {
		tilesInRange = append(tilesInRange, i)
	}
	// Querying traces, and then digests is much faster because the indexes can be used more
	// efficiently. kjlubick@ tried specifying INNER LOOKUP JOIN but that didn't work on v20.2.7
	const statement = `
WITH
DigestsOfInterest AS (
    SELECT DISTINCT digest, trace_id FROM TiledTraceDigests
    WHERE tile_id = ANY($1) AND trace_id = ANY($2)
)
SELECT encode(DigestsOfInterest.trace_id, 'hex'), encode(DigestsOfInterest.digest, 'hex') FROM DigestsOfInterest
JOIN Expectations ON grouping_id = $3 AND label = 'p' AND
  Expectations.digest = DigestsOfInterest.digest
ORDER BY 1, 2`

	rows, err := wh.DB.Query(ctx, statement, tilesInRange, tracesForGroup, groupingID)
	if err != nil {
		return frontend.PositiveDigestsByGroupingIDResponse{}, skerr.Wrapf(err, "fetching digests")
	}
	defer rows.Close()
	traceToDigests := make(map[string][]types.Digest, len(tracesForGroup))
	for rows.Next() {
		var d types.Digest
		var t string
		if err := rows.Scan(&t, &d); err != nil {
			return frontend.PositiveDigestsByGroupingIDResponse{}, skerr.Wrap(err)
		}
		traceToDigests[t] = append(traceToDigests[t], d)
	}

	rv := frontend.PositiveDigestsByGroupingIDResponse{}
	for traceID, digests := range traceToDigests {
		rv.Traces = append(rv.Traces, frontend.PositiveDigestsTraceInfo{
			TraceID:         traceID,
			PositiveDigests: digests,
		})
	}
	// Sort by trace ID for determinism - the digests should already be sorted
	// because of the SQL query.
	sort.Slice(rv.Traces, func(i, j int) bool {
		return rv.Traces[i].TraceID < rv.Traces[j].TraceID
	})
	return rv, nil
}

// getTracesForGroup returns all the traces that are a part of the specified grouping.
func (wh *Handlers) getTracesForGroup(ctx context.Context, id schema.GroupingID) ([]schema.TraceID, error) {
	ctx, span := trace.StartSpan(ctx, "getTracesForGroup")
	defer span.End()
	const statement = `SELECT trace_id FROM Traces
AS OF SYSTEM TIME '-0.1s'
WHERE grouping_id = $1`
	rows, err := wh.DB.Query(ctx, statement, id)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching trace ids")
	}
	defer rows.Close()
	var rv []schema.TraceID
	for rows.Next() {
		var t schema.TraceID
		if err := rows.Scan(&t); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, t)
	}
	return rv, nil
}
