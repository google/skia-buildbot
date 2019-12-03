package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/search/export"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/status"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/tilesource"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
	"go.skia.org/infra/golden/go/validation"
	"go.skia.org/infra/golden/go/web/frontend"
	"golang.org/x/time/rate"
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
	Baseliner                        baseline.BaselineFetcher
	ChangeListStore                  clstore.Store
	CodeReviewURLTemplate            string
	ContinuousIntegrationURLTemplate string
	DiffStore                        diff.DiffStore
	ExpectationsStore                expstorage.ExpectationsStore
	GCSClient                        storage.GCSClient
	IgnoreStore                      ignore.Store
	Indexer                          indexer.IndexSource
	SearchAPI                        search.SearchAPI
	StatusWatcher                    *status.StatusWatcher
	TileSource                       tilesource.TileSource
	TryJobStore                      tjstore.Store
	VCS                              vcsinfo.VCS
}

// Handlers represents all the handlers (e.g. JSON endpoints) of Gold.
// It should be created by clients using NewHandlers.
type Handlers struct {
	HandlersConfig

	anonymousExpensiveQuota *rate.Limiter
	anonymousCheapQuota     *rate.Limiter
}

// NewHandlers returns a new instance of Handlers.
func NewHandlers(conf HandlersConfig, val validateFields) (*Handlers, error) {
	// These fields are required by all types.
	if conf.Baseliner == nil {
		return nil, skerr.Fmt("Baseliner cannot be nil")
	}
	if conf.ChangeListStore == nil {
		return nil, skerr.Fmt("ChangeListStore cannot be nil")
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
		} else if corpus = qp.Get(types.CORPUS_FIELD); corpus == "" {
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
func (wh *Handlers) computeByBlame(ctx context.Context, corpus string) ([]ByBlameEntry, error) {
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
	grouped := map[string][]ByBlame{}

	// The Commit info for each group id.
	commitinfo := map[string][]*tiling.Commit{}
	// map [groupid] [test] TestRollup
	rollups := map[string]map[types.TestName]TestRollup{}

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
				ci := []*tiling.Commit{}
				for _, index := range dist.Freq {
					ci = append(ci, commits[index])
				}
				sort.Sort(CommitSlice(ci))
				commitinfo[groupid] = ci
			}
			// Construct a ByBlame and add it to grouped.
			value := ByBlame{
				Test:          test,
				Digest:        d,
				Blame:         dist,
				CommitIndices: dist.Freq,
			}
			if _, ok := grouped[groupid]; !ok {
				grouped[groupid] = []ByBlame{value}
			} else {
				grouped[groupid] = append(grouped[groupid], value)
			}
			if _, ok := rollups[groupid]; !ok {
				rollups[groupid] = map[types.TestName]TestRollup{}
			}
			// Calculate the rollups.
			r, ok := rollups[groupid][test]
			if !ok {
				r = TestRollup{
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
	blameEntries := make([]ByBlameEntry, 0, len(grouped))
	for groupid, byBlames := range grouped {
		rollup := rollups[groupid]
		nTests := len(rollup)
		var affectedTests []TestRollup

		// Only include the affected tests if there are no more than 10 of them.
		if nTests <= 10 {
			affectedTests = make([]TestRollup, 0, nTests)
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

		blameEntries = append(blameEntries, ByBlameEntry{
			GroupID:       groupid,
			NDigests:      len(byBlames),
			NTests:        nTests,
			AffectedTests: affectedTests,
			Commits:       commitinfo[groupid],
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
func lookUpCommits(freq []int, commits []*tiling.Commit) []string {
	ret := []string{}
	for _, index := range freq {
		ret = append(ret, commits[index].Hash)
	}
	return ret
}

// ByBlameEntry is a helper structure that is serialized to
// JSON and sent to the front-end.
type ByBlameEntry struct {
	GroupID       string           `json:"groupID"`
	NDigests      int              `json:"nDigests"`
	NTests        int              `json:"nTests"`
	AffectedTests []TestRollup     `json:"affectedTests"`
	Commits       []*tiling.Commit `json:"commits"`
}

// ByBlame describes a single digest and its blames.
type ByBlame struct {
	Test          types.TestName          `json:"test"`
	Digest        types.Digest            `json:"digest"`
	Blame         blame.BlameDistribution `json:"blame"`
	CommitIndices []int                   `json:"commit_indices"`
	Key           string
}

// CommitSlice is a utility type simple for sorting Commit slices so earliest commits come first.
type CommitSlice []*tiling.Commit

func (p CommitSlice) Len() int           { return len(p) }
func (p CommitSlice) Less(i, j int) bool { return p[i].CommitTime > p[j].CommitTime }
func (p CommitSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type TestRollup struct {
	Test         types.TestName `json:"test"`
	Num          int            `json:"num"`
	SampleDigest types.Digest   `json:"sample_digest"`
}

// ChangeListsHandler returns the list of code_review.ChangeLists that have
// uploaded results to Gold (via TryJobs).
func (wh *Handlers) ChangeListsHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	offset, size, err := httputils.PaginationParams(r.URL.Query(), 0, pageSize, maxPageSize)
	if err != nil {
		httputils.ReportError(w, err, "Invalid pagination params.", http.StatusInternalServerError)
		return
	}

	cls, pagination, err := wh.getIngestedChangeLists(r.Context(), offset, size)

	if err != nil {
		httputils.ReportError(w, err, "Retrieving changelists results failed.", http.StatusInternalServerError)
		return
	}

	sendResponseWithPagination(w, cls, pagination)
}

// getIngestedChangeLists performs the core of the logic for ChangeListsHandler,
// by fetching N ChangeLists given an offset.
func (wh *Handlers) getIngestedChangeLists(ctx context.Context, offset, size int) ([]frontend.ChangeList, *httputils.ResponsePagination, error) {
	cls, total, err := wh.ChangeListStore.GetChangeLists(ctx, offset, size)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "fetching ChangeLists from [%d:%d)", offset, offset+size)
	}
	crs := wh.ChangeListStore.System()
	var retCls []frontend.ChangeList
	for _, cl := range cls {
		retCls = append(retCls, frontend.ConvertChangeList(cl, crs, wh.CodeReviewURLTemplate))
	}

	pagination := &httputils.ResponsePagination{
		Offset: offset,
		Size:   size,
		Total:  total,
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
		httputils.ReportError(w, nil, "Must specify 'id' of ChangeList.", http.StatusInternalServerError)
		return
	}

	rv, err := wh.getCLSummary(r.Context(), clID)
	if err != nil {
		httputils.ReportError(w, err, "could not retrieve data for the specified CL.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, rv)
}

// getCLSummary does a bulk of the work for ChangeListSummaryHandler, specifically
// fetching the ChangeList and PatchSets from clstore and any associated TryJobs from
// the tjstore.
func (wh *Handlers) getCLSummary(ctx context.Context, clID string) (frontend.ChangeListSummary, error) {
	cl, err := wh.ChangeListStore.GetChangeList(ctx, clID)
	if err != nil {
		return frontend.ChangeListSummary{}, skerr.Wrapf(err, "getting CL %s", clID)
	}

	// We know xps is sorted by order, if it is non-nil
	xps, err := wh.ChangeListStore.GetPatchSets(ctx, clID)
	if err != nil {
		return frontend.ChangeListSummary{}, skerr.Wrapf(err, "getting PatchSets for CL %s", clID)
	}

	crs := wh.ChangeListStore.System()
	var patchsets []frontend.PatchSet
	maxOrder := 0

	// TODO(kjlubick): maybe fetch these in parallel (with errgroup)
	for _, ps := range xps {
		if ps.Order > maxOrder {
			maxOrder = ps.Order
		}
		psID := tjstore.CombinedPSID{
			CL:  clID,
			CRS: wh.ChangeListStore.System(),
			PS:  ps.SystemID,
		}
		xtj, err := wh.TryJobStore.GetTryJobs(ctx, psID)
		if err != nil {
			return frontend.ChangeListSummary{}, skerr.Wrapf(err, "getting TryJobs for CL %s - PS %s", clID, ps.SystemID)
		}
		cis := wh.TryJobStore.System()
		var tryjobs []frontend.TryJob
		for _, tj := range xtj {
			tryjobs = append(tryjobs, frontend.ConvertTryJob(tj, cis, wh.ContinuousIntegrationURLTemplate))
		}

		patchsets = append(patchsets, frontend.PatchSet{
			SystemID: ps.SystemID,
			Order:    ps.Order,
			TryJobs:  tryjobs,
		})
	}

	return frontend.ChangeListSummary{
		CL:                frontend.ConvertChangeList(cl, crs, wh.CodeReviewURLTemplate),
		PatchSets:         patchsets,
		NumTotalPatchSets: maxOrder,
	}, nil
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
		msg := "Search query cannot contain blame or issue information."
		httputils.ReportError(w, errors.New(msg), msg, http.StatusInternalServerError)
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

	// Extract: test, digest.
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

	ret, err := wh.SearchAPI.GetDigestDetails(r.Context(), types.TestName(test), types.Digest(digest))
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
		httputils.ReportError(w, fmt.Errorf("Some query parameters are missing or wrong: %q %q %q", test, left, right), "Missing query parameters.", http.StatusInternalServerError)
		return
	}

	ret, err := wh.SearchAPI.DiffDigests(r.Context(), types.TestName(test), types.Digest(left), types.Digest(right))
	if err != nil {
		httputils.ReportError(w, err, "Unable to compare digests", http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, ret)
}

// IgnoresHandler returns the current ignore rules in JSON format.
func (wh *Handlers) IgnoresHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	includeCounts := mux.Vars(r)["counts"] != ""
	ignores, err := wh.getIgnores(r.Context(), includeCounts)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve ignore rules, there may be none.", http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, ignores)
}

// getIgnores fetches the ignores from the store and optionally counts how many
// times they are applied.
func (wh *Handlers) getIgnores(ctx context.Context, withCounts bool) ([]frontend.IgnoreRule, error) {
	rules, err := wh.IgnoreStore.List(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching ignores from store")
	}

	ret := make([]frontend.IgnoreRule, 0, len(rules))
	for _, r := range rules {
		ret = append(ret, frontend.ConvertIgnoreRule(r))
	}

	if withCounts {
		if err := wh.addIgnoreCounts(ctx, ret); err != nil {
			return nil, skerr.Wrapf(err, "adding ignore counts to %d rules", len(ret))
		}
	}

	return ret, nil
}

// addIgnoreCounts goes through the whole tile and counts how many traces each of the rules
// applies to.
func (wh *Handlers) addIgnoreCounts(ctx context.Context, rules []frontend.IgnoreRule) error {
	// TODO(kjlubick)
	return skerr.Fmt("not impl")
}

// IgnoresRequest encapsulates a single ignore rule that is submitted for addition or update.
type IgnoresRequest struct {
	Duration string `json:"duration"`
	Filter   string `json:"filter"`
	Note     string `json:"note"`
}

// IgnoresUpdateHandler updates an existing ignores rule.
func (wh *Handlers) IgnoresUpdateHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	user := login.LoggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to update an ignore rule.", http.StatusUnauthorized)
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		http.Error(w, "ID must be non-empty.", http.StatusBadRequest)
		return
	}
	req := &IgnoresRequest{}
	if err := parseJSON(r, req); err != nil {
		httputils.ReportError(w, err, "Failed to parse submitted data", http.StatusBadRequest)
		return
	}
	if req.Filter == "" {
		http.Error(w, "Filter can't be empty", http.StatusBadRequest)
		return
	}
	d, err := human.ParseDuration(req.Duration)
	if err != nil {
		httputils.ReportError(w, err, "Failed to parse duration", http.StatusBadRequest)
		return
	}
	ignoreRule := ignore.NewRule(user, time.Now().Add(d), req.Filter, req.Note)
	ignoreRule.ID = id

	err = wh.IgnoreStore.Update(r.Context(), id, ignoreRule)
	if err != nil {
		httputils.ReportError(w, err, "Unable to update ignore rule", http.StatusInternalServerError)
		return
	}

	// If update worked just list the current ignores and return them.
	wh.IgnoresHandler(w, r)
}

// IgnoresDeleteHandler deletes an existing ignores rule.
func (wh *Handlers) IgnoresDeleteHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	user := login.LoggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to delete an ignore rule", http.StatusUnauthorized)
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		http.Error(w, "ID must be non-empty.", http.StatusBadRequest)
		return
	}

	if numDeleted, err := wh.IgnoreStore.Delete(r.Context(), id); err != nil {
		httputils.ReportError(w, err, "Unable to delete ignore rule", http.StatusInternalServerError)
		return
	} else if numDeleted == 1 {
		sklog.Infof("Successfully deleted ignore with id %s", id)
		// If delete worked just list the current ignores and return them.
		wh.IgnoresHandler(w, r)
	} else {
		sklog.Infof("Deleting ignore with id %s from ignorestore failed", id)
		http.Error(w, "Could not delete ignore - try again later", http.StatusInternalServerError)
		return
	}
}

// IgnoresAddHandler is for adding a new ignore rule.
func (wh *Handlers) IgnoresAddHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	user := login.LoggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to add an ignore rule", http.StatusUnauthorized)
		return
	}
	req := &IgnoresRequest{}
	if err := parseJSON(r, req); err != nil {
		httputils.ReportError(w, err, "Failed to parse submitted data", http.StatusBadRequest)
		return
	}
	if req.Filter == "" {
		http.Error(w, "Filter can't be empty", http.StatusBadRequest)
		return
	}
	d, err := human.ParseDuration(req.Duration)
	if err != nil {
		httputils.ReportError(w, err, "Failed to parse duration", http.StatusBadRequest)
		return
	}
	ignoreRule := ignore.NewRule(user, time.Now().Add(d), req.Filter, req.Note)

	if err = wh.IgnoreStore.Create(r.Context(), ignoreRule); err != nil {
		httputils.ReportError(w, err, "Failed to create ignore rule", http.StatusInternalServerError)
		return
	}

	wh.IgnoresHandler(w, r)
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
		httputils.ReportError(w, fmt.Errorf("Not logged in."), "You must be logged in to triage.", http.StatusInternalServerError)
		return
	}

	req := frontend.TriageRequest{}
	if err := parseJSON(r, &req); err != nil {
		httputils.ReportError(w, err, "Failed to parse JSON request.", http.StatusInternalServerError)
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
	// Build the expectations change request from the list of digests passed in.
	tc := make([]expstorage.Delta, 0, len(req.TestDigestStatus))
	for test, digests := range req.TestDigestStatus {
		for d, label := range digests {
			if !expectations.ValidLabel(label) {
				return skerr.Fmt("invalid label %q in triage request", label)
			}
			tc = append(tc, expstorage.Delta{
				Grouping: test,
				Digest:   d,
				Label:    expectations.LabelFromString(label),
			})
		}
	}

	// Use the expectations store for the master branch, unless an issue was given
	// in the request, then get the expectations store for the issue.
	expStore := wh.ExpectationsStore
	// TODO(kjlubick) remove the legacy check here after the frontend bakes in.
	if req.ChangeListID != "" && req.ChangeListID != "0" {
		expStore = wh.ExpectationsStore.ForChangeList(req.ChangeListID, wh.ChangeListStore.System())
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
		httputils.ReportError(w, err, "Unable to parse query parameter.", http.StatusInternalServerError)
		return
	}
	testName := q.TraceValues.Get(types.PRIMARY_KEY_FIELD)
	if testName == "" {
		httputils.ReportError(w, fmt.Errorf("test name parameter missing"), "No test name provided.", http.StatusInternalServerError)
		return
	}

	idx := wh.Indexer.GetIndex()
	searchResponse, err := wh.SearchAPI.Search(r.Context(), &q)
	if err != nil {
		httputils.ReportError(w, err, "Search for digests failed.", http.StatusInternalServerError)
		return
	}

	// TODO(kjlubick): Check if we need to sort these
	// // Sort the digests so they are displayed with untriaged last, which means
	// // they will be displayed 'on top', because in SVG document order is z-order.

	digests := types.DigestSlice{}
	for _, digest := range searchResponse.Digests {
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
		ParamsetByDigest: map[types.Digest]map[string][]string{},
		ParamsetsUnion:   map[string][]string{},
	}
	for i, d := range searchResponse.Digests {
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
		d3.ParamsetsUnion = util.AddParamSetToParamSet(d3.ParamsetsUnion, d3.ParamsetByDigest[d.Digest])
	}

	for _, p := range d3.ParamsetsUnion {
		sort.Strings(p)
	}

	sendJSONResponse(w, d3)
}

// Node represents a single node in a d3 diagram. Used in ClusterDiffResult.
type Node struct {
	Name   types.Digest `json:"name"`
	Status string       `json:"status"`
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
	ParamsetByDigest map[types.Digest]map[string][]string `json:"paramsetByDigest"`
	ParamsetsUnion   map[string][]string                  `json:"paramsetsUnion"`
}

// ListTestsHandler returns a JSON list with high level information about
// each test.
//
// It takes these parameters:
//  include - If true ignored digests should be included. (true, false)
//  query   - A query to restrict the responses to, encoded as a URL encoded paramset.
//  head    - if only digest that appear at head should be included.
//  unt     - If true include tests that have untriaged digests. (true, false)
//  pos     - If true include tests that have positive digests. (true, false)
//  neg     - If true include tests that have negative digests. (true, false)
//
// The return format looks like:
//
//  [
//    {
//      "name": "01-original",
//      "diameter": 123242,
//      "untriaged": 2,
//      "num": 2
//    },
//    ...
//  ]
//
func (wh *Handlers) ListTestsHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	// Parse the query object like with the other searches.
	q := query.Search{}
	if err := query.ParseSearch(r, &q); err != nil {
		httputils.ReportError(w, err, "Failed to parse form data.", http.StatusInternalServerError)
		return
	}

	// If the query only includes source_type parameters, and include==false, then we can just
	// filter the response from summaries.Get(). If the query is broader than that, or
	// include==true, then we need to call summaries.SummarizeByGrouping().
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, err, "Invalid request.", http.StatusInternalServerError)
		return
	}

	idx := wh.Indexer.GetIndex()
	corpora, hasSourceType := q.TraceValues[types.CORPUS_FIELD]
	sumSlice := []*summary.TriageStatus{}
	if !q.IncludeIgnores && q.Head && len(q.TraceValues) == 1 && hasSourceType {
		sumMap := idx.GetSummaries(types.ExcludeIgnoredTraces)
		for _, sum := range sumMap {
			if util.In(sum.Corpus, corpora) && includeSummary(sum, &q) {
				sumSlice = append(sumSlice, sum)
			}
		}
	} else {
		if hasSourceType && len(corpora) != 1 {
			http.Error(w, "please supply only one corpus when filtering", http.StatusBadRequest)
			return
		}
		sklog.Infof("%q %q %q", r.FormValue("query"), r.FormValue("include"), r.FormValue("head"))
		statuses, err := idx.SummarizeByGrouping(r.Context(), corpora[0], q.TraceValues, q.IgnoreState(), q.Head)
		if err != nil {
			httputils.ReportError(w, err, "Failed to calculate summaries.", http.StatusInternalServerError)
			return
		}
		for _, sum := range statuses {
			if includeSummary(sum, &q) {
				sumSlice = append(sumSlice, sum)
			}
		}
	}

	sort.Sort(SummarySlice(sumSlice))
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(sumSlice); err != nil {
		sklog.Errorf("Failed to write or encode result: %s", err)
	}
}

