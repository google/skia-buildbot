package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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
	"go.skia.org/infra/golden/go/tryjobs"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/validation"
	"go.skia.org/infra/golden/go/web/frontend"
)

const (
	// pageSize is the default page size used for pagination.
	pageSize = 20

	// maxPageSize is the maximum page size used for pagination.
	maxPageSize = 100
)

// WebHandlers holds the environment needed by the various http hander functions
// that have WebHandlers as its receiver.
type WebHandlers struct {
	Baseliner                      baseline.BaselineFetcher
	ChangeListStore                clstore.Store
	CodeReviewURLPrefix            string
	ContinuousIntegrationURLPrefix string
	DeprecatedTryjobMonitor        tryjobs.TryjobMonitor
	DeprecatedTryjobStore          tryjobstore.TryjobStore
	DiffStore                      diff.DiffStore
	ExpectationsStore              expstorage.ExpectationsStore
	GCSClient                      storage.GCSClient
	IgnoreStore                    ignore.IgnoreStore
	Indexer                        indexer.IndexSource
	SearchAPI                      search.SearchAPI
	StatusWatcher                  *status.StatusWatcher
	TileSource                     tilesource.TileSource
	TryJobStore                    tjstore.Store
	VCS                            vcsinfo.VCS
}

// TODO(stephana): once the byBlameHandler is removed, refactor this to
// remove the redundant types ByBlameEntry and ByBlame.

// ByBlameHandler returns a json object with the digests to be triaged grouped by blamelist.
func (wh *WebHandlers) ByBlameHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()

	// Extract the corpus from the query parameters.
	var qp url.Values = nil
	if v := r.FormValue("query"); v != "" {
		var err error
		if qp, err = url.ParseQuery(v); err != nil {
			httputils.ReportError(w, r, err, "invalid input")
			return
		} else if qp.Get(types.CORPUS_FIELD) == "" {
			// If no corpus specified report an error.
			httputils.ReportError(w, r, skerr.Fmt("did not receive value for corpus"), "invalid input")
			return
		}
	}

	blameEntries, err := wh.computeByBlame(qp)
	if err != nil {
		httputils.ReportError(w, r, skerr.Wrapf(err, "computing blame %v", qp), "")
		return
	}

	// Wrap the result in an object because we don't want to return
	// a JSON array.
	sendJSONResponse(w, map[string]interface{}{"data": blameEntries})
}

// computeByBlame creates several ByBlameEntry structs based on the state
// of HEAD and returns them in a slice, for use by the frontend.
func (wh *WebHandlers) computeByBlame(qp url.Values) ([]ByBlameEntry, error) {
	idx := wh.Indexer.GetIndex()
	// At this point query contains at least a corpus.
	untriagedSummaries, err := idx.CalcSummaries(nil, qp, types.ExcludeIgnoredTraces, true /*=head*/)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get untriaged summaries for query %v", qp)
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

	for test, s := range untriagedSummaries {
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
	sort.Sort(ByBlameEntrySlice(blameEntries))

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

type ByBlameEntrySlice []ByBlameEntry

func (b ByBlameEntrySlice) Len() int { return len(b) }
func (b ByBlameEntrySlice) Less(i, j int) bool {
	return b[i].NDigests > b[j].NDigests ||
		// For test determinism, use GroupID as a tie-breaker
		(b[i].NDigests == b[j].NDigests && b[i].GroupID < b[j].GroupID)
}
func (b ByBlameEntrySlice) Swap(i, j int) { b[i], b[j] = b[j], b[i] }

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

// DeprecatedTryjobListHandler returns the list of Gerrit issues that have triggered
// or produced tryjob results recently.
func (wh *WebHandlers) DeprecatedTryjobListHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	var tryjobRuns []*tryjobstore.Issue
	var total int

	offset, size, err := httputils.PaginationParams(r.URL.Query(), 0, pageSize, maxPageSize)
	if err == nil {
		tryjobRuns, total, err = wh.DeprecatedTryjobStore.ListIssues(offset, size)
	}

	if err != nil {
		httputils.ReportError(w, r, err, "Retrieving trybot results failed.")
		return
	}

	pagination := &httputils.ResponsePagination{
		Offset: offset,
		Size:   size,
		Total:  total,
	}
	sendResponseWithPagination(w, tryjobRuns, pagination)
}

// ChangeListsHandler returns the list of code_review.ChangeLists that have
// uploaded results to Gold (via TryJobs).
func (wh *WebHandlers) ChangeListsHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	offset, size, err := httputils.PaginationParams(r.URL.Query(), 0, pageSize, maxPageSize)
	if err != nil {
		httputils.ReportError(w, r, err, "Invalid pagination params.")
		return
	}

	cls, pagination, err := wh.getIngestedChangeLists(r.Context(), offset, size)

	if err != nil {
		httputils.ReportError(w, r, err, "Retrieving changelists results failed.")
		return
	}

	sendResponseWithPagination(w, cls, pagination)
}

