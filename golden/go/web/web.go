package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/time/rate"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/search/export"
	"go.skia.org/infra/golden/go/search/query"
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
	DiffStore         diff.DiffStore
	ExpectationsStore expectations.Store
	GCSClient         storage.GCSClient
	IgnoreStore       ignore.Store
	Indexer           indexer.IndexSource
	ReviewSystems     []clstore.ReviewSystem
	SearchAPI         search.SearchAPI
	StatusWatcher     *status.StatusWatcher
	TileSource        tilesource.TileSource
	TryJobStore       tjstore.Store
	VCS               vcsinfo.VCS
}

// Handlers represents all the handlers (e.g. JSON endpoints) of Gold.
// It should be created by clients using NewHandlers.
type Handlers struct {
	HandlersConfig

	anonymousExpensiveQuota *rate.Limiter
	anonymousCheapQuota     *rate.Limiter

	// These can be set for unit tests to simplify the testing.
	testingAuthAs string
	testingNow    time.Time
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
		if conf.DiffStore == nil {
			return nil, skerr.Fmt("DiffStore cannot be nil")
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
		if conf.StatusWatcher == nil {
			return nil, skerr.Fmt("StatusWatcher cannot be nil")
		}
		if conf.TileSource == nil {
			return nil, skerr.Fmt("TileSource cannot be nil")
		}
		if conf.TryJobStore == nil {
			return nil, skerr.Fmt("TryJobStore cannot be nil")
		}
		if conf.VCS == nil {
			return nil, skerr.Fmt("VCS cannot be nil")
		}
	}
	return &Handlers{
		HandlersConfig:          conf,
		anonymousExpensiveQuota: rate.NewLimiter(maxAnonQPSExpensive, maxAnonBurstExpensive),
		anonymousCheapQuota:     rate.NewLimiter(maxAnonQPSCheap, maxAnonBurstCheap),
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

	// Wrap the result in an object because we don't want to return
	// a JSON array.
	sendJSONResponse(w, map[string]interface{}{"data": blameEntries})
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

// ChangeListsHandler returns the list of code_review.ChangeLists that have
// uploaded results to Gold (via TryJobs).
func (wh *Handlers) ChangeListsHandler(w http.ResponseWriter, r *http.Request) {
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
	cls, pagination, err := wh.getIngestedChangeLists(r.Context(), offset, size, activeOnly)

	if err != nil {
		httputils.ReportError(w, err, "Retrieving changelists results failed.", http.StatusInternalServerError)
		return
	}

	sendResponseWithPagination(w, cls, pagination)
}

// getIngestedChangeLists performs the core of the logic for ChangeListsHandler,
// by fetching N ChangeLists given an offset.
func (wh *Handlers) getIngestedChangeLists(ctx context.Context, offset, size int, activeOnly bool) ([]frontend.ChangeList, *httputils.ResponsePagination, error) {
	so := clstore.SearchOptions{
		StartIdx: offset,
		Limit:    size,
	}
	if activeOnly {
		so.OpenCLsOnly = true
	}

	grandTotal := 0
	var retCls []frontend.ChangeList
	for _, system := range wh.ReviewSystems {
		cls, total, err := system.Store.GetChangeLists(ctx, so)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "fetching ChangeLists from [%d:%d)", offset, offset+size)
		}

		for _, cl := range cls {
			retCls = append(retCls, frontend.ConvertChangeList(cl, system.ID, system.URLTemplate))
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

// ChangeListSummaryHandler returns a summary of the data we have collected
// for a given ChangeList, specifically any TryJobs that have uploaded data
// to Gold belonging to various patchsets in it.
func (wh *Handlers) ChangeListSummaryHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	// mux.Vars also has "system", which can be used if we ever need to implement
	// the functionality to handle two code review systems at once.
	clID, ok := mux.Vars(r)["id"]
	if !ok {
		http.Error(w, "Must specify 'id' of ChangeList.", http.StatusBadRequest)
		return
	}
	crs, ok := mux.Vars(r)["system"]
	if !ok {
		http.Error(w, "Must specify 'system' of ChangeList.", http.StatusBadRequest)
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

// getCLSummary does a bulk of the work for ChangeListSummaryHandler, specifically
// fetching the ChangeList and PatchSets from clstore and any associated TryJobs from
// the tjstore.
func (wh *Handlers) getCLSummary(ctx context.Context, system clstore.ReviewSystem, clID string) (frontend.ChangeListSummary, error) {
	cl, err := system.Store.GetChangeList(ctx, clID)
	if err != nil {
		return frontend.ChangeListSummary{}, skerr.Wrapf(err, "getting CL %s", clID)
	}

	// We know xps is sorted by order, if it is non-nil
	xps, err := system.Store.GetPatchSets(ctx, clID)
	if err != nil {
		return frontend.ChangeListSummary{}, skerr.Wrapf(err, "getting PatchSets for CL %s", clID)
	}

	var patchsets []frontend.PatchSet
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
			return frontend.ChangeListSummary{}, skerr.Wrapf(err, "getting TryJobs for CL %s - PS %s", clID, ps.SystemID)
		}
		var tryjobs []frontend.TryJob
		for _, tj := range xtj {
			templ := cisTemplates[tj.System]
			tryjobs = append(tryjobs, frontend.ConvertTryJob(tj, templ))
		}

		patchsets = append(patchsets, frontend.PatchSet{
			SystemID: ps.SystemID,
			Order:    ps.Order,
			TryJobs:  tryjobs,
		})
	}

	return frontend.ChangeListSummary{
		CL:                frontend.ConvertChangeList(cl, system.ID, system.URLTemplate),
		PatchSets:         patchsets,
		NumTotalPatchSets: maxOrder,
	}, nil
}

// ChangeListUntriagedHandler writes out a list of untriaged digests uploaded by this CL that
// are not on master already and are not ignored.
func (wh *Handlers) ChangeListUntriagedHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	requestVars := mux.Vars(r)
	clID, ok := requestVars["id"]
	if !ok {
		http.Error(w, "Must specify 'id' of ChangeList.", http.StatusBadRequest)
		return
	}
	psID, ok := requestVars["patchset"]
	if !ok {
		http.Error(w, "Must specify 'patchset' of ChangeList.", http.StatusBadRequest)
		return
	}
	crs, ok := requestVars["system"]
	if !ok {
		http.Error(w, "Must specify 'system' of ChangeList.", http.StatusBadRequest)
		return
	}

	dl, err := wh.SearchAPI.UntriagedUnignoredTryJobExclusiveDigests(r.Context(), tjstore.CombinedPSID{
		CL:  clID,
		CRS: crs,
		PS:  psID,
	})
	if err != nil {
		httputils.ReportError(w, err, "could not retrieve untriaged digests for CL.", http.StatusInternalServerError)
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

	searchResponse, err := wh.SearchAPI.Search(ctx, q)
	if err != nil {
		httputils.ReportError(w, err, "Search for digests failed.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, searchResponse)
}

// ExportHandler is the endpoint to export the Gold knowledge base.
// It has the same interface as the search endpoint.
func (wh *Handlers) ExportHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	q, ok := parseSearchQuery(w, r)
	if !ok {
		return
	}

	if q.ChangeListID != "" || q.BlameGroupID != "" {
		http.Error(w, "Search query cannot contain blame or issue information.", http.StatusBadRequest)
		return
	}

	// Mark the query to avoid expensive diffs.
	q.NoDiff = true

	// Execute the search
	searchResponse, err := wh.SearchAPI.Search(r.Context(), q)
	if err != nil {
		httputils.ReportError(w, err, "Search for digests failed.", http.StatusInternalServerError)
		return
	}

	// Figure out the base URL. This will work in most situations and doesn't
	// require to pass along additional headers.
	var baseURL string
	if strings.Contains(r.Host, "localhost") {
		baseURL = "http://" + r.Host
	} else {
		baseURL = "https://" + r.Host
	}

	ret := export.ToTestRecords(searchResponse, baseURL)

	// Set it up so that it triggers a save in the browser.
	setJSONHeaders(w)
	w.Header().Set("Content-Disposition", "attachment; filename=meta.json")

	if err := export.WriteTestRecords(ret, w); err != nil {
		httputils.ReportError(w, err, "Unable to serialized knowledge base.", http.StatusInternalServerError)
	}
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

	sendJSONResponse(w, ignores)
}

// getIgnores fetches the ignores from the store and optionally counts how many
// times they are applied.
func (wh *Handlers) getIgnores(ctx context.Context, withCounts bool) ([]*frontend.IgnoreRule, error) {
	rules, err := wh.IgnoreStore.List(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching ignores from store")
	}

	// We want to make a slice of pointers because addIgnoreCounts will add the counts in-place.
	ret := make([]*frontend.IgnoreRule, 0, len(rules))
	for _, r := range rules {
		fr, err := frontend.ConvertIgnoreRule(r)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		ret = append(ret, &fr)
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
func (wh *Handlers) addIgnoreCounts(ctx context.Context, rules []*frontend.IgnoreRule) error {
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
			id, trace := tp.ID, tp.Trace
			if _, ok := nonIgnoredTraces[id]; ok {
				// This wasn't ignored, so we can skip having to count it
				continue
			}
			idxMatched := -1
			untIdxMatched := -1
			numMatched := 0
			untMatched := 0
			for i, r := range rules {
				if trace.Matches(r.ParsedQuery) {
					numMatched++
					ruleCounts[i].Count++
					idxMatched = i

					// Check to see if the digest is untriaged at head
					if d := trace.AtHead(); d != tiling.MissingDigest && exp.Classification(trace.TestName(), d) == expectations.Untriaged {
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
		for i, r := range rules {
			r.Count += ruleCounts[i].Count
			r.UntriagedCount += ruleCounts[i].UntriagedCount
			r.ExclusiveCount += ruleCounts[i].ExclusiveCount
			r.ExclusiveUntriagedCount += ruleCounts[i].ExclusiveUntriagedCount
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
	ignoreRule := ignore.NewRule(user, wh.now().Add(expiresInterval), irb.Filter, irb.Note)
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

	ignoreRule := ignore.NewRule(user, wh.now().Add(expiresInterval), irb.Filter, irb.Note)
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
	if req.ChangeListID != "" && req.ChangeListID != "0" {
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
	if req.ChangeListID != "" && req.ChangeListID != "0" {
		expStore = wh.ExpectationsStore.ForChangeList(req.ChangeListID, req.CodeReviewSystem)
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
func (wh *Handlers) StatusHandler(w http.ResponseWriter, r *http.Request) {
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

	idx := wh.Indexer.GetIndex()
	searchResponse, err := wh.SearchAPI.Search(r.Context(), &q)
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
		diffs, err := wh.DiffStore.Get(r.Context(), d.Digest, remaining)
		if err != nil {
			sklog.Errorf("Failed to calculate differences: %s", err)
			continue
		}
		for otherDigest, dm := range diffs {
			d3.Links = append(d3.Links, Link{
				Source: digestIndex[d.Digest],
				Target: digestIndex[otherDigest],
				Value:  dm.PixelDiffPercent,
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

	pagination := &httputils.ResponsePagination{
		Offset: offset,
		Size:   size,
		Total:  total,
	}

	sendResponseWithPagination(w, logEntries, pagination)
}

// getTriageLog does the actual work of the TriageLogHandler, but is easier to test.
func (wh *Handlers) getTriageLog(ctx context.Context, crs, changeListID string, offset, size int, withDetails bool) ([]frontend.TriageLogEntry, int, error) {
	expStore := wh.ExpectationsStore
	// TODO(kjlubick) remove this legacy handler
	if changeListID != "" && changeListID != "0" {
		expStore = wh.ExpectationsStore.ForChangeList(changeListID, crs)
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
// TODO(kjlubick): This does not properly handle undoing of ChangeListExpectations.
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

// BaselineHandler returns a JSON representation of that baseline including
// baselines for a options issue. It can respond to requests like these:
//
//    /json/expectations
//    /json/expectations?issue=123456
//    /json/expectations?issue=123456&issueOnly=true
//
// It also supports requests in the legacy format below:
//
//    /json/expectations/commit/HEAD
//    /json/expectations/commit/09e87c3d93e2bb188a8dae01b7f8b9ffb2ebcad1
//    /json/expectations/commit/09e87c3d93e2bb188a8dae01b7f8b9ffb2ebcad1?issue=123456
//    /json/expectations/commit/09e87c3d93e2bb188a8dae01b7f8b9ffb2ebcad1?issue=123456&issueOnly=true
//
// TODO(lovisolo): Remove references to legacy routes when goldctl is fully migrated.
//
// The "issue" parameter indicates the changelist ID for which we would like to
// retrieve the baseline. In that case the returned options will be a blend of
// the master baseline and the baseline defined for the changelist (usually
// based on tryjob results).
//
// Parameter "issueOnly" is for debugging purposes only.
func (wh *Handlers) BaselineHandler(w http.ResponseWriter, r *http.Request) {
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

// ChangeListSearchRedirect redirects the user to a search page showing the search results
// for a given CL. It will do a quick scan of the untriaged digests - if it finds some, it will
// include the corpus containing some of those untriaged digests in the search query so the user
// will see results (instead of getting directed to a corpus with no results).
func (wh *Handlers) ChangeListSearchRedirect(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
	}

	requestVars := mux.Vars(r)
	crs, ok := requestVars["system"]
	if !ok {
		http.Error(w, "Must specify 'system' of ChangeList.", http.StatusBadRequest)
		return
	}
	clID, ok := requestVars["id"]
	if !ok {
		http.Error(w, "Must specify 'id' of ChangeList.", http.StatusBadRequest)
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
		if _, err := system.Store.GetChangeList(r.Context(), clID); err != nil {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, baseURL, http.StatusTemporaryRedirect)
		return
	}

	digestList, err := wh.SearchAPI.UntriagedUnignoredTryJobExclusiveDigests(r.Context(), clIdx.LatestPatchSet)
	if err != nil {
		sklog.Errorf("Could not find corpus to redirect to for CL %s: %s", clID, err)
		http.Redirect(w, r, baseURL, http.StatusTemporaryRedirect)
		return
	}
	if len(digestList.Corpora) == 0 {
		http.Redirect(w, r, baseURL, http.StatusTemporaryRedirect)
		return
	}

	withCorpus := baseURL + "&query=source_type%3D" + digestList.Corpora[0]
	sklog.Debugf("Redirecting to %s", withCorpus)
	http.Redirect(w, r, withCorpus, http.StatusTemporaryRedirect)
}

func (wh *Handlers) now() time.Time {
	if !wh.testingNow.IsZero() {
		return wh.testingNow
	}
	return time.Now()
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