// includeSummary returns true if the given summary matches the query flags.
func includeSummary(s *summary.TriageStatus, q *query.Search) bool {
	return s != nil && ((s.Pos > 0 && q.Pos) ||
		(s.Neg > 0 && q.Neg) ||
		(s.Untriaged > 0 && q.Unt))
}

type SummarySlice []*summary.TriageStatus

func (p SummarySlice) Len() int           { return len(p) }
func (p SummarySlice) Less(i, j int) bool { return p[i].Untriaged > p[j].Untriaged }
func (p SummarySlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// FailureList contains the list of the digests that could not be processed
// the count value is for convenience to make it easier to inspect the JSON
// output and might be removed in the future.
type FailureList struct {
	Count          int                   `json:"count"`
	DigestFailures []*diff.DigestFailure `json:"failures"`
}

// ListFailureHandler returns the digests that have failed to load.
func (wh *Handlers) ListFailureHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	unavailable, err := wh.DiffStore.UnavailableDigests(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Could not fetch failures", http.StatusInternalServerError)
		return
	}
	ret := FailureList{
		DigestFailures: make([]*diff.DigestFailure, 0, len(unavailable)),
		Count:          len(unavailable),
	}

	for _, failure := range unavailable {
		ret.DigestFailures = append(ret.DigestFailures, failure)
	}

	// Sort failures newest to oldest
	sort.Slice(ret.DigestFailures, func(i, j int) bool {
		return ret.DigestFailures[i].TS > ret.DigestFailures[j].TS
	})

	// Limit the errors to the last 50 errors.
	if ret.Count > 50 {
		ret.DigestFailures = ret.DigestFailures[:50]
	}
	sendJSONResponse(w, &ret)
}

