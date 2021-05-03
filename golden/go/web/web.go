package web

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search"
	search_fe "go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/search2"
	search2_fe "go.skia.org/infra/golden/go/search2/frontend"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/status"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tilesource"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/tjstore"
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
)

type validateFields int

const (
	// FullFrontEnd means all fields should be set
	FullFrontEnd validateFields = iota
	// BaselineSubset means just the fields needed for Baseline Server should be set.
	BaselineSubset
)

// HandlersConfig holds the environment needed by the various http handler functions.
type HandlersConfig struct {
	Baseliner         baseline.BaselineFetcher
	DB                *pgxpool.Pool
	ExpectationsStore expectations.Store
	GCSClient         storage.GCSClient
	IgnoreStore       ignore.Store
	Indexer           indexer.IndexSource
	ReviewSystems     []clstore.ReviewSystem
	SearchAPI         search.SearchAPI
	Search2API        search2.API
	StatusWatcher     *status.StatusWatcher
	TileSource        tilesource.TileSource
	TryJobStore       tjstore.Store
}

// Handlers represents all the handlers (e.g. JSON endpoints) of Gold.
// It should be created by clients using NewHandlers.
type Handlers struct {
	HandlersConfig

	anonymousExpensiveQuota *rate.Limiter
	anonymousCheapQuota     *rate.Limiter
	anonymousGerritQuota    *rate.Limiter

	clSummaryCache *lru.Cache

	// These can be set for unit tests to simplify the testing.
	testingAuthAs string
}

// NewHandlers returns a new instance of Handlers.
func NewHandlers(conf HandlersConfig, val validateFields) (*Handlers, error) {
	// These fields are required by all types.
	if conf.Baseliner == nil {
		return nil, skerr.Fmt("Baseliner cannot be nil")
	}
	if len(conf.ReviewSystems) == 0 {
		return nil, skerr.Fmt("ReviewSystems cannot be empty")
	}
	if conf.GCSClient == nil {
		return nil, skerr.Fmt("GCSClient cannot be nil")
	}

	if val == FullFrontEnd {
		if conf.DB == nil {
			return nil, skerr.Fmt("DB cannot be nil")
		}
		if conf.ExpectationsStore == nil {
			return nil, skerr.Fmt("ExpectationsStore cannot be nil")
		}
		if conf.IgnoreStore == nil {
			return nil, skerr.Fmt("IgnoreStore cannot be nil")
		}
		if conf.Indexer == nil {
			return nil, skerr.Fmt("Indexer cannot be nil")
		}
		if conf.SearchAPI == nil {
			return nil, skerr.Fmt("SearchAPI cannot be nil")
		}
		if conf.Search2API == nil {
			return nil, skerr.Fmt("Search2API cannot be nil")
		}
		if conf.StatusWatcher == nil {
			return nil, skerr.Fmt("StatusWatcher cannot be nil")
		}
		if conf.TileSource == nil {
			return nil, skerr.Fmt("TileSource cannot be nil")
		}
		if conf.TryJobStore == nil {
			return nil, skerr.Fmt("TryJobStore cannot be nil")
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

// TODO(stephana): once the byBlameHandler is removed, refactor this to
// remove the redundant types ByBlameEntry and ByBlame.

// ByBlameHandler returns a json object with the digests to be triaged grouped by blamelist.
func (wh *Handlers) ByBlameHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

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
	}

	blameEntries, err := wh.computeByBlame(r.Context(), corpus)
	if err != nil {
		httputils.ReportError(w, err, "could not compute blames", http.StatusInternalServerError)
		return
	}

	// Wrap the result in an object because we don't want to return a JSON array.
	sendJSONResponse(w, frontend.ByBlameResponse{Data: blameEntries})
}

// computeByBlame creates several ByBlameEntry structs based on the state
// of HEAD and returns them in a slice, for use by the frontend.
func (wh *Handlers) computeByBlame(ctx context.Context, corpus string) ([]frontend.ByBlameEntry, error) {
	idx := wh.Indexer.GetIndex()
	// At this point query contains at least a corpus.
	untriagedSummaries, err := idx.SummarizeByGrouping(ctx, corpus, nil, types.ExcludeIgnoredTraces, true)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get summaries for corpus %q", corpus)
	}
	commits := idx.Tile().DataCommits()

	// This is a very simple grouping of digests, for every digest we look up the
	// blame list for that digest and then use the concatenated git hashes as a
	// group id. All of the digests are then grouped by their group id.

	// Collects a ByBlame for each untriaged digest, keyed by group id.
	grouped := map[string][]frontend.ByBlame{}

	// The Commit info for each group id.
	commitinfo := map[string][]tiling.Commit{}
	// map [groupid] [test] TestRollup
	rollups := map[string]map[types.TestName]frontend.TestRollup{}

	for _, s := range untriagedSummaries {
		test := s.Name
		for _, d := range s.UntHashes {
			dist := idx.GetBlame(test, d, commits)
			if dist.IsEmpty() {
				// Should only happen if the index isn't quite ready being prepared.
				// Since we wait until the index is created before exposing the web
				// server, this should never happen.
				sklog.Warningf("empty blame for %s %s", test, d)
				continue
			}
			groupid := strings.Join(lookUpCommits(dist.Freq, commits), ":")
			// Only fill in commitinfo for each groupid only once.
			if _, ok := commitinfo[groupid]; !ok {
				var blameCommits []tiling.Commit
				for _, index := range dist.Freq {
					blameCommits = append(blameCommits, commits[index])
				}
				sort.Slice(blameCommits, func(i, j int) bool {
					return blameCommits[i].CommitTime.After(blameCommits[j].CommitTime)
				})
				commitinfo[groupid] = blameCommits
			}
			// Construct a ByBlame and add it to grouped.
			value := frontend.ByBlame{
				Test:          test,
				Digest:        d,
				Blame:         dist,
				CommitIndices: dist.Freq,
			}
			if _, ok := grouped[groupid]; !ok {
				grouped[groupid] = []frontend.ByBlame{value}
			} else {
				grouped[groupid] = append(grouped[groupid], value)
			}
			if _, ok := rollups[groupid]; !ok {
				rollups[groupid] = map[types.TestName]frontend.TestRollup{}
			}
			// Calculate the rollups.
			r, ok := rollups[groupid][test]
			if !ok {
				r = frontend.TestRollup{
					Test:         test,
					Num:          0,
					SampleDigest: d,
				}
			}
			r.Num += 1
			rollups[groupid][test] = r
		}
	}

	// Assemble the response.
	blameEntries := make([]frontend.ByBlameEntry, 0, len(grouped))
	for groupid, byBlames := range grouped {
		rollup := rollups[groupid]
		nTests := len(rollup)
		var affectedTests []frontend.TestRollup

		// Only include the affected tests if there are no more than 10 of them.
		if nTests <= 10 {
			affectedTests = make([]frontend.TestRollup, 0, nTests)
			for _, testInfo := range rollup {
				affectedTests = append(affectedTests, testInfo)
			}
			sort.Slice(affectedTests, func(i, j int) bool {
				// Put the highest amount of digests first
				return affectedTests[i].Num > affectedTests[j].Num ||
					// Break ties based on test name (for determinism).
					(affectedTests[i].Num == affectedTests[j].Num && affectedTests[i].Test < affectedTests[j].Test)
			})
		}

		blameEntries = append(blameEntries, frontend.ByBlameEntry{
			GroupID:       groupid,
			NDigests:      len(byBlames),
			NTests:        nTests,
			AffectedTests: affectedTests,
			Commits:       frontend.FromTilingCommits(commitinfo[groupid]),
		})
	}
	sort.Slice(blameEntries, func(i, j int) bool {
		return blameEntries[i].NDigests > blameEntries[j].NDigests ||
			// For test determinism, use GroupID as a tie-breaker
			(blameEntries[i].NDigests == blameEntries[j].NDigests && blameEntries[i].GroupID < blameEntries[j].GroupID)
	})

	return blameEntries, nil
}