// getIngestedChangeLists performs the core of the logic for ChangeListsHandler,
// by fetching N ChangeLists given an offset.
func (wh *WebHandlers) getIngestedChangeLists(ctx context.Context, offset, size int) ([]frontend.ChangeList, *httputils.ResponsePagination, error) {
	cls, total, err := wh.ChangeListStore.GetChangeLists(ctx, offset, size)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "fetching ChangeLists from [%d:%d)", offset, offset+size)
	}
	crs := wh.ChangeListStore.System()
	var retCls []frontend.ChangeList
	for _, cl := range cls {
		retCls = append(retCls, frontend.ConvertChangeList(cl, crs, wh.CodeReviewURLPrefix))
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
func (wh *WebHandlers) ChangeListSummaryHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	// mux.Vars also has "system", which can be used if we ever need to implement
	// the functionality to handle two code review systems at once.
	clID, ok := mux.Vars(r)["id"]
	if !ok {
		httputils.ReportError(w, r, nil, "Must specify 'id' of ChangeList.")
		return
	}

	rv, err := wh.getCLSummary(r.Context(), clID)
	if err != nil {
		httputils.ReportError(w, r, err, "could not retrieve data for the specified CL.")
		return
	}
	sendJSONResponse(w, rv)
}

// getCLSummary does a bulk of the work for ChangeListSummaryHandler, specifically
// fetching the ChangeList and PatchSets from clstore and any associated TryJobs from
// the tjstore.
func (wh *WebHandlers) getCLSummary(ctx context.Context, clID string) (frontend.ChangeListSummary, error) {
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
			tryjobs = append(tryjobs, frontend.ConvertTryJob(tj, cis, wh.ContinuousIntegrationURLPrefix))
		}

		patchsets = append(patchsets, frontend.PatchSet{
			SystemID: ps.SystemID,
			Order:    ps.Order,
			TryJobs:  tryjobs,
		})
	}

	return frontend.ChangeListSummary{
		CL:                frontend.ConvertChangeList(cl, crs, wh.CodeReviewURLPrefix),
		PatchSets:         patchsets,
		NumTotalPatchSets: maxOrder,
	}, nil
}

// SearchHandler is the endpoint for all searches, including accessing
// results that belong to a tryjob.
func (wh *WebHandlers) SearchHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	query, ok := parseSearchQuery(w, r)
	if !ok {
		return
	}

	searchResponse, err := wh.SearchAPI.Search(r.Context(), query)
	if err != nil {
		httputils.ReportError(w, r, err, "Search for digests failed.")
		return
	}
	sendJSONResponse(w, searchResponse)
}

// ExportHandler is the endpoint to export the Gold knowledge base.
// It has the same interface as the search endpoint.
func (wh *WebHandlers) ExportHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	ctx := r.Context()

	query, ok := parseSearchQuery(w, r)
	if !ok {
		return
	}

	if !types.IsMasterBranch(query.DeprecatedIssue) || (query.BlameGroupID != "") {
		msg := "Search query cannot contain blame or issue information."
		httputils.ReportError(w, r, errors.New(msg), msg)
		return
	}

	// Mark the query to avoid expensive diffs.
	query.NoDiff = true

	// Execute the search
	searchResponse, err := wh.SearchAPI.Search(ctx, query)
	if err != nil {
		httputils.ReportError(w, r, err, "Search for digests failed.")
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
		httputils.ReportError(w, r, err, "Unable to serialized knowledge base.")
	}
}

// parseSearchQuery extracts the search query from request.
func parseSearchQuery(w http.ResponseWriter, r *http.Request) (*query.Search, bool) {
	q := query.Search{Limit: 50}
	if err := query.ParseSearch(r, &q); err != nil {
		httputils.ReportError(w, r, err, "Search for digests failed.")
		return nil, false
	}
	return &q, true
}