// ClearFailureHandler removes failing digests from the local cache and
// returns the current failures.
func (wh *Handlers) ClearFailureHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if !wh.purgeDigests(w, r) {
		return
	}
	wh.ListFailureHandler(w, r)
}

// ClearDigests clears digests from the local cache and GS.
func (wh *Handlers) ClearDigests(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if !wh.purgeDigests(w, r) {
		return
	}
	sendJSONResponse(w, &struct{}{})
}

// purgeDigests removes digests from the local cache and from GS if a query argument is set.
// Returns true if there was no error sent to the response writer.
func (wh *Handlers) purgeDigests(w http.ResponseWriter, r *http.Request) bool {
	user := login.LoggedInAs(r)
	if user == "" {
		httputils.ReportError(w, fmt.Errorf("Not logged in."), "You must be logged in to clear digests.", http.StatusInternalServerError)
		return false
	}

	digests := types.DigestSlice{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&digests); err != nil {
		httputils.ReportError(w, err, "Unable to decode digest list.", http.StatusInternalServerError)
		return false
	}
	purgeGCS := r.URL.Query().Get("purge") == "true"

	if err := wh.DiffStore.PurgeDigests(r.Context(), digests, purgeGCS); err != nil {
		httputils.ReportError(w, err, "Unable to clear digests.", http.StatusInternalServerError)
		return false
	}
	return true
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

	changeList := q.Get("issue")

	details := q.Get("details") == "true"
	logEntries, total, err := wh.getTriageLog(r.Context(), changeList, offset, size, details)

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
func (wh *Handlers) getTriageLog(ctx context.Context, changeListID string, offset, size int, withDetails bool) ([]frontend.TriageLogEntry, int, error) {
	expStore := wh.ExpectationsStore
	// TODO(kjlubick) remove this legacy handler
	if changeListID != "" && changeListID != "0" {
		expStore = wh.ExpectationsStore.ForChangeList(changeListID, wh.ChangeListStore.System())
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
		httputils.ReportError(w, fmt.Errorf("Not logged in."), "You must be logged in to change expectations.", http.StatusInternalServerError)
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

	tile := wh.Indexer.GetIndex().Tile().GetTile(types.IncludeIgnoredTraces)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tile.ParamSet); err != nil {
		sklog.Errorf("Failed to write or encode result: %s", err)
	}
}

// CommitsHandler returns the commits from the most recent tile.
// Note that this returns things of tiling.Commit, which lacks information
// like the message. For a fuller commit, see GitLogHandler.
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
	if err := json.NewEncoder(w).Encode(cpxTile.DataCommits()); err != nil {
		sklog.Errorf("Failed to write or encode result: %s", err)
	}
}

