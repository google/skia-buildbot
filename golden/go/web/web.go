package web

import (
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
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/status"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/validation"
)

const (
	// DEFAULT_PAGE_SIZE is the default page size used for pagination.
	DEFAULT_PAGE_SIZE = 20

	// MAX_PAGE_SIZE is the maximum page size used for pagination.
	MAX_PAGE_SIZE = 100
)

// WebHandlers holds the environment needed by the various http hander functions
// that have WebHandlers as its receiver.
type WebHandlers struct {
	Storages      *storage.Storage
	StatusWatcher *status.StatusWatcher
	Indexer       *indexer.Indexer
	IssueTracker  issues.IssueTracker
	SearchAPI     *search.SearchAPI
}

// TODO(stephana): once the byBlameHandler is removed, refactor this to
// remove the redundant types ByBlameEntry and ByBlame.

// JsonByBlameHandler returns a json object with the digests to be triaged grouped by blamelist.
func (wh *WebHandlers) JsonByBlameHandler(w http.ResponseWriter, r *http.Request) {
	idx := wh.Indexer.GetIndex()

	// Extract the corpus from the query.
	var query url.Values = nil
	var err error = nil
	if q := r.FormValue("query"); q != "" {
		if query, err = url.ParseQuery(q); query.Get(types.CORPUS_FIELD) == "" {
			err = fmt.Errorf("Got query field, but did not contain %s field.", types.CORPUS_FIELD)
		}
	}

	// If no corpus specified return an error.
	if err != nil {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Did not receive value for corpus/%s.", types.CORPUS_FIELD))
		return
	}

	// At this point query contains at least a corpus.
	tile, sum, err := allUntriagedSummaries(idx, query)
	commits := tile.Commits
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to load summaries.")
		return
	}

	// This is a very simple grouping of digests, for every digest we look up the
	// blame list for that digest and then use the concatenated git hashes as a
	// group id. All of the digests are then grouped by their group id.

	// Collects a ByBlame for each untriaged digest, keyed by group id.
	grouped := map[string][]*ByBlame{}

	// The Commit info for each group id.
	commitinfo := map[string][]*tiling.Commit{}
	// map [groupid] [test] TestRollup
	rollups := map[string]map[string]*TestRollup{}

	for test, s := range sum {
		for _, d := range s.UntHashes {
			dist := idx.GetBlame(test, d, commits)
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
			value := &ByBlame{
				Test:          test,
				Digest:        d,
				Blame:         dist,
				CommitIndices: dist.Freq,
			}
			if _, ok := grouped[groupid]; !ok {
				grouped[groupid] = []*ByBlame{value}
			} else {
				grouped[groupid] = append(grouped[groupid], value)
			}
			if _, ok := rollups[groupid]; !ok {
				rollups[groupid] = map[string]*TestRollup{}
			}
			// Calculate the rollups.
			if _, ok := rollups[groupid][test]; !ok {
				rollups[groupid][test] = &TestRollup{
					Test:         test,
					Num:          0,
					SampleDigest: d,
				}
			}
			rollups[groupid][test].Num += 1
		}
	}

	// Assemble the response.
	blameEntries := make([]*ByBlameEntry, 0, len(grouped))
	for groupid, byBlames := range grouped {
		rollup := rollups[groupid]
		nTests := len(rollup)
		var affectedTests []*TestRollup = nil

		// Only include the affected tests if there are no more than 10 of them.
		if nTests <= 10 {
			affectedTests = make([]*TestRollup, 0, nTests)
			for _, testInfo := range rollup {
				affectedTests = append(affectedTests, testInfo)
			}
		}

		blameEntries = append(blameEntries, &ByBlameEntry{
			GroupID:       groupid,
			NDigests:      len(byBlames),
			NTests:        nTests,
			AffectedTests: affectedTests,
			Commits:       commitinfo[groupid],
		})
	}
	sort.Sort(ByBlameEntrySlice(blameEntries))

	// Wrap the result in an object because we don't want to return
	// a JSON array.
	sendJsonResponse(w, map[string]interface{}{"data": blameEntries})
}