// DetailsHandler returns the details about a single digest.
func (wh *WebHandlers) DetailsHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	// Extract: test, digest.
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse form values")
		return
	}
	test := r.Form.Get("test")
	digest := r.Form.Get("digest")
	if test == "" || !validation.IsValidDigest(digest) {
		httputils.ReportError(w, r, fmt.Errorf("Some query parameters are wrong or missing: %q %q", test, digest), "Missing query parameters.")
		return
	}

	ret, err := wh.SearchAPI.GetDigestDetails(types.TestName(test), types.Digest(digest))
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to get digest details.")
		return
	}
	sendJSONResponse(w, ret)
}

// DiffHandler returns difference between two digests.
func (wh *WebHandlers) DiffHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	// Extract: test, left, right where left and right are digests.
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse form values")
		return
	}
	test := r.Form.Get("test")
	left := r.Form.Get("left")
	right := r.Form.Get("right")
	if test == "" || !validation.IsValidDigest(left) || !validation.IsValidDigest(right) {
		httputils.ReportError(w, r, fmt.Errorf("Some query parameters are missing or wrong: %q %q %q", test, left, right), "Missing query parameters.")
		return
	}

	ret, err := wh.SearchAPI.DiffDigests(types.TestName(test), types.Digest(left), types.Digest(right))
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to compare digests")
		return
	}

	sendJSONResponse(w, ret)
}

// IgnoresRequest encapsulates a single ignore rule that is submitted for addition or update.
type IgnoresRequest struct {
	Duration string `json:"duration"`
	Filter   string `json:"filter"`
	Note     string `json:"note"`
}

// IgnoresHandler returns the current ignore rules in JSON format.
func (wh *WebHandlers) IgnoresHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "application/json")

	// TODO(kjlubick): these ignore structs used to have counts of how often they were applied
	// in the file - Fix that after the Storages refactoring.
	ignores, err := wh.IgnoreStore.List()
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve ignore rules, there may be none.")
		return
	}

	// TODO(stephana): Wrap in response envelope if it makes sense !
	enc := json.NewEncoder(w)
	if err := enc.Encode(ignores); err != nil {
		sklog.Errorf("Failed to write or encode result: %s", err)
	}
}

// IgnoresUpdateHandler updates an existing ignores rule.
func (wh *WebHandlers) IgnoresUpdateHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	user := login.LoggedInAs(r)
	if user == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to update an ignore rule.")
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 0)
	if err != nil {
		httputils.ReportError(w, r, err, "ID must be valid integer.")
		return
	}
	req := &IgnoresRequest{}
	if err := parseJSON(r, req); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse submitted data.")
		return
	}
	if req.Filter == "" {
		httputils.ReportError(w, r, fmt.Errorf("Invalid Filter: %q", req.Filter), "Filters can't be empty.")
		return
	}
	d, err := human.ParseDuration(req.Duration)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to parse duration")
		return
	}
	ignoreRule := ignore.NewIgnoreRule(user, time.Now().Add(d), req.Filter, req.Note)
	ignoreRule.ID = id

	err = wh.IgnoreStore.Update(id, ignoreRule)
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to update ignore rule.")
		return
	}

	// If update worked just list the current ignores and return them.
	wh.IgnoresHandler(w, r)
}

// IgnoresDeleteHandler deletes an existing ignores rule.
func (wh *WebHandlers) IgnoresDeleteHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	user := login.LoggedInAs(r)
	if user == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to add an ignore rule.")
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 0)
	if err != nil {
		httputils.ReportError(w, r, err, "ID must be valid integer.")
		return
	}

	if numDeleted, err := wh.IgnoreStore.Delete(id); err != nil {
		httputils.ReportError(w, r, err, "Unable to delete ignore rule.")
	} else if numDeleted == 1 {
		sklog.Infof("Successfully deleted ignore with id %d", id)
		// If delete worked just list the current ignores and return them.
		wh.IgnoresHandler(w, r)
	} else {
		sklog.Infof("Deleting ignore with id %d from ignorestore failed", id)
		http.Error(w, "Could not delete ignore - try again later", http.StatusInternalServerError)
		return
	}
}