// gitLog is a struct to mimic the return value of googlesource (see GitLogHandler)
type gitLog struct {
	Log []commitInfo `json:"log"`
}

// commitInfo is a simplified view of a commit. The author and timestamp should
// already be on the frontend, as those are stored in tiling.Commit but Message
// is not, so it needs to be provided in this struct.
type commitInfo struct {
	Commit  string `json:"commit"`
	Message string `json:"message"`
}

// GitLogHandler takes a request with a start and end commit and returns
// an array of commit hashes and messages similar to the JSON googlesource would
// return for a query like:
// https://chromium.googlesource.com/chromium/src/+log/[start]~1..[end]
// Essentially, we just need the commit Subject for each of the commits,
// although this could easily be expanded to have more of the commit info.
func (wh *Handlers) GitLogHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	// We have start and end as request params
	p := r.URL.Query()
	startHashes := p["start"] // the oldest commit
	endHashes := p["end"]     // the newest commit
	if len(startHashes) == 0 || len(endHashes) == 0 {
		http.Error(w, "must supply a start and end hash", http.StatusBadRequest)
		return
	}
	start := startHashes[0]
	end := endHashes[0]
	ctx := r.Context()

	rv := gitLog{}

	if start == end {
		// Single commit
		c, err := wh.VCS.Details(ctx, start, false)
		if err != nil || c == nil {
			sklog.Infof("Could not find commit with hash %s: %v", start, err)
			http.Error(w, "invalid start and end hash", http.StatusBadRequest)
			return
		}

		rv.Log = []commitInfo{
			{
				Commit:  c.Hash,
				Message: c.Subject,
			},
		}
	} else {
		// range of commits
		details, err := wh.VCS.DetailsMulti(ctx, []string{start, end}, false)
		if err != nil || len(details) < 2 || details[0] == nil || details[1] == nil {
			sklog.Infof("Invalid gitlog request start=%s end=%s: %#v %v", start, end, details, err)
			http.Error(w, "invalid start or end hash", http.StatusBadRequest)
			return
		}

		first, second := details[0].Timestamp, details[1].Timestamp
		// Add one nanosecond because range is exclusive on the end and we want to include that
		// last commit
		indexCommits := wh.VCS.Range(first, second.Add(time.Nanosecond))
		if indexCommits == nil {
			// indexCommit should never be nil, but just in case...
			http.Error(w, "no commits found between hashes", http.StatusBadRequest)
			return
		}

		hashes := make([]string, 0, len(indexCommits))
		for _, c := range indexCommits {
			hashes = append(hashes, c.Hash)
		}

		commits, err := wh.VCS.DetailsMulti(ctx, hashes, false)
		if err != nil {
			httputils.ReportError(w, err, "Failed to look up commit data.", http.StatusInternalServerError)
			return
		}

		ci := make([]commitInfo, 0, len(commits))
		for _, c := range commits {
			ci = append(ci, commitInfo{
				Commit:  c.Hash,
				Message: c.Subject,
			})
		}
		rv.Log = ci
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(rv); err != nil {
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

// DigestTableHandler returns a JSON description for the given test.
// The result is intended to be displayed in a grid-like fashion.
//
// Input format of a POST request:
//
// Output format in JSON:
//
//
func (wh *Handlers) DigestTableHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	// Note that testName cannot be empty by definition of the route that got us here.
	var q query.DigestTable
	if err := query.ParseDTQuery(r.Body, 5, &q); err != nil {
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}

	table, err := wh.SearchAPI.GetDigestTable(&q)
	if err != nil {
		httputils.ReportError(w, err, "Search for digests failed.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, table)
}

// BaselineHandler returns a JSON representation of that baseline including
// baselines for a options issue. It can respond to requests like these:
//    /json/baseline
//    /json/baseline/64789
// where the latter contains the issue id for which we would like to retrieve
// the baseline. In that case the returned options will be blend of the master
// baseline and the baseline defined for the issue (usually based on tryjob
// results).
func (wh *Handlers) BaselineHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	// No limit for anon users - this is an endpoint backed up by baseline servers, and
	// should be able to handle a large load.
	issueIDStr := ""
	issueOnly := false
	var ok bool

	// TODO(stephana): The codepath for using issue_id as segment of the request path is
	// deprecated and should be removed with the route that defines it (see shared.go).
	if issueIDStr, ok = mux.Vars(r)["issue_id"]; !ok {
		q := r.URL.Query()
		issueIDStr = q.Get("issue")
		issueOnly = q.Get("issueOnly") == "true"
	}

	bl, err := wh.Baseliner.FetchBaseline(r.Context(), issueIDStr, wh.ChangeListStore.System(), issueOnly)
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