// allUntriagedSummaries returns a tile and summaries for all untriaged GMs.
func allUntriagedSummaries(idx *indexer.SearchIndex, query url.Values) (*tiling.Tile, map[string]*summary.Summary, error) {
	tile := idx.CpxTile().GetTile(true)

	// Get a list of all untriaged images by test.
	sum, err := idx.CalcSummaries([]string{}, query, false, true)
	if err != nil {
		return nil, nil, fmt.Errorf("Couldn't load summaries: %s", err)
	}
	return tile, sum, nil
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
	AffectedTests []*TestRollup    `json:"affectedTests"`
	Commits       []*tiling.Commit `json:"commits"`
}

type ByBlameEntrySlice []*ByBlameEntry

func (b ByBlameEntrySlice) Len() int           { return len(b) }
func (b ByBlameEntrySlice) Less(i, j int) bool { return b[i].GroupID < b[j].GroupID }
func (b ByBlameEntrySlice) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

// ByBlame describes a single digest and it's blames.
type ByBlame struct {
	Test          string                   `json:"test"`
	Digest        string                   `json:"digest"`
	Blame         *blame.BlameDistribution `json:"blame"`
	CommitIndices []int                    `json:"commit_indices"`
	Key           string
}

// CommitSlice is a utility type simple for sorting Commit slices so earliest commits come first.
type CommitSlice []*tiling.Commit

func (p CommitSlice) Len() int           { return len(p) }
func (p CommitSlice) Less(i, j int) bool { return p[i].CommitTime > p[j].CommitTime }
func (p CommitSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type TestRollup struct {
	Test         string `json:"test"`
	Num          int    `json:"num"`
	SampleDigest string `json:"sample_digest"`
}

// JsonTryjobListHandler returns the list of Gerrit issues that have triggered
// or produced tryjob results recently.
func (wh *WebHandlers) JsonTryjobListHandler(w http.ResponseWriter, r *http.Request) {
	var tryjobRuns []*tryjobstore.Issue
	var total int

	offset, size, err := httputils.PaginationParams(r.URL.Query(), 0, DEFAULT_PAGE_SIZE, MAX_PAGE_SIZE)
	if err == nil {
		tryjobRuns, total, err = wh.Storages.TryjobStore.ListIssues(offset, size)
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
	sendResponse(w, tryjobRuns, 200, pagination)
}

// JsonTryjobsSummaryHandler is the endpoint to get a summary of the tryjob
// results for a Gerrit issue.
func (wh *WebHandlers) JsonTryjobSummaryHandler(w http.ResponseWriter, r *http.Request) {
	issueID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		httputils.ReportError(w, r, err, "ID must be valid integer.")
		return
	}

	if issueID <= 0 {
		httputils.ReportError(w, r, fmt.Errorf("Issue id is <= 0"), "Valid issue ID required.")
		return
	}

	resp, err := wh.SearchAPI.Summary(issueID)
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to retrieve tryjobs summary.")
		return
	}
	sendJsonResponse(w, resp)
}

// JsonSearchHandler is the endpoint for all searches.
func (wh *WebHandlers) JsonSearchHandler(w http.ResponseWriter, r *http.Request) {
	query, ok := parseSearchQuery(w, r)
	if !ok {
		return
	}

	searchResponse, err := wh.SearchAPI.Search(r.Context(), query)
	if err != nil {
		httputils.ReportError(w, r, err, "Search for digests failed.")
		return
	}
	sendJsonResponse(w, searchResponse)
}

// JsonExportHandler is the endpoint to export the Gold knowledge base.
// It has the same interface as the search endpoint.
func (wh *WebHandlers) JsonExportHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query, ok := parseSearchQuery(w, r)
	if !ok {
		return
	}

	if (query.Issue > 0) || (query.BlameGroupID != "") {
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

	ret := search.GetExportRecords(searchResponse, baseURL)

	// Set it up so that it triggers a save in the browser.
	setJSONHeaders(w)
	w.Header().Set("Content-Disposition", "attachment; filename=meta.json")

	if err := search.WriteExportTestRecords(ret, w); err != nil {
		httputils.ReportError(w, r, err, "Unable to serialized knowledge base.")
	}
}

// parseSearchQuery extracts the search query from request.
func parseSearchQuery(w http.ResponseWriter, r *http.Request) (*search.Query, bool) {
	query := search.Query{Limit: 50}
	if err := search.ParseQuery(r, &query); err != nil {
		httputils.ReportError(w, r, err, "Search for digests failed.")
		return nil, false
	}
	return &query, true
}

// JsonDetailsHandler returns the details about a single digest.
func (wh *WebHandlers) JsonDetailsHandler(w http.ResponseWriter, r *http.Request) {
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

	ret, err := wh.SearchAPI.GetDigestDetails(test, digest)
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to get digest details.")
		return
	}
	sendJsonResponse(w, ret)
}