// IgnoresAddHandler is for adding a new ignore rule.
func (wh *WebHandlers) IgnoresAddHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	user := login.LoggedInAs(r)
	if user == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to add an ignore rule.")
		return
	}
	req := &IgnoresRequest{}
	if err := parseJSON(r, req); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse submitted data.")
		return
	}
	if req.Filter == "" {
		httputils.ReportError(w, r, fmt.Errorf("Invalid Filter: %q", req.Filter), "Filters can't be empty.")
		return
	}
	d, err := human.ParseDuration(req.Duration)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to parse duration")
		return
	}
	ignoreRule := ignore.NewIgnoreRule(user, time.Now().Add(d), req.Filter, req.Note)

	if err = wh.IgnoreStore.Create(ignoreRule); err != nil {
		httputils.ReportError(w, r, err, "Failed to create ignore rule.")
		return
	}

	wh.IgnoresHandler(w, r)
}

// TriageHandler handles a request to change the triage status of one or more
// digests of one test.
//
// It accepts a POST'd JSON serialization of TriageRequest and updates
// the expectations.
func (wh *WebHandlers) TriageHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	user := login.LoggedInAs(r)
	if user == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to triage.")
		return
	}

	req := frontend.TriageRequest{}
	if err := parseJSON(r, &req); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse JSON request.")
		return
	}
	sklog.Infof("Triage request: %#v", req)

	if err := wh.triage(r.Context(), user, req); err != nil {
		httputils.ReportError(w, r, err, "Could not triage")
		return
	}
	// Nothing to return, so just set 200
	w.WriteHeader(http.StatusOK)
}

// triage processes the given TriageRequest.
func (wh *WebHandlers) triage(ctx context.Context, user string, req frontend.TriageRequest) error {
	// Build the expectations change request from the list of digests passed in.
	tc := make(types.Expectations, len(req.TestDigestStatus))
	for test, digests := range req.TestDigestStatus {
		labeledDigests := make(map[types.Digest]types.Label, len(digests))
		for d, label := range digests {
			if !types.ValidLabel(label) {
				return skerr.Fmt("invalid label %q in triage request", label)
			}
			labeledDigests[d] = types.LabelFromString(label)
		}
		tc[test] = labeledDigests
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
func (wh *WebHandlers) StatusHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	sendJSONResponse(w, wh.StatusWatcher.GetStatus())
}

// ClusterDiffHandler calculates the NxN diffs of all the digests that match
// the incoming query and returns the data in a format appropriate for
// handling in d3.
func (wh *WebHandlers) ClusterDiffHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	ctx := r.Context()

	// Extract the test name as we only allow clustering within a test.
	q := query.Search{Limit: 50}
	if err := query.ParseSearch(r, &q); err != nil {
		httputils.ReportError(w, r, err, "Unable to parse query parameter.")
		return
	}
	testName := q.TraceValues.Get(types.PRIMARY_KEY_FIELD)
	if testName == "" {
		httputils.ReportError(w, r, fmt.Errorf("test name parameter missing"), "No test name provided.")
		return
	}

	idx := wh.Indexer.GetIndex()
	searchResponse, err := wh.SearchAPI.Search(ctx, &q)
	if err != nil {
		httputils.ReportError(w, r, err, "Search for digests failed.")
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
		diffs, err := wh.DiffStore.Get(diff.PRIORITY_NOW, d.Digest, remaining)
		if err != nil {
			sklog.Errorf("Failed to calculate differences: %s", err)
			continue
		}
		for otherDigest, diffs := range diffs {
			dm := diffs.(*diff.DiffMetrics)
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
func (wh *WebHandlers) ListTestsHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	// Parse the query object like with the other searches.
	q := query.Search{}
	if err := query.ParseSearch(r, &q); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse form data.")
		return
	}

	// If the query only includes source_type parameters, and include==false, then we can just
	// filter the response from summaries.Get(). If the query is broader than that, or
	// include==true, then we need to call summaries.CalcSummaries().
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, r, err, "Invalid request.")
		return
	}

	idx := wh.Indexer.GetIndex()
	corpus, hasSourceType := q.TraceValues[types.CORPUS_FIELD]
	sumSlice := []*summary.Summary{}
	if !q.IncludeIgnores && q.Head && len(q.TraceValues) == 1 && hasSourceType {
		sumMap := idx.GetSummaries(types.ExcludeIgnoredTraces)
		for _, sum := range sumMap {
			if util.In(sum.Corpus, corpus) && includeSummary(sum, &q) {
				sumSlice = append(sumSlice, sum)
			}
		}
	} else {
		sklog.Infof("%q %q %q", r.FormValue("query"), r.FormValue("include"), r.FormValue("head"))
		sumMap, err := idx.CalcSummaries(nil, q.TraceValues, q.IgnoreState(), q.Head)
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to calculate summaries.")
			return
		}
		for _, sum := range sumMap {
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
func includeSummary(s *summary.Summary, q *query.Search) bool {
	return ((s.Pos > 0) && (q.Pos)) ||
		((s.Neg > 0) && (q.Neg)) ||
		((s.Untriaged > 0) && (q.Unt))
}

type SummarySlice []*summary.Summary

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
func (wh *WebHandlers) ListFailureHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	unavailable := wh.DiffStore.UnavailableDigests()
	ret := FailureList{
		DigestFailures: make([]*diff.DigestFailure, 0, len(unavailable)),
		Count:          len(unavailable),
	}

	for _, failure := range unavailable {
		ret.DigestFailures = append(ret.DigestFailures, failure)
	}

	sort.Sort(sort.Reverse(diff.DigestFailureSlice(ret.DigestFailures)))

	// Limit the errors to the last 50 errors.
	if ret.Count > 50 {
		ret.DigestFailures = ret.DigestFailures[:50]
	}
	sendJSONResponse(w, &ret)
}