// lookUpCommits returns the commit hashes for the commit indices in 'freq'.
func lookUpCommits(freq []int, commits []tiling.Commit) []string {
	var ret []string
	for _, index := range freq {
		ret = append(ret, commits[index].Hash)
	}
	return ret
}

// ChangelistsHandler returns the list of code_review.Changelists that have
// uploaded results to Gold (via TryJobs).
func (wh *Handlers) ChangelistsHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.limitForAnonUsers(r); err != nil {
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
	cls, pagination, err := wh.getIngestedChangelists(r.Context(), offset, size, activeOnly)

	if err != nil {
		httputils.ReportError(w, err, "Retrieving changelists results failed.", http.StatusInternalServerError)
		return
	}

	response := frontend.ChangelistsResponse{
		Changelists:        cls,
		ResponsePagination: *pagination,
	}

	sendJSONResponse(w, response)
}

// getIngestedChangelists performs the core of the logic for ChangelistsHandler,
// by fetching N Changelists given an offset.
func (wh *Handlers) getIngestedChangelists(ctx context.Context, offset, size int, activeOnly bool) ([]frontend.Changelist, *httputils.ResponsePagination, error) {
	so := clstore.SearchOptions{
		StartIdx: offset,
		Limit:    size,
	}
	if activeOnly {
		so.OpenCLsOnly = true
	}

	grandTotal := 0
	var retCls []frontend.Changelist
	for _, system := range wh.ReviewSystems {
		cls, total, err := system.Store.GetChangelists(ctx, so)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "fetching Changelists from [%d:%d)", offset, offset+size)
		}

		for _, cl := range cls {
			retCls = append(retCls, frontend.ConvertChangelist(cl, system.ID, system.URLTemplate))
		}
		if grandTotal == clstore.CountMany || total == clstore.CountMany {
			grandTotal = clstore.CountMany
		} else {
			grandTotal += total
		}
	}

	pagination := &httputils.ResponsePagination{
		Offset: offset,
		Size:   size,
		Total:  grandTotal,
	}
	return retCls, pagination, nil
}