// JsonDiffHandler returns difference between two digests.
func (wh *WebHandlers) JsonDiffHandler(w http.ResponseWriter, r *http.Request) {
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

	ret, err := search.CompareDigests(test, left, right, wh.Storages, wh.Indexer.GetIndex())
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to compare digests")
		return
	}

	sendJsonResponse(w, ret)
}

// IgnoresRequest encapsulates a single ignore rule that is submitted for addition or update.
type IgnoresRequest struct {
	Duration string `json:"duration"`
	Filter   string `json:"filter"`
	Note     string `json:"note"`
}

// JsonIgnoresHandler returns the current ignore rules in JSON format.
func (wh *WebHandlers) JsonIgnoresHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ignores := []*ignore.IgnoreRule{}
	var err error
	ignores, err = wh.Storages.IgnoreStore.List(true)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve ignored traces.")
		return
	}

	// TODO(stephana): Wrap in response envelope if it makes sense !
	enc := json.NewEncoder(w)
	if err := enc.Encode(ignores); err != nil {
		sklog.Errorf("Failed to write or encode result: %s", err)
	}
}

// JsonIgnoresUpdateHandler updates an existing ignores rule.
func (wh *WebHandlers) JsonIgnoresUpdateHandler(w http.ResponseWriter, r *http.Request) {
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
	if err := parseJson(r, req); err != nil {
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
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to create ignore rule.")
		return
	}
	ignoreRule.ID = id

	err = wh.Storages.IgnoreStore.Update(id, ignoreRule)
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to update ignore rule.")
		return
	}

	// If update worked just list the current ignores and return them.
	wh.JsonIgnoresHandler(w, r)
}

// JsonIgnoresDeleteHandler deletes an existing ignores rule.
func (wh *WebHandlers) JsonIgnoresDeleteHandler(w http.ResponseWriter, r *http.Request) {
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

	if _, err = wh.Storages.IgnoreStore.Delete(id); err != nil {
		httputils.ReportError(w, r, err, "Unable to delete ignore rule.")
	} else {
		// If delete worked just list the current ignores and return them.
		wh.JsonIgnoresHandler(w, r)
	}
}

// JsonIgnoresAddHandler is for adding a new ignore rule.
func (wh *WebHandlers) JsonIgnoresAddHandler(w http.ResponseWriter, r *http.Request) {
	user := login.LoggedInAs(r)
	if user == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to add an ignore rule.")
		return
	}
	req := &IgnoresRequest{}
	if err := parseJson(r, req); err != nil {
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
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to create ignore rule.")
		return
	}

	if err = wh.Storages.IgnoreStore.Create(ignoreRule); err != nil {
		httputils.ReportError(w, r, err, "Failed to create ignore rule.")
		return
	}

	wh.JsonIgnoresHandler(w, r)
}

// TriageRequest is the form of the JSON posted to jsonTriageHandler.
type TriageRequest struct {
	// TestDigestStatus maps status to test name and digests as: map[testName][digest]status
	TestDigestStatus map[string]map[string]string `json:"testDigestStatus"`

	// Issue is the id of the code review issue for which we want to change the expectations.
	Issue int64 `json:"issue"`
}

// JsonTriageHandler handles a request to change the triage status of one or more
// digests of one test.
//
// It accepts a POST'd JSON serialization of TriageRequest and updates
// the expectations.
func (wh *WebHandlers) JsonTriageHandler(w http.ResponseWriter, r *http.Request) {
	user := login.LoggedInAs(r)
	if user == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to triage.")
		return
	}

	req := &TriageRequest{}
	if err := parseJson(r, req); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse JSON request.")
		return
	}
	sklog.Infof("Triage request: %#v", req)

	var tc types.TestExp

	// Build the expectations change request from the list of digests passed in.
	tc = make(types.TestExp, len(req.TestDigestStatus))
	for test, digests := range req.TestDigestStatus {
		labeledDigests := make(map[string]types.Label, len(digests))
		for d, label := range digests {
			if !types.ValidLabel(label) {
				httputils.ReportError(w, r, nil, "Receive invalid label in triage request.")
				return
			}
			labeledDigests[d] = types.LabelFromString(label)
		}
		tc[test] = labeledDigests
	}

	// Use the expectations store for the master branch, unless an issue was given
	// in the request, then get the expectations store for the issue.
	expStore := wh.Storages.ExpectationsStore
	if req.Issue > 0 {
		expStore = wh.Storages.IssueExpStoreFactory(req.Issue)
	}

	// Add the change.
	if err := expStore.AddChange(tc, user); err != nil {
		httputils.ReportError(w, r, err, "Failed to store the updated expectations.")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(map[string]string{}); err != nil {
		sklog.Errorf("Failed to write or encode result: %s", err)
	}
}