// ClearFailureHandler removes failing digests from the local cache and
// returns the current failures.
func (wh *WebHandlers) ClearFailureHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if !wh.purgeDigests(w, r) {
		return
	}
	wh.ListFailureHandler(w, r)
}

// ClearDigests clears digests from the local cache and GS.
func (wh *WebHandlers) ClearDigests(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	if !wh.purgeDigests(w, r) {
		return
	}
	sendJSONResponse(w, &struct{}{})
}

// purgeDigests removes digests from the local cache and from GS if a query argument is set.
// Returns true if there was no error sent to the response writer.
func (wh *WebHandlers) purgeDigests(w http.ResponseWriter, r *http.Request) bool {
	user := login.LoggedInAs(r)
	if user == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to clear digests.")
		return false
	}

	digests := types.DigestSlice{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&digests); err != nil {
		httputils.ReportError(w, r, err, "Unable to decode digest list.")
		return false
	}
	purgeGCS := r.URL.Query().Get("purge") == "true"

	if err := wh.DiffStore.PurgeDigests(digests, purgeGCS); err != nil {
		httputils.ReportError(w, r, err, "Unable to clear digests.")
		return false
	}
	return true
}

// TriageLogHandler returns the entries in the triagelog paginated
// in reverse chronological order.
func (wh *WebHandlers) TriageLogHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	// Get the pagination params.
	var logEntries []expstorage.TriageLogEntry
	var total int

	q := r.URL.Query()
	offset, size, err := httputils.PaginationParams(q, 0, pageSize, maxPageSize)
	if err == nil {
		deprecatedIssueStr := q.Get("issue")

		details := q.Get("details") == "true"
		expStore := wh.ExpectationsStore
		// TODO(kjlubick) remove this legacy handler
		if deprecatedIssueStr != "" && deprecatedIssueStr != "0" {
			expStore = wh.ExpectationsStore.ForChangeList(deprecatedIssueStr, wh.ChangeListStore.System())
		}

		logEntries, total, err = expStore.QueryLog(r.Context(), offset, size, details)
	}

	if err != nil {
		httputils.ReportError(w, r, err, "Unable to retrieve triage log.")
		return
	}

	pagination := &httputils.ResponsePagination{
		Offset: offset,
		Size:   size,
		Total:  total,
	}

	sendResponseWithPagination(w, logEntries, pagination)
}

// TriageUndoHandler performs an "undo" for a given change id.
// The change id's are returned in the result of jsonTriageLogHandler.
// It accepts one query parameter 'id' which is the id if the change
// that should be reversed.
// If successful it returns the same result as a call to jsonTriageLogHandler
// to reflect the changed triagelog.
func (wh *WebHandlers) TriageUndoHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	// Get the user and make sure they are logged in.
	user := login.LoggedInAs(r)
	if user == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to change expectations.")
		return
	}

	// Extract the id to undo.
	changeID := r.URL.Query().Get("id")

	// Do the undo procedure.
	_, err := wh.ExpectationsStore.UndoChange(r.Context(), changeID, user)
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to undo.")
		return
	}

	// Send the same response as a query for the first page.
	wh.TriageLogHandler(w, r)
}