// PatchsetsAndTryjobsForCL returns a summary of the data we have collected
// for a given Changelist, specifically any TryJobs that have uploaded data
// to Gold belonging to various patchsets in it.
func (wh *Handlers) PatchsetsAndTryjobsForCL(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.limitForAnonUsers(r); err != nil {
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

	rv, err := wh.getCLSummary(r.Context(), system, clID)
	if err != nil {
		httputils.ReportError(w, err, "could not retrieve data for the specified CL.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, rv)
}

// A list of CI systems we support. So far, the mapping of task ID to link is project agnostic. If
// that stops being the case, then we'll need to supply this mapping on a per-instance basis.
var cisTemplates = map[string]string{
	"cirrus":      "https://cirrus-ci.com/task/%s",
	"buildbucket": "https://cr-buildbucket.appspot.com/build/%s",
}

// getCLSummary does a bulk of the work for PatchsetsAndTryjobsForCL, specifically
// fetching the Changelist and Patchsets from clstore and any associated TryJobs from
// the tjstore.
func (wh *Handlers) getCLSummary(ctx context.Context, system clstore.ReviewSystem, clID string) (frontend.ChangelistSummary, error) {
	cl, err := system.Store.GetChangelist(ctx, clID)
	if err != nil {
		return frontend.ChangelistSummary{}, skerr.Wrapf(err, "getting CL %s", clID)
	}

	// We know xps is sorted by order, if it is non-nil
	xps, err := system.Store.GetPatchsets(ctx, clID)
	if err != nil {
		return frontend.ChangelistSummary{}, skerr.Wrapf(err, "getting Patchsets for CL %s", clID)
	}

	var patchsets []frontend.Patchset
	maxOrder := 0

	// TODO(kjlubick): maybe fetch these in parallel (with errgroup)
	for _, ps := range xps {
		if ps.Order > maxOrder {
			maxOrder = ps.Order
		}
		psID := tjstore.CombinedPSID{
			CL:  clID,
			CRS: system.ID,
			PS:  ps.SystemID,
		}
		xtj, err := wh.TryJobStore.GetTryJobs(ctx, psID)
		if err != nil {
			return frontend.ChangelistSummary{}, skerr.Wrapf(err, "getting TryJobs for CL %s - PS %s", clID, ps.SystemID)
		}
		var tryjobs []frontend.TryJob
		for _, tj := range xtj {
			templ := cisTemplates[tj.System]
			tryjobs = append(tryjobs, frontend.ConvertTryJob(tj, templ))
		}

		patchsets = append(patchsets, frontend.Patchset{
			SystemID: ps.SystemID,
			Order:    ps.Order,
			TryJobs:  tryjobs,
		})
	}

	return frontend.ChangelistSummary{
		CL:                frontend.ConvertChangelist(cl, system.ID, system.URLTemplate),
		Patchsets:         patchsets,
		NumTotalPatchsets: maxOrder,
	}, nil
}

// ChangelistUntriagedHandler writes out a list of untriaged digests uploaded by this CL that
// are not on master already and are not ignored.
func (wh *Handlers) ChangelistUntriagedHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.cheapLimitForGerritPlugin(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	requestVars := mux.Vars(r)
	clID, ok := requestVars["id"]
	if !ok {
		http.Error(w, "Must specify 'id' of Changelist.", http.StatusBadRequest)
		return
	}
	psID, ok := requestVars["patchset"]
	if !ok {
		http.Error(w, "Must specify 'patchset' of Changelist.", http.StatusBadRequest)
		return
	}
	crs, ok := requestVars["system"]
	if !ok {
		http.Error(w, "Must specify 'system' of Changelist.", http.StatusBadRequest)
		return
	}

	id := tjstore.CombinedPSID{
		CL:  clID,
		CRS: crs,
		PS:  psID,
	}
	dl, err := wh.SearchAPI.UntriagedUnignoredTryJobExclusiveDigests(r.Context(), id)
	if err != nil {
		sklog.Warningf("Could not get untriaged digests for %v - possibly this CL/PS has none or is too old to be indexed: %s", id, err)
		// Errors can trip up the Gerrit Plugin (at least until skbug/10706 is resolved).
		sendJSONResponse(w, search_fe.UntriagedDigestList{TS: time.Now()})
		return
	}
	sendJSONResponse(w, dl)
}

// SearchHandler is the endpoint for all searches, including accessing
// results that belong to a tryjob.  It times out after 3 minutes, to prevent outstanding requests
// from growing unbounded.
func (wh *Handlers) SearchHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
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
	ctx, span := trace.StartSpan(ctx, "SearchHandler")
	defer span.End()

	searchResponse, err := wh.SearchAPI.Search(ctx, q)
	if err != nil {
		httputils.ReportError(w, err, "Search for digests failed.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, searchResponse)
}

// SearchHandler2 searches the data in the new SQL backend. It times out after 3 minutes, to prevent
// outstanding requests from growing unbounded.
func (wh *Handlers) SearchHandler2(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
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
	ctx, span := trace.StartSpan(ctx, "web_SearchHandler2", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()

	searchResponse, err := wh.Search2API.Search(ctx, q)
	if err != nil {
		httputils.ReportError(w, err, "Search for digests failed in the SQL backend.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, searchResponse)
}

// parseSearchQuery extracts the search query from request.
func parseSearchQuery(w http.ResponseWriter, r *http.Request) (*query.Search, bool) {
	q := query.Search{Limit: 50}
	if err := query.ParseSearch(r, &q); err != nil {
		httputils.ReportError(w, err, "Search for digests failed.", http.StatusInternalServerError)
		return nil, false
	}
	return &q, true
}

// DetailsHandler returns the details about a single digest.
func (wh *Handlers) DetailsHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	// Extract: test, digest, issue
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, err, "Failed to parse form values", http.StatusInternalServerError)
		return
	}
	test := r.Form.Get("test")
	digest := r.Form.Get("digest")
	if test == "" || !validation.IsValidDigest(digest) {
		http.Error(w, "Some query parameters are wrong or missing", http.StatusBadRequest)
		return
	}
	clID := r.Form.Get("changelist_id")
	crs := r.Form.Get("crs")
	if clID != "" {
		if _, ok := wh.getCodeReviewSystem(crs); !ok {
			http.Error(w, "Invalid Code Review System; did you include crs?", http.StatusBadRequest)
			return
		}
	} else {
		crs = ""
	}

	ret, err := wh.SearchAPI.GetDigestDetails(r.Context(), types.TestName(test), types.Digest(digest), clID, crs)
	if err != nil {
		httputils.ReportError(w, err, "Unable to get digest details.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, ret)
}

// DiffHandler returns difference between two digests.
func (wh *Handlers) DiffHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	// Extract: test, left, right where left and right are digests.
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, err, "Failed to parse form values", http.StatusInternalServerError)
		return
	}
	// TODO(kjlubick) require corpus
	test := r.Form.Get("test")
	left := r.Form.Get("left")
	right := r.Form.Get("right")
	if test == "" || !validation.IsValidDigest(left) || !validation.IsValidDigest(right) {
		sklog.Debugf("Bad query params: %q %q %q", test, left, right)
		http.Error(w, "invalid query params", http.StatusBadRequest)
		return
	}
	clID := r.Form.Get("changelist_id")
	crs := r.Form.Get("crs")
	if clID != "" {
		if _, ok := wh.getCodeReviewSystem(crs); !ok {
			http.Error(w, "Invalid Code Review System; did you include crs?", http.StatusBadRequest)
			return
		}
	} else {
		crs = ""
	}

	ret, err := wh.SearchAPI.DiffDigests(r.Context(), types.TestName(test), types.Digest(left), types.Digest(right), clID, crs)
	if err != nil {
		httputils.ReportError(w, err, "Unable to compare digests", http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, ret)
}

// ListIgnoreRules returns the current ignore rules in JSON format.
func (wh *Handlers) ListIgnoreRules(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()

	_, includeCounts := r.URL.Query()["counts"]
	// Counting can be expensive, since it goes through every trace.
	if includeCounts {
		if err := wh.limitForAnonUsers(r); err != nil {
			httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
			return
		}
	} else if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	ignores, err := wh.getIgnores(r.Context(), includeCounts)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve ignore rules, there may be none.", http.StatusInternalServerError)
		return
	}

	response := frontend.IgnoresResponse{
		Rules: ignores,
	}

	sendJSONResponse(w, response)
}

// getIgnores fetches the ignores from the store and optionally counts how many
// times they are applied.
func (wh *Handlers) getIgnores(ctx context.Context, withCounts bool) ([]frontend.IgnoreRule, error) {
	rules, err := wh.IgnoreStore.List(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching ignores from store")
	}

	// We want to make a slice of pointers because addIgnoreCounts will add the counts in-place.
	ret := make([]frontend.IgnoreRule, 0, len(rules))
	for _, r := range rules {
		fr, err := frontend.ConvertIgnoreRule(r)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		ret = append(ret, fr)
	}

	if withCounts {
		// addIgnoreCounts updates the values of ret directly
		if err := wh.addIgnoreCounts(ctx, ret); err != nil {
			return nil, skerr.Wrapf(err, "adding ignore counts to %d rules", len(ret))
		}
	}

	return ret, nil
}

// addIgnoreCounts goes through the whole tile and counts how many traces each of the rules
// applies to. This uses the most recent index, so there may be some discrepancies in the counts
// if a new rule has been added since the last index was computed.
func (wh *Handlers) addIgnoreCounts(ctx context.Context, rules []frontend.IgnoreRule) error {
	defer metrics2.FuncTimer().Stop()
	sklog.Debugf("adding counts to %d rules", len(rules))

	exp, err := wh.ExpectationsStore.Get(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	// Go through every trace and look for only those that are ignored. Then, count how many
	// rules apply to a given ignored trace.
	idx := wh.Indexer.GetIndex()
	nonIgnoredTraces := idx.DigestCountsByTrace(types.ExcludeIgnoredTraces)
	traces := idx.SlicedTraces(types.IncludeIgnoredTraces, nil)
	const numShards = 32
	chunkSize := len(traces) / numShards
	// Very small shards are likely not worth the overhead.
	if chunkSize < 50 {
		chunkSize = 50
	}
	// This mutex protects the passed in rules array and allows the final step of each
	// of the goroutines below to be done safely in parallel to add each shard's results
	// to the total.
	var mutex sync.Mutex
	err = util.ChunkIterParallel(ctx, len(traces), chunkSize, func(ctx context.Context, start, stop int) error {
		type counts struct {
			Count                   int
			UntriagedCount          int
			ExclusiveCount          int
			ExclusiveUntriagedCount int
		}
		ruleCounts := make([]counts, len(rules))
		for _, tp := range traces[start:stop] {
			if err := ctx.Err(); err != nil {
				return skerr.Wrap(err)
			}
			id, tr := tp.ID, tp.Trace
			if _, ok := nonIgnoredTraces[id]; ok {
				// This wasn't ignored, so we can skip having to count it
				continue
			}
			idxMatched := -1
			untIdxMatched := -1
			numMatched := 0
			untMatched := 0
			for i, r := range rules {
				if tr.Matches(r.ParsedQuery) {
					numMatched++
					ruleCounts[i].Count++
					idxMatched = i

					// Check to see if the digest is untriaged at head
					if d := tr.AtHead(); d != tiling.MissingDigest && exp.Classification(tr.TestName(), d) == expectations.Untriaged {
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
		mutex.Lock()
		defer mutex.Unlock()
		for i := range rules {
			(&rules[i]).Count += ruleCounts[i].Count
			(&rules[i]).UntriagedCount += ruleCounts[i].UntriagedCount
			(&rules[i]).ExclusiveCount += ruleCounts[i].ExclusiveCount
			(&rules[i]).ExclusiveUntriagedCount += ruleCounts[i].ExclusiveUntriagedCount
		}
		return nil
	})
	return skerr.Wrap(err)
}

// UpdateIgnoreRule updates an existing ignores rule.
func (wh *Handlers) UpdateIgnoreRule(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
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
	ts := now.Now(r.Context())
	ignoreRule := ignore.NewRule(user, ts.Add(expiresInterval), irb.Filter, irb.Note)
	ignoreRule.ID = id
	if err := wh.IgnoreStore.Update(r.Context(), ignoreRule); err != nil {
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
	defer metrics2.FuncTimer().Stop()
	user := wh.loggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to delete an ignore rule", http.StatusUnauthorized)
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		http.Error(w, "ID must be non-empty.", http.StatusBadRequest)
		return
	}

	if err := wh.IgnoreStore.Delete(r.Context(), id); err != nil {
		httputils.ReportError(w, err, "Unable to delete ignore rule", http.StatusInternalServerError)
		return
	}
	sklog.Infof("Successfully deleted ignore with id %s", id)
	sendJSONResponse(w, map[string]string{"deleted": "true"})
}

// AddIgnoreRule is for adding a new ignore rule.
func (wh *Handlers) AddIgnoreRule(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	user := wh.loggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to add an ignore rule", http.StatusUnauthorized)
		return
	}

	expiresInterval, irb, err := getValidatedIgnoreRule(r)
	if err != nil {
		httputils.ReportError(w, err, "invalid ignore rule input", http.StatusBadRequest)
		return
	}
	ts := now.Now(r.Context())
	ignoreRule := ignore.NewRule(user, ts.Add(expiresInterval), irb.Filter, irb.Note)
	if err := wh.IgnoreStore.Create(r.Context(), ignoreRule); err != nil {
		httputils.ReportError(w, err, "Failed to create ignore rule", http.StatusInternalServerError)
		return
	}

	sklog.Infof("Successfully added ignore from %s", user)
	sendJSONResponse(w, map[string]string{"added": "true"})
}

// TriageHandler handles a request to change the triage status of one or more
// digests of one test.
//
// It accepts a POST'd JSON serialization of TriageRequest and updates
// the expectations.
func (wh *Handlers) TriageHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	user := login.LoggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to triage.", http.StatusUnauthorized)
		return
	}

	req := frontend.TriageRequest{}
	if err := parseJSON(r, &req); err != nil {
		httputils.ReportError(w, err, "Failed to parse JSON request.", http.StatusBadRequest)
		return
	}
	sklog.Infof("Triage request: %#v", req)

	if err := wh.triage(r.Context(), user, req); err != nil {
		httputils.ReportError(w, err, "Could not triage", http.StatusInternalServerError)
		return
	}
	// Nothing to return, so just set 200
	w.WriteHeader(http.StatusOK)
}

// triage processes the given TriageRequest.
func (wh *Handlers) triage(ctx context.Context, user string, req frontend.TriageRequest) error {
	// TODO(kjlubick) remove the legacy check for "0" when the frontend no longer sends it.
	if req.ChangelistID != "" && req.ChangelistID != "0" {
		if req.CodeReviewSystem == "" {
			// TODO(kjlubick) remove this default after the search page is converted to lit-html.
			req.CodeReviewSystem = wh.ReviewSystems[0].ID
		}
		if _, ok := wh.getCodeReviewSystem(req.CodeReviewSystem); !ok {
			return skerr.Fmt("Unknown Code Review System; did you remember to include crs?")
		}
	} else {
		req.CodeReviewSystem = ""
	}

	// Build the expectations change request from the list of digests passed in.
	tc := make([]expectations.Delta, 0, len(req.TestDigestStatus))
	for test, digests := range req.TestDigestStatus {
		for d, label := range digests {
			if label == "" {
				// Empty string means the frontend didn't have a closest digest to use when making a
				// "bulk triage to the closest digest" request. It's easier to catch this on the server
				// side than make the JS check for empty string and mutate the POST body.
				continue
			}
			if !expectations.ValidLabel(label) {
				return skerr.Fmt("invalid label %q in triage request", label)
			}
			tc = append(tc, expectations.Delta{
				Grouping: test,
				Digest:   d,
				Label:    label,
			})
		}
	}

	// Use the expectations store for the master branch, unless an issue was given
	// in the request, then get the expectations store for the issue.
	expStore := wh.ExpectationsStore
	// TODO(kjlubick) remove the legacy check here after the frontend bakes in.
	if req.ChangelistID != "" && req.ChangelistID != "0" {
		expStore = wh.ExpectationsStore.ForChangelist(req.ChangelistID, req.CodeReviewSystem)
	}

	// If set, use the image matching algorithm's name as the author of this change.
	if req.ImageMatchingAlgorithm != "" {
		user = req.ImageMatchingAlgorithm
	}

	// Add the change.
	if err := expStore.AddChange(ctx, tc, user); err != nil {
		return skerr.Wrapf(err, "Failed to store the updated expectations.")
	}
	return nil
}

// StatusHandler returns the current status of with respect to HEAD.
func (wh *Handlers) StatusHandler(w http.ResponseWriter, _ *http.Request) {
	defer metrics2.FuncTimer().Stop()

	// This should be an incredibly cheap call and therefore does not count against any quota.
	sendJSONResponse(w, wh.StatusWatcher.GetStatus())
}

// ClusterDiffHandler calculates the NxN diffs of all the digests that match
// the incoming query and returns the data in a format appropriate for
// handling in d3.
func (wh *Handlers) ClusterDiffHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	// Extract the test name as we only allow clustering within a test.
	q := query.Search{Limit: 50}
	if err := query.ParseSearch(r, &q); err != nil {
		httputils.ReportError(w, err, "Unable to parse query parameter.", http.StatusBadRequest)
		return
	}
	testNames := q.TraceValues[types.PrimaryKeyField]
	if len(testNames) == 0 {
		http.Error(w, "No test name provided.", http.StatusBadRequest)
		return
	}
	testName := testNames[0]
	ctx := r.Context()
	ctx, span := trace.StartSpan(ctx, "ClusterDiff_sql")
	defer span.End()

	idx := wh.Indexer.GetIndex()
	searchResponse, err := wh.SearchAPI.Search(ctx, &q)
	if err != nil {
		httputils.ReportError(w, err, "Search for digests failed.", http.StatusInternalServerError)
		return
	}

	// TODO(kjlubick): Check if we need to sort these
	// Sort the digests so they are displayed with untriaged last, which means
	// they will be displayed 'on top', because in SVG document order is z-order.

	digests := types.DigestSlice{}
	for _, digest := range searchResponse.Results {
		digests = append(digests, digest.Digest)
	}

	digestIndex := map[types.Digest]int{}
	for i, d := range digests {
		digestIndex[d] = i
	}

	d3 := ClusterDiffResult{
		Test:             types.TestName(testName),
		Nodes:            []Node{},
		Links:            []Link{},
		ParamsetByDigest: map[types.Digest]paramtools.ParamSet{},
		ParamsetsUnion:   paramtools.ParamSet{},
	}
	for i, d := range searchResponse.Results {
		d3.Nodes = append(d3.Nodes, Node{
			Name:   d.Digest,
			Status: d.Status,
		})
		remaining := digests[i:]
		links, err := wh.getLinksBetween(r.Context(), d.Digest, remaining)
		if err != nil {
			httputils.ReportError(w, err, "could not compute diff metrics", http.StatusInternalServerError)
			return
		}
		for otherDigest, distance := range links {
			d3.Links = append(d3.Links, Link{
				Source: digestIndex[d.Digest],
				Target: digestIndex[otherDigest],
				Value:  distance,
			})
		}
		d3.ParamsetByDigest[d.Digest] = idx.GetParamsetSummary(d.Test, d.Digest, types.ExcludeIgnoredTraces)
		for _, p := range d3.ParamsetByDigest[d.Digest] {
			sort.Strings(p)
		}
		d3.ParamsetsUnion.AddParamSet(d3.ParamsetByDigest[d.Digest])
	}

	for _, p := range d3.ParamsetsUnion {
		sort.Strings(p)
	}

	sendJSONResponse(w, d3)
}

// getLinksBetween queries the SQL DB for the PercentPixelsDiff between the left digest and
// the right digests. It returns them in a map.
func (wh *Handlers) getLinksBetween(ctx context.Context, left types.Digest, right types.DigestSlice) (map[types.Digest]float32, error) {
	ctx, span := trace.StartSpan(ctx, "getLinksBetween")
	span.AddAttributes(trace.Int64Attribute("num_right", int64(len(right))))
	defer span.End()
	const statement = `
SELECT encode(right_digest, 'hex'), percent_pixels_diff FROM DiffMetrics
AS OF SYSTEM TIME '-0.1s'
WHERE left_digest = $1 AND right_digest IN `
	arguments := make([]interface{}, 0, len(right)+1)
	lb, err := sql.DigestToBytes(left)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	arguments = append(arguments, lb)
	for _, r := range right {
		rb, err := sql.DigestToBytes(r)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		arguments = append(arguments, rb)
	}
	vp := sql.ValuesPlaceholders(len(arguments), 1)
	rows, err := wh.DB.Query(ctx, statement+vp, arguments...)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	rv := map[types.Digest]float32{}
	for rows.Next() {
		var rightD types.Digest
		var linkDistance float32
		if err := rows.Scan(&rightD, &linkDistance); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv[rightD] = linkDistance
	}
	return rv, nil
}

// Node represents a single node in a d3 diagram. Used in ClusterDiffResult.
type Node struct {
	Name   types.Digest       `json:"name"`
	Status expectations.Label `json:"status"`
}

// Link represents a link between d3 nodes, used in ClusterDiffResult.
type Link struct {
	Source int     `json:"source"`
	Target int     `json:"target"`
	Value  float32 `json:"value"`
}

// ClusterDiffResult contains the result of comparing all digests within a test.
// It is structured to be easy to render by the D3.js.
type ClusterDiffResult struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`

	Test             types.TestName                       `json:"test"`
	ParamsetByDigest map[types.Digest]paramtools.ParamSet `json:"paramsetByDigest"`
	ParamsetsUnion   paramtools.ParamSet                  `json:"paramsetsUnion"`
}

// ListTestsHandler returns a summary of the digests seen for a given test.
func (wh *Handlers) ListTestsHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
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

	idx := wh.Indexer.GetIndex()
	summaries, err := idx.SummarizeByGrouping(r.Context(), q.Corpus, q.TraceValues, q.IgnoreState, q.OnlyIncludeDigestsProducedAtHead)
	if err != nil {
		httputils.ReportError(w, err, "Could not compute query.", http.StatusInternalServerError)
		return
	}
	// We explicitly want a zero-length slice instead of a nil slice because the latter serializes
	// to JSON as null instead of []
	tests := make([]frontend.TestSummary, 0, len(summaries))
	for _, s := range summaries {
		if s != nil {
			tests = append(tests, frontend.TestSummary{
				Name:             s.Name,
				PositiveDigests:  s.Pos,
				NegativeDigests:  s.Neg,
				UntriagedDigests: s.Untriaged,
			})
		}
	}
	// For determinism, sort by test name. The client will have the power to sort these differently.
	sort.Slice(tests, func(i, j int) bool {
		return tests[i].Name < tests[j].Name
	})
	// Outputs: []frontend.TestSummary
	//   Frontend will have option to hide tests with no digests.
	sendJSONResponse(w, tests)
}

// TriageLogHandler returns the entries in the triagelog paginated
// in reverse chronological order.
func (wh *Handlers) TriageLogHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.limitForAnonUsers(r); err != nil {
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

	details := q.Get("details") == "true"
	logEntries, total, err := wh.getTriageLog(r.Context(), crs, clID, offset, size, details)

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

// getTriageLog does the actual work of the TriageLogHandler, but is easier to test.
func (wh *Handlers) getTriageLog(ctx context.Context, crs, changelistID string, offset, size int, withDetails bool) ([]frontend.TriageLogEntry, int, error) {
	expStore := wh.ExpectationsStore
	// TODO(kjlubick) remove this legacy handler
	if changelistID != "" && changelistID != "0" {
		expStore = wh.ExpectationsStore.ForChangelist(changelistID, crs)
	}
	entries, total, err := expStore.QueryLog(ctx, offset, size, withDetails)
	if err != nil {
		return nil, -1, skerr.Wrap(err)
	}
	logEntries := make([]frontend.TriageLogEntry, 0, len(entries))
	for _, e := range entries {
		logEntries = append(logEntries, frontend.ConvertLogEntry(e))
	}
	return logEntries, total, nil
}

// TriageUndoHandler performs an "undo" for a given change id.
// The change id's are returned in the result of jsonTriageLogHandler.
// It accepts one query parameter 'id' which is the id if the change
// that should be reversed.
// If successful it returns the same result as a call to jsonTriageLogHandler
// to reflect the changed triagelog.
// TODO(kjlubick): This does not properly handle undoing of ChangelistExpectations.
func (wh *Handlers) TriageUndoHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	// Get the user and make sure they are logged in.
	user := login.LoggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to change expectations", http.StatusUnauthorized)
		return
	}

	// Extract the id to undo.
	changeID := r.URL.Query().Get("id")

	// Do the undo procedure.
	if err := wh.ExpectationsStore.UndoChange(r.Context(), changeID, user); err != nil {
		httputils.ReportError(w, err, "Unable to undo.", http.StatusInternalServerError)
		return
	}

	// Send the same response as a query for the first page.
	wh.TriageLogHandler(w, r)
}

// ParamsHandler returns the union of all parameters.
func (wh *Handlers) ParamsHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, err, "Invalid form headers", http.StatusBadRequest)
		return
	}
	clID := r.Form.Get("changelist_id")
	crs := r.Form.Get("crs")
	if clID != "" {
		if crs == "" {
			// TODO(kjlubick) remove this default after the search page is converted to lit-html.
			crs = wh.ReviewSystems[0].ID
		}
		if _, ok := wh.getCodeReviewSystem(crs); !ok {
			http.Error(w, "Invalid Code Review System; did you include crs?", http.StatusBadRequest)
			return
		}
	} else {
		crs = ""
	}

	if clID != "" {
		clIdx := wh.Indexer.GetIndexForCL(crs, clID)
		if clIdx != nil {
			sendJSONResponse(w, clIdx.ParamSet)
			return
		}
		// Fallback to master branch
	}

	tile := wh.Indexer.GetIndex().Tile().GetTile(types.IncludeIgnoredTraces)
	sendJSONResponse(w, tile.ParamSet)
}

// CommitsHandler returns the commits from the most recent tile.
func (wh *Handlers) CommitsHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	cpxTile := wh.TileSource.GetTile()
	if cpxTile == nil {
		httputils.ReportError(w, nil, "Not loaded yet - try back later", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(frontend.FromTilingCommits(cpxTile.DataCommits())); err != nil {
		sklog.Errorf("Failed to write or encode result: %s", err)
	}
}

// TextKnownHashesProxy returns known hashes that have been written to GCS in the background
// Each line contains a single digest for an image. Bots will then only upload images which
// have a hash not found on this list, avoiding significant amounts of unnecessary uploads.
func (wh *Handlers) TextKnownHashesProxy(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	// No limit for anon users - this is an endpoint backed up by baseline servers, and
	// should be able to handle a large load.

	w.Header().Set("Content-Type", "text/plain")
	if err := wh.GCSClient.LoadKnownDigests(r.Context(), w); err != nil {
		sklog.Errorf("Failed to copy the known hashes from GCS: %s", err)
		return
	}
}

// BaselineHandlerV1 differs from BaselineHandlerV2 in that the "primary" field in the JSON response
// is named "master_str".
//
// TODO(lovisolo): Remove this after all clients have been migrated to the V2 of this RPC.
func (wh *Handlers) BaselineHandlerV1(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	// No limit for anon users - this is an endpoint backed up by baseline servers, and
	// should be able to handle a large load.

	// Track usage of the legacy /json/expectations/commit/{commit_hash} route.
	if _, ok := mux.Vars(r)["commit_hash"]; ok {
		metrics2.GetCounter("gold_baselinehandler_route_legacy").Inc(1)
	} else {
		metrics2.GetCounter("gold_baselinehandler_route_new").Inc(1)
	}

	q := r.URL.Query()
	clID := q.Get("issue")
	issueOnly := q.Get("issueOnly") == "true"
	crs := q.Get("crs")

	if clID != "" {
		if crs == "" {
			// TODO(kjlubick) remove this default after the search page is converted to lit-html.
			crs = wh.ReviewSystems[0].ID
		}
		if _, ok := wh.getCodeReviewSystem(crs); !ok {
			http.Error(w, "Invalid CRS provided.", http.StatusBadRequest)
			return
		}
	} else {
		crs = ""
	}

	bl, err := wh.Baseliner.FetchBaseline(r.Context(), clID, crs, issueOnly)
	if err != nil {
		httputils.ReportError(w, err, "Fetching baselines failed.", http.StatusInternalServerError)
		return
	}
	bl.Expectations = expectations.Baseline{}
	sendJSONResponse(w, bl)
}

// BaselineHandlerV2 returns a JSON representation of that baseline including
// baselines for a options issue. It can respond to requests like these:
//
//    /json/expectations
//    /json/expectations?issue=123456
//    /json/expectations?issue=123456&issueOnly=true
//
// The "issue" parameter indicates the changelist ID for which we would like to
// retrieve the baseline. In that case the returned options will be a blend of
// the master baseline and the baseline defined for the changelist (usually
// based on tryjob results).
//
// Parameter "issueOnly" is for debugging purposes only.
func (wh *Handlers) BaselineHandlerV2(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	// No limit for anon users - this is an endpoint backed up by baseline servers, and
	// should be able to handle a large load.

	q := r.URL.Query()
	clID := q.Get("issue")
	issueOnly := q.Get("issueOnly") == "true"
	crs := q.Get("crs")

	if clID != "" {
		if crs == "" {
			// TODO(kjlubick) remove this default after the search page is converted to lit-html.
			crs = wh.ReviewSystems[0].ID
		}
		if _, ok := wh.getCodeReviewSystem(crs); !ok {
			http.Error(w, "Invalid CRS provided.", http.StatusBadRequest)
			return
		}
	} else {
		crs = ""
	}

	bl, err := wh.Baseliner.FetchBaseline(r.Context(), clID, crs, issueOnly)
	if err != nil {
		httputils.ReportError(w, err, "Fetching baselines failed.", http.StatusInternalServerError)
		return
	}

	// TODO(lovisolo): Delete after the ExpectationsMasterStr field has been removed.
	bl.DeprecatedExpectations = expectations.Baseline{}

	sendJSONResponse(w, bl)
}

// MakeResourceHandler creates a static file handler that sets a caching policy.
func MakeResourceHandler(resourceDir string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(resourceDir))
	return func(w http.ResponseWriter, r *http.Request) {
		defer metrics2.FuncTimer().Stop()
		// No limit for anon users - this should be fast enough to handle a large load.
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}

// DigestListHandler returns a list of digests for a given test. This is used by goldctl's
// local diff tech.
func (wh *Handlers) DigestListHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, err, "Failed to parse form values", http.StatusInternalServerError)
		return
	}

	test := r.Form.Get("test")
	corpus := r.Form.Get("corpus")
	if test == "" || corpus == "" {
		http.Error(w, "You must include 'test' and 'corpus'", http.StatusBadRequest)
		return
	}

	out := wh.getDigestsResponse(test, corpus)
	sendJSONResponse(w, out)
}

// getDigestsResponse returns the digests belonging to the given test (and eventually corpus).
func (wh *Handlers) getDigestsResponse(test, corpus string) frontend.DigestListResponse {
	// TODO(kjlubick): Grouping by only test is something we should avoid. We should
	// at least group by test and corpus, but maybe something more robust depending
	// on the instance (e.g. Skia might want to group by colorspace)
	idx := wh.Indexer.GetIndex()
	dc := idx.DigestCountsByTest(types.IncludeIgnoredTraces)

	var xd []types.Digest
	for d := range dc[types.TestName(test)] {
		xd = append(xd, d)
	}

	// Sort alphabetically for determinism
	sort.Slice(xd, func(i, j int) bool {
		return xd[i] < xd[j]
	})

	return frontend.DigestListResponse{
		Digests: xd,
	}
}

// Whoami returns the email address of the user or service account used to authenticate the
// request. For debugging purposes only.
func (wh *Handlers) Whoami(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	user := wh.loggedInAs(r)
	sendJSONResponse(w, map[string]string{"whoami": user})
}

func (wh *Handlers) LatestPositiveDigestHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	traceId, ok := mux.Vars(r)["traceId"]
	if !ok {
		http.Error(w, "Must specify traceId.", http.StatusBadRequest)
		return
	}

	digest, err := wh.Indexer.GetIndex().MostRecentPositiveDigest(r.Context(), tiling.TraceID(traceId))
	if err != nil {
		httputils.ReportError(w, err, "Could not retrieve most recent positive digest.", http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, frontend.MostRecentPositiveDigestResponse{Digest: digest})
}

// GetPerTraceDigestsByTestName returns the digests in the current trace for the given test name
// and corpus, grouped by trace ID.
func (wh *Handlers) GetPerTraceDigestsByTestName(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
	}

	corpus, ok := mux.Vars(r)["corpus"]
	if !ok {
		http.Error(w, "Must specify corpus.", http.StatusBadRequest)
		return
	}

	testName, ok := mux.Vars(r)["testName"]
	if !ok {
		http.Error(w, "Must specify testName.", http.StatusBadRequest)
		return
	}

	digestsByTraceId := frontend.GetPerTraceDigestsByTestNameResponse{}

	// Iterate over all traces in the current tile for the given test name.
	tracesById := wh.Indexer.GetIndex().SlicedTraces(types.IncludeIgnoredTraces, map[string][]string{
		types.CorpusField:     {corpus},
		types.PrimaryKeyField: {testName},
	})
	for _, tracePair := range tracesById {
		// Populate map with the trace's digests.
		digestsByTraceId[tracePair.ID] = tracePair.Trace.Digests
	}

	sendJSONResponse(w, digestsByTraceId)
}

const maxFlakyTraces = 10000 // We don't want to return a slice longer than this because it could
// end up with a result that is too big. 10k * ~200 bytes per trace means this return size will be
// <= 2MB.

// GetFlakyTracesData returns all traces with a number of unique digests (in the current sliding
// window of commits) greater than or equal to a certain threshold.
func (wh *Handlers) GetFlakyTracesData(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
	}

	minUniqueDigests := 10
	minUniqueDigestsStr, ok := mux.Vars(r)["minUniqueDigests"]
	if ok {
		var err error
		minUniqueDigests, err = strconv.Atoi(minUniqueDigestsStr)
		if err != nil {
			httputils.ReportError(w, err, "invalid value for minUniqueDigests", http.StatusBadRequest)
			return
		}
	}

	idx := wh.Indexer.GetIndex()
	counts := idx.DigestCountsByTrace(types.IncludeIgnoredTraces)

	flakyData := frontend.FlakyTracesDataResponse{
		TileSize:    len(idx.Tile().DataCommits()),
		TotalTraces: len(counts),
	}

	for traceID, dc := range counts {
		if len(dc) >= minUniqueDigests {
			flakyData.Traces = append(flakyData.Traces, frontend.FlakyTrace{
				ID:            traceID,
				UniqueDigests: len(dc),
			})
		}
	}
	flakyData.TotalFlakyTraces = len(flakyData.Traces)

	// Sort the flakiest traces first.
	sort.Slice(flakyData.Traces, func(i, j int) bool {
		if flakyData.Traces[i].UniqueDigests == flakyData.Traces[j].UniqueDigests {
			return flakyData.Traces[i].ID < flakyData.Traces[j].ID
		}
		return flakyData.Traces[i].UniqueDigests > flakyData.Traces[j].UniqueDigests
	})

	// Limit the number of traces to maxFlakyTraces, if needed.
	if len(flakyData.Traces) > maxFlakyTraces {
		flakyData.Traces = flakyData.Traces[:maxFlakyTraces]
	}

	sendJSONResponse(w, flakyData)
}

// ChangelistSearchRedirect redirects the user to a search page showing the search results
// for a given CL. It will do a quick scan of the untriaged digests - if it finds some, it will
// include the corpus containing some of those untriaged digests in the search query so the user
// will see results (instead of getting directed to a corpus with no results).
func (wh *Handlers) ChangelistSearchRedirect(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
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
	system, ok := wh.getCodeReviewSystem(crs)
	if !ok {
		http.Error(w, "Invalid Code Review System", http.StatusBadRequest)
		return
	}

	baseURL := fmt.Sprintf("/search?issue=%s&crs=%s", clID, system.ID)

	clIdx := wh.Indexer.GetIndexForCL(system.ID, clID)
	if clIdx == nil {
		// Not cached, so we can't cheaply determine the corpus to include
		if _, err := system.Store.GetChangelist(r.Context(), clID); err != nil {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, baseURL, http.StatusTemporaryRedirect)
		return
	}

	digestList, err := wh.SearchAPI.UntriagedUnignoredTryJobExclusiveDigests(r.Context(), clIdx.LatestPatchset)
	if err != nil {
		sklog.Errorf("Could not find corpus to redirect to for CL %s: %s", clID, err)
		http.Redirect(w, r, baseURL, http.StatusTemporaryRedirect)
		return
	}
	if len(digestList.Corpora) == 0 {
		http.Redirect(w, r, baseURL, http.StatusTemporaryRedirect)
		return
	}

	withCorpus := baseURL + "&corpus=" + digestList.Corpora[0]
	sklog.Debugf("Redirecting to %s", withCorpus)
	http.Redirect(w, r, withCorpus, http.StatusTemporaryRedirect)
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
	// No rate limit, as this should be quite fast.
	_, imgFile := path.Split(r.URL.Path)
	// Get the file that was requested and verify that it's a valid PNG file.
	if !strings.HasSuffix(imgFile, dotPNG) {
		noCacheNotFound(w, r)
		return
	}

	// Trim the image extension to get the image or diff ID.
	imgID := imgFile[:len(imgFile)-len(dotPNG)]
	// Cache images for 12 hours.
	w.Header().Set("Cache-Control", "public, max-age=43200")
	if len(imgID) == validDigestLength {
		// Example request:
		// https://skia-infra-gold.skia.org/img/images/8588cad6f3821b948468df35b67778ef.png
		wh.serveImageWithDigest(w, r, types.Digest(imgID))
	} else if len(imgID) == validDigestLength*2+1 {
		// Example request:
		// https://skia-infra-gold.skia.org/img/diffs/81c4d3a64cf32143ff6c1fbf4cbbec2d-d20731492287002a3f046eae4bd4ce7d.png
		left := types.Digest(imgID[:validDigestLength])
		// + 1 for the dash
		right := types.Digest(imgID[validDigestLength+1:])
		wh.serveImageDiff(w, r, left, right)
	} else {
		noCacheNotFound(w, r)
		return
	}
}

// serveImageWithDigest downloads the image from GCS and returns it. If there is an error, a 404
// or 500 error is returned, as appropriate.
func (wh *Handlers) serveImageWithDigest(w http.ResponseWriter, r *http.Request, digest types.Digest) {
	ctx, span := trace.StartSpan(r.Context(), "frontend_serveImageWithDigest")
	defer span.End()
	// Go's image package has no color profile support and we convert to 8-bit NRGBA to diff,
	// but our source images may have embedded color profiles and be up to 16-bit. So we must
	// at least take care to serve the original .pngs unaltered.
	b, err := wh.GCSClient.GetImage(ctx, digest)
	if err != nil {
		sklog.Warningf("Could not get image with digest %s: %s", digest, err)
		noCacheNotFound(w, r)
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
func (wh *Handlers) serveImageDiff(w http.ResponseWriter, r *http.Request, left types.Digest, right types.Digest) {
	ctx, span := trace.StartSpan(r.Context(), "frontend_serveImageDiff")
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
		noCacheNotFound(w, r)
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
func noCacheNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	http.NotFound(w, r)
}

// ChangelistSummaryHandler returns a summary of the new and untriaged digests produced by this
// CL across all Patchsets.
func (wh *Handlers) ChangelistSummaryHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_ChangelistSummaryHandler")
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
	rv := search2_fe.ConvertChangelistSummaryResponseV1(sum)
	sendJSONResponse(w, rv)
}

// getCLSummary2 fetches, caches, and returns the summary for a given CL. If the result has already
// been cached, it will return that cached value with a flag if the value is still up to date or
// not. If the cached data is stale, it will spawn a goroutine to update the cached value.
func (wh *Handlers) getCLSummary2(ctx context.Context, qCLID string) (search2.NewAndUntriagedSummary, error) {
	ts, err := wh.Search2API.ChangelistLastUpdated(ctx, qCLID)
	if err != nil {
		return search2.NewAndUntriagedSummary{}, skerr.Wrap(err)
	}

	cached, ok := wh.clSummaryCache.Get(qCLID)
	if ok {
		sum, ok := cached.(search2.NewAndUntriagedSummary)
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
				if possiblyUpdated, ok := cached.(search2.NewAndUntriagedSummary); ok {
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
		return search2.NewAndUntriagedSummary{}, skerr.Wrap(err)
	}
	wh.clSummaryCache.Add(qCLID, sum)
	return sum, nil
}

// StartCacheWarming starts a go routine to warm the CL Summary cache. This way, most summaries are
// responsive, even on big instances.
func (wh *Handlers) StartCacheWarming(ctx context.Context) {
	// We warm every CL that was open and produced data or saw triage activity in the last 5 days.
	// After the first cycle, we will incrementally update the cache.
	lastCheck := now.Now(ctx).Add(-5 * 24 * time.Hour)
	go util.RepeatCtx(ctx, time.Minute, func(ctx context.Context) {
		ctx, span := trace.StartSpan(ctx, "web_warmCacheCycle", trace.WithSampler(trace.AlwaysSample()))
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