// JsonStatusHandler returns the current status of with respect to HEAD.
func (wh *WebHandlers) JsonStatusHandler(w http.ResponseWriter, r *http.Request) {
	sendJsonResponse(w, wh.StatusWatcher.GetStatus())
}

// JsonClusterDiffHandler calculates the NxN diffs of all the digests that match
// the incoming query and returns the data in a format appropriate for
// handling in d3.
func (wh *WebHandlers) JsonClusterDiffHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract the test name as we only allow clustering within a test.
	q := search.Query{Limit: 50}
	if err := search.ParseQuery(r, &q); err != nil {
		httputils.ReportError(w, r, err, "Unable to parse query parameter.")
		return
	}
	testName := q.Query.Get(types.PRIMARY_KEY_FIELD)
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

	// TODO(stephana): Check if this is still necessary.
	// // Sort the digests so they are displayed with untriaged last, which means
	// // they will be displayed 'on top', because in SVG document order is z-order.
	// sort.Sort(SearchDigestSlice(searchResponse.Digests))

	digests := []string{}
	for _, digest := range searchResponse.Digests {
		digests = append(digests, digest.Digest)
	}

	digestIndex := map[string]int{}
	for i, d := range digests {
		digestIndex[d] = i
	}

	d3 := ClusterDiffResult{
		Test:             testName,
		Nodes:            []Node{},
		Links:            []Link{},
		ParamsetByDigest: map[string]map[string][]string{},
		ParamsetsUnion:   map[string][]string{},
	}
	for i, d := range searchResponse.Digests {
		d3.Nodes = append(d3.Nodes, Node{
			Name:   d.Digest,
			Status: d.Status,
		})
		remaining := digests[i:]
		diffs, err := wh.Storages.DiffStore.Get(diff.PRIORITY_NOW, d.Digest, remaining)
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
		d3.ParamsetByDigest[d.Digest] = idx.GetParamsetSummary(d.Test, d.Digest, false)
		for _, p := range d3.ParamsetByDigest[d.Digest] {
			sort.Strings(p)
		}
		d3.ParamsetsUnion = util.AddParamSetToParamSet(d3.ParamsetsUnion, d3.ParamsetByDigest[d.Digest])
	}

	for _, p := range d3.ParamsetsUnion {
		sort.Strings(p)
	}

	sendJsonResponse(w, d3)
}

// SearchDigestSlice is for sorting search.Digest's in the order of digest status.
type SearchDigestSlice []*search.Digest