// ParamsHandler returns the union of all parameters.
func (wh *WebHandlers) ParamsHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	tile := wh.Indexer.GetIndex().Tile().GetTile(types.IncludeIgnoredTraces)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tile.ParamSet); err != nil {
		sklog.Errorf("Failed to write or encode result: %s", err)
	}
}

// ParamsHandler returns the commits from the most recent tile.
// Note that this returns things of tiling.Commit, which lacks information
// like the message. For a fuller commit, see GitLogHandler.
func (wh *WebHandlers) CommitsHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	cpxTile, err := wh.TileSource.GetTile()
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to load tile")
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
func (wh *WebHandlers) GitLogHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
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
			httputils.ReportError(w, r, err, "Failed to look up commit data.")
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

// textAllHashesHandler returns the list of all hashes we currently know about
// regardless of triage status.
// Endpoint used by the buildbots to avoid transferring already known images.
func (wh *WebHandlers) TextAllHashesHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	unavailableDigests := wh.DiffStore.UnavailableDigests()

	idx := wh.Indexer.GetIndex()
	byTest := idx.DigestCountsByTest(types.IncludeIgnoredTraces)
	hashes := map[types.Digest]bool{}
	for _, test := range byTest {
		for k := range test {
			if _, ok := unavailableDigests[k]; !ok {
				hashes[k] = true
			}
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	for k := range hashes {
		if _, err := w.Write([]byte(k)); err != nil {
			sklog.Errorf("Failed to write or encode result: %s", err)
			return
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			sklog.Errorf("Failed to write or encode result: %s", err)
			return
		}
	}
}

// TextKnownHashesProxy returns known hashes that have been written to GCS in the background
// Each line contains a single digest for an image. Bots will then only upload images which
// have a hash not found on this list, avoiding significant amounts of unnecessary uploads.
func (wh *WebHandlers) TextKnownHashesProxy(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	w.Header().Set("Content-Type", "text/plain")
	if err := wh.GCSClient.LoadKnownDigests(w); err != nil {
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
func (wh *WebHandlers) DigestTableHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	// Note that testName cannot be empty by definition of the route that got us here.
	var q query.DigestTable
	if err := query.ParseDTQuery(r.Body, 5, &q); err != nil {
		httputils.ReportError(w, r, err, err.Error())
		return
	}

	table, err := wh.SearchAPI.GetDigestTable(&q)
	if err != nil {
		httputils.ReportError(w, r, err, "Search for digests failed.")
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
func (wh *WebHandlers) BaselineHandler(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
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

	bl, err := wh.Baseliner.FetchBaseline(issueIDStr, wh.ChangeListStore.System(), issueOnly)
	if err != nil {
		httputils.ReportError(w, r, err, "Fetching baselines failed.")
		return
	}

	sendJSONResponse(w, bl)
}

// RefreshIssue forces a refresh of a Gerrit issue, i.e. reload data that
// might not have been polled yet etc.
func (wh *WebHandlers) RefreshIssue(w http.ResponseWriter, r *http.Request) {
	defer metrics2.FuncTimer().Stop()
	user := login.LoggedInAs(r)
	if user == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to refresh tryjob data")
		return
	}

	issueID := types.MasterBranch
	var err error
	issueIDStr, ok := mux.Vars(r)["id"]
	if ok {
		issueID, err = strconv.ParseInt(issueIDStr, 10, 64)
		if err != nil {
			httputils.ReportError(w, r, err, "Issue ID must be valid integer.")
			return
		}
	}

	if err := wh.DeprecatedTryjobMonitor.ForceRefresh(issueID); err != nil {
		httputils.ReportError(w, r, err, "Refreshing issue failed.")
		return
	}
	sendJSONResponse(w, map[string]string{})
}

// MakeResourceHandler creates a static file handler that sets a caching policy.
func MakeResourceHandler(resourceDir string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(resourceDir))
	return func(w http.ResponseWriter, r *http.Request) {
		defer metrics2.FuncTimer().Stop()
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}