func (p SearchDigestSlice) Len() int { return len(p) }
func (p SearchDigestSlice) Less(i, j int) bool {
	if p[i].Status == p[j].Status {
		return p[i].Digest < p[j].Digest
	} else {
		// Alphabetical order, so neg, pos, unt.
		return p[i].Status < p[j].Status
	}
}
func (p SearchDigestSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// Node represents a single node in a d3 diagram. Used in ClusterDiffResult.
type Node struct {
	Name   string `json:"name"`
	Status string `json:"status"`
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

	Test             string                         `json:"test"`
	ParamsetByDigest map[string]map[string][]string `json:"paramsetByDigest"`
	ParamsetsUnion   map[string][]string            `json:"paramsetsUnion"`
}

// JsonListTestsHandler returns a JSON list with high level information about
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
func (wh *WebHandlers) JsonListTestsHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the query object like with the other searches.
	query := search.Query{}
	if err := search.ParseQuery(r, &query); err != nil {
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
	corpus, hasSourceType := query.Query[types.CORPUS_FIELD]
	sumSlice := []*summary.Summary{}
	if !query.IncludeIgnores && query.Head && len(query.Query) == 1 && hasSourceType {
		sumMap := idx.GetSummaries(false)
		for _, s := range sumMap {
			if util.In(s.Corpus, corpus) && includeSummary(s, &query) {
				sumSlice = append(sumSlice, s)
			}
		}
	} else {
		sklog.Infof("%q %q %q", r.FormValue("query"), r.FormValue("include"), r.FormValue("head"))
		sumMap, err := idx.CalcSummaries(nil, query.Query, query.IncludeIgnores, query.Head)
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to calculate summaries.")
			return
		}
		for _, s := range sumMap {
			if includeSummary(s, &query) {
				sumSlice = append(sumSlice, s)
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
func includeSummary(s *summary.Summary, q *search.Query) bool {
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

// JsonListFailureHandler returns the digests that have failed to load.
func (wh *WebHandlers) JsonListFailureHandler(w http.ResponseWriter, r *http.Request) {
	unavailable := wh.Storages.DiffStore.UnavailableDigests()
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
	sendJsonResponse(w, &ret)
}

// JsonClearFailureHandler removes failing digests from the local cache and
// returns the current failures.
func (wh *WebHandlers) JsonClearFailureHandler(w http.ResponseWriter, r *http.Request) {
	if !wh.purgeDigests(w, r) {
		return
	}
	wh.JsonListFailureHandler(w, r)
}

// JsonClearDigests clears digests from the local cache and GS.
func (wh *WebHandlers) JsonClearDigests(w http.ResponseWriter, r *http.Request) {
	if !wh.purgeDigests(w, r) {
		return
	}
	sendJsonResponse(w, &struct{}{})
}

// purgeDigests removes digests from the local cache and from GS if a query argument is set.
// Returns true if there was no error sent to the response writer.
func (wh *WebHandlers) purgeDigests(w http.ResponseWriter, r *http.Request) bool {
	user := login.LoggedInAs(r)
	if user == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to clear digests.")
		return false
	}

	digests := []string{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&digests); err != nil {
		httputils.ReportError(w, r, err, "Unable to decode digest list.")
		return false
	}
	purgeGCS := r.URL.Query().Get("purge") == "true"

	if err := wh.Storages.DiffStore.PurgeDigests(digests, purgeGCS); err != nil {
		httputils.ReportError(w, r, err, "Unable to clear digests.")
		return false
	}
	return true
}

// JsonTriageLogHandler returns the entries in the triagelog paginated
// in reverse chronological order.
func (wh *WebHandlers) JsonTriageLogHandler(w http.ResponseWriter, r *http.Request) {
	// Get the pagination params.
	var logEntries []*expstorage.TriageLogEntry
	var total int

	q := r.URL.Query()
	offset, size, err := httputils.PaginationParams(q, 0, DEFAULT_PAGE_SIZE, MAX_PAGE_SIZE)
	if err == nil {
		validate := shared.Validation{}
		issue := validate.Int64Value("issue", q.Get("issue"), 0)
		if err := validate.Errors(); err != nil {
			httputils.ReportError(w, r, err, "Unable to retrieve triage log.")
			return
		}

		details := q.Get("details") == "true"
		expStore := wh.Storages.ExpectationsStore
		if issue > 0 {
			expStore = wh.Storages.IssueExpStoreFactory(issue)
		}

		logEntries, total, err = expStore.QueryLog(offset, size, details)
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

	sendResponse(w, logEntries, http.StatusOK, pagination)
}

// JsonTriageUndoHandler performs an "undo" for a given change id.
// The change id's are returned in the result of jsonTriageLogHandler.
// It accepts one query parameter 'id' which is the id if the change
// that should be reversed.
// If successful it returns the same result as a call to jsonTriageLogHandler
// to reflect the changed triagelog.
func (wh *WebHandlers) JsonTriageUndoHandler(w http.ResponseWriter, r *http.Request) {
	// Get the user and make sure they are logged in.
	user := login.LoggedInAs(r)
	if user == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to change expectations.")
		return
	}

	// Extract the id to undo.
	changeID, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		httputils.ReportError(w, r, err, "Invalid change id.")
		return
	}

	// Do the undo procedure.
	_, err = wh.Storages.ExpectationsStore.UndoChange(changeID, user)
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to undo.")
		return
	}

	// Send the same response as a query for the first page.
	wh.JsonTriageLogHandler(w, r)
}

// JsonParamsHandler returns the union of all parameters.
func (wh *WebHandlers) JsonParamsHandler(w http.ResponseWriter, r *http.Request) {
	tile := wh.Indexer.GetIndex().CpxTile().GetTile(true)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tile.ParamSet); err != nil {
		sklog.Errorf("Failed to write or encode result: %s", err)
	}
}

// JsonParamsHandler returns the most current commits.
func (wh *WebHandlers) JsonCommitsHandler(w http.ResponseWriter, r *http.Request) {
	cpxTile, err := wh.Storages.GetLastTileTrimmed()
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to load tile")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cpxTile.DataCommits()); err != nil {
		sklog.Errorf("Failed to write or encode result: %s", err)
	}
}

// textAllHashesHandler returns the list of all hashes we currently know about
// regardless of triage status.
// Endpoint used by the buildbots to avoid transferring already known images.
func (wh *WebHandlers) TextAllHashesHandler(w http.ResponseWriter, r *http.Request) {
	unavailableDigests := wh.Storages.DiffStore.UnavailableDigests()

	idx := wh.Indexer.GetIndex()
	byTest := idx.TalliesByTest(true)
	hashes := map[string]bool{}
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
// This is used by the bots to avoid transferring already known images.
func (wh *WebHandlers) TextKnownHashesProxy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	if err := wh.Storages.GStorageClient.LoadKnownDigests(w); err != nil {
		sklog.Errorf("Failed to copy the known hashes from GCS: %s", err)
		return
	}
}

// JsonCompareTestHandler returns a JSON description for the given test.
// The result is intended to be displayed in a grid-like fashion.
//
// Input format of a POST request:
//
// Output format in JSON:
//
//
func (wh *WebHandlers) JsonCompareTestHandler(w http.ResponseWriter, r *http.Request) {
	// Note that testName cannot be empty by definition of the route that got us here.
	var ctQuery search.CTQuery
	if err := search.ParseCTQuery(r.Body, 5, &ctQuery); err != nil {
		httputils.ReportError(w, r, err, err.Error())
		return
	}

	compareResult, err := search.CompareTest(&ctQuery, wh.Storages, wh.Indexer.GetIndex())
	if err != nil {
		httputils.ReportError(w, r, err, "Search for digests failed.")
		return
	}
	sendJsonResponse(w, compareResult)
}

// JsonBaselineHandler returns a JSON representation of that baseline including
// baselines for a options issue. It can respond to requests like these:
//    /json/baseline
//    /json/baseline/64789
// where the latter contains the issue id for which we would like to retrieve
// the baseline. In that case the returned options will be blend of the master
// baseline and the baseline defined for the issue (usually based on tryjob
// results).
func (wh *WebHandlers) JsonBaselineHandler(w http.ResponseWriter, r *http.Request) {
	commitHash := ""
	issueID := int64(0)
	var err error
	if issueIDStr, ok := mux.Vars(r)["issue_id"]; ok {
		issueID, err = strconv.ParseInt(issueIDStr, 10, 64)
		if err != nil {
			httputils.ReportError(w, r, err, "Issue ID must be valid integer.")
			return
		}
	} else {
		// Since this was not called for an issue, we need to extract a Git hash.
		var ok bool
		if commitHash, ok = mux.Vars(r)["commit_hash"]; !ok {
			msg := "No commit hash provided to fetch expectations"
			httputils.ReportError(w, r, skerr.Fmt(msg), msg)
			return
		}
	}

	baseline, err := wh.Storages.Baseliner.FetchBaseline(commitHash, issueID, 0)
	if err != nil {
		httputils.ReportError(w, r, err, "Fetching baselines failed.")
		return
	}

	sendJsonResponse(w, baseline)
}

// JsonRefreshIssue forces a refresh of a Gerrit issue, i.e. reload data that
// might not have been polled yet etc.
func (wh *WebHandlers) JsonRefreshIssue(w http.ResponseWriter, r *http.Request) {
	user := login.LoggedInAs(r)
	if user == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to refresh tryjob data")
		return
	}

	issueID := int64(0)
	var err error
	issueIDStr, ok := mux.Vars(r)["id"]
	if ok {
		issueID, err = strconv.ParseInt(issueIDStr, 10, 64)
		if err != nil {
			httputils.ReportError(w, r, err, "Issue ID must be valid integer.")
			return
		}
	}

	if err := wh.Storages.TryjobMonitor.ForceRefresh(issueID); err != nil {
		httputils.ReportError(w, r, err, "Refreshing issue failed.")
		return
	}
	sendJsonResponse(w, map[string]string{})
}

// MakeResourceHandler creates a static file handler that sets a caching policy.
func MakeResourceHandler(resourceDir string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(resourceDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}
