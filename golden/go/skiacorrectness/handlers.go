package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/trybot"
	"go.skia.org/infra/golden/go/types"
)

// TODO(stephana): once the byBlameHandler is removed, refactor this to
// remove the redundant types ByBlameEntry and ByBlame.

// jsonByBlameHandler returns a json object with the digests to be triaged grouped by blamelist.
func jsonByBlameHandler(w http.ResponseWriter, r *http.Request) {
	tile, sum, err := allUntriagedSummaries()
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
			dist := blamer.GetBlame(test, d, commits)
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

// jsonSearchHandler is the endpoint for all searches.
func jsonSearchHandler(w http.ResponseWriter, r *http.Request) {
	query := search.Query{Limit: 50}
	if err := parseQuery(r, &query); err != nil {
		httputils.ReportError(w, r, err, "Search for digests failed.")
		return
	}

	searchResponse, err := search.Search(&query, storages, tallies, blamer, paramsetSum)
	if err != nil {
		httputils.ReportError(w, r, err, "Search for digests failed.")
		return
	}
	sendJsonResponse(w, &SearchResult{
		Digests: searchResponse.Digests,
		Commits: searchResponse.Commits,
		Issue:   adaptIssueResponse(searchResponse.IssueResponse),
	})
}

// SearchResult encapsulates the results of a search request.
type SearchResult struct {
	Digests    []*search.Digest   `json:"digests"`
	Commits    []*tiling.Commit   `json:"commits"`
	Issue      *IssueSearchResult `json:"issue"`
	NumMatches int
}

// TODO (stephana): Replace search.IssueResponse with IssueSearchResult
// as soon as the search2Handler is retired.

// IssueSearchResult is the (temporary) output struct for search.IssueResponse.
type IssueSearchResult struct {
	*trybot.Issue

	// Override the Patchsets field of trybot.Issue to contain a list of PatchsetDetails.
	Patchsets []*trybot.PatchsetDetail `json:"patchsets"`

	// QueryPatchsets contains the list of patchsets that are included in the returned digests.
	QueryPatchsets []string `json:"queryPatchsets"`
}

func adaptIssueResponse(ir *search.IssueResponse) *IssueSearchResult {
	if ir == nil {
		return nil
	}

	// Create a list of PatchsetDetails in the same order as the patchsets in the issue.
	patchSets := make([]*trybot.PatchsetDetail, 0, len(ir.IssueDetails.PatchsetDetails))
	for _, pid := range ir.Patchsets {
		if pSet, ok := ir.PatchsetDetails[pid]; ok {
			patchSets = append(patchSets, pSet)
		}
	}

	return &IssueSearchResult{
		Issue:          ir.IssueDetails.Issue,
		Patchsets:      patchSets,
		QueryPatchsets: ir.QueryPatchsets,
	}
}

// TODO(stephana): Remove polyDiffJSONDigestHandler and polyDetailsHandler once all
// detail and diff request go through jsonDetailsHandler and jsonDiffHandler.

// jsonDetailsHandler returns the details about a single digest.
func jsonDetailsHandler(w http.ResponseWriter, r *http.Request) {
	// Extract: test, digest.
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse form values")
		return
	}
	test := r.Form.Get("test")
	digest := r.Form.Get("digest")
	if test == "" || digest == "" {
		httputils.ReportError(w, r, fmt.Errorf("Some query parameters are missing: %q %q", test, digest), "Missing query parameters.")
		return
	}

	ret, err := search.GetDigestDetails(test, digest, storages, paramsetSum, tallies)
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to get digest details.")
		return
	}
	sendJsonResponse(w, ret)
}

// jsonDiffHandler returns difference between two digests.
func jsonDiffHandler(w http.ResponseWriter, r *http.Request) {
	// Extract: test, left, right where left and right are digests.
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse form values")
		return
	}
	test := r.Form.Get("test")
	left := r.Form.Get("left")
	right := r.Form.Get("right")
	if test == "" || left == "" || right == "" {
		httputils.ReportError(w, r, fmt.Errorf("Some query parameters are missing: %q %q %q", test, left, right), "Missing query parameters.")
		return
	}

	ret, err := search.CompareDigests(test, left, right, storages, paramsetSum)
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

// jsonIgnoresHandler returns the current ignore rules in JSON format.
func jsonIgnoresHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ignores := []*ignore.IgnoreRule{}
	var err error
	ignores, err = storages.IgnoreStore.List(true)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve ignored traces.")
	}

	// TODO(stephana): Wrap in response envelope if it makes sense !
	enc := json.NewEncoder(w)
	if err := enc.Encode(ignores); err != nil {
		glog.Errorf("Failed to write or encode result: %s", err)
	}
}

// jsonIgnoresUpdateHandler updates an existing ignores rule.
func jsonIgnoresUpdateHandler(w http.ResponseWriter, r *http.Request) {
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
	ignoreRule.ID = int(id)

	err = storages.IgnoreStore.Update(int(id), ignoreRule)
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to update ignore rule.")
	} else {
		// If update worked just list the current ignores and return them.
		jsonIgnoresHandler(w, r)
	}
}

// jsonIgnoresDeleteHandler deletes an existing ignores rule.
func jsonIgnoresDeleteHandler(w http.ResponseWriter, r *http.Request) {
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

	if _, err = storages.IgnoreStore.Delete(int(id), user); err != nil {
		httputils.ReportError(w, r, err, "Unable to delete ignore rule.")
	} else {
		// If delete worked just list the current ignores and return them.
		jsonIgnoresHandler(w, r)
	}
}

// jsonIgnoresAddHandler is for adding a new ignore rule.
func jsonIgnoresAddHandler(w http.ResponseWriter, r *http.Request) {
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

	if err = storages.IgnoreStore.Create(ignoreRule); err != nil {
		httputils.ReportError(w, r, err, "Failed to create ignore rule.")
		return
	}

	jsonIgnoresHandler(w, r)
}

// TriageRequest is the form of the JSON posted to jsonTriageHandler.
type TriageRequest struct {
	Test    string   `json:"test"`
	Digest  []string `json:"digest"`
	Status  string   `json:"status"`
	All     bool     `json:"all"` // Ignore Digest and instead use the query, filter, and include.
	Query   string   `json:"query"`
	Filter  string   `json:"filter"`
	Include bool     `json:"include"` // Include ignored digests.
	Head    bool     `json:"head"`    // Only include digests at head if true.
}

// jsonTriageHandler handles a request to change the triage status of one or more
// digests of one test.
//
// It accepts a POST'd JSON serialization of TriageRequest and updates
// the expectations.
func jsonTriageHandler(w http.ResponseWriter, r *http.Request) {
	req := &TriageRequest{}
	if err := parseJson(r, req); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse JSON request.")
		return
	}
	glog.Infof("Triage request: %#v", req)
	user := login.LoggedInAs(r)
	if user == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to triage.")
		return
	}

	// Build the expecations change request from the list of digests passed in.
	digests := req.Digest

	// Or build the expectations change request from filter, query, and include.
	if req.All {
		exp, err := storages.ExpectationsStore.Get()
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to load expectations.")
			return
		}
		e := exp.Tests[req.Test]
		ii, _, err := imgInfo(req.Filter, req.Query, req.Test, e, -1, req.Include, false, "", "", req.Head)
		digests = []string{}
		for _, d := range ii {
			digests = append(digests, d.Digest)
		}
	}

	// Label the digests.
	labelledDigests := map[string]types.Label{}
	for _, d := range digests {
		labelledDigests[d] = types.LabelFromString(req.Status)
	}

	tc := map[string]types.TestClassification{
		req.Test: labelledDigests,
	}

	// Otherwise update the expectations directly.
	if err := storages.ExpectationsStore.AddChange(tc, user); err != nil {
		httputils.ReportError(w, r, err, "Failed to store the updated expectations.")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(map[string]string{}); err != nil {
		glog.Errorf("Failed to write or encode result: %s", err)
	}
}

// jsonStatusHandler returns the current status of with respect to HEAD.
func jsonStatusHandler(w http.ResponseWriter, r *http.Request) {
	sendJsonResponse(w, statusWatcher.GetStatus())
}

// TODO (stephana): Remove nxnJSONHandler and the D3 struct once the new UI launched.

// jsonClusterDiffHandler calculates the NxN diffs of all the digests that match
// the incoming query and returns the data in a format appropriate for
// handling in d3.
func jsonClusterDiffHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the test name as we only allow clustering within a test.
	q := search.Query{Limit: 50}
	if err := parseQuery(r, &q); err != nil {
		httputils.ReportError(w, r, err, "Unable to parse query parameter.")
		return
	}
	testName := q.Query.Get(types.PRIMARY_KEY_FIELD)
	if testName == "" {
		httputils.ReportError(w, r, fmt.Errorf("test name parameter missing"), "No test name provided.")
		return
	}

	searchResponse, err := search.Search(&q, storages, tallies, blamer, paramsetSum)
	if err != nil {
		httputils.ReportError(w, r, err, "Search for digests failed.")
		return
	}
	// Sort the digests so they are displayed with untriaged last, which means
	// they will be displayed 'on top', because in SVG document order is z-order.
	sort.Sort(SearchDigestSlice(searchResponse.Digests))

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
		remaining := digests[i:len(digests)]
		diffs, err := storages.DiffStore.Get(d.Digest, remaining)
		if err != nil {
			glog.Errorf("Failed to calculate differences: %s", err)
			continue
		}
		for otherDigest, diff := range diffs {
			d3.Links = append(d3.Links, Link{
				Source: digestIndex[d.Digest],
				Target: digestIndex[otherDigest],
				Value:  diff.PixelDiffPercent,
			})
		}
		d3.ParamsetByDigest[d.Digest] = paramsetSum.Get(d.Test, d.Digest, false)
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

// ClusterDiffResult contains the result of comparing all digests within a test.
// It is structured to be easy to render by the D3.js.
type ClusterDiffResult struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`

	Test             string                         `json:"test"`
	ParamsetByDigest map[string]map[string][]string `json:"paramsetByDigest"`
	ParamsetsUnion   map[string][]string            `json:"paramsetsUnion"`
}

// TOOD(stephana): Remove polyListTestsHandler which has a slightly different
// input format with respect to query parameters.

// jsonListTestsHandler returns a JSON list with high level information about
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
func jsonListTestsHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the query object like with the other searches.
	query := search.Query{}
	if err := parseQuery(r, &query); err != nil {
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

	corpus, hasSourceType := query.Query["source_type"]
	sumSlice := []*summary.Summary{}
	if !query.IncludeIgnores && query.Head && len(query.Query) == 1 && hasSourceType {
		sumMap := summaries.Get()
		for _, s := range sumMap {
			if util.In(s.Corpus, corpus) && includeSummary(s, &query) {
				sumSlice = append(sumSlice, s)
			}
		}
	} else {
		glog.Infof("%q %q %q", r.FormValue("query"), r.FormValue("include"), r.FormValue("head"))
		sumMap, err := summaries.CalcSummaries(nil, query.Query, query.IncludeIgnores, query.Head)
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
		glog.Errorf("Failed to write or encode result: %s", err)
	}
}

// includeSummary returns true if the given summary matches the query flags.
func includeSummary(s *summary.Summary, q *search.Query) bool {
	return ((s.Pos > 0) && (q.Pos)) ||
		((s.Neg > 0) && (q.Neg)) ||
		((s.Untriaged > 0) && (q.Unt))
}

// TODO(stephana): Remove queryFromRequest in favor of parseQuery as the generic
// way to parse input parameters for search-like endpoints.
// Remove the "Limit" field and replace with pagination.

// parseQuery parses the request parameters,
func parseQuery(r *http.Request, query *search.Query) error {
	// Get the limit
	if l := r.FormValue("limit"); l != "" {
		limit, err := strconv.Atoi(l)
		if err != nil {
			return fmt.Errorf("Unable to parse a limit of: %s", l)
		}
		query.Limit = limit
	}

	// Parse the query
	var err error
	query.Query = url.Values{}
	if q := r.FormValue("query"); q != "" {
		query.Query, err = url.ParseQuery(q)
		if err != nil {
			return fmt.Errorf("Unable to parse query: %s. Error: %s", q, err)
		}
	}

	// Parse out the patchsets.
	if temp := r.FormValue("patchsets"); temp != "" {
		patchsets := strings.Split(temp, ",")
		query.Patchsets = patchsets
	}

	query.BlameGroupID = r.FormValue("blame")
	query.Pos = r.FormValue("pos") == "true"
	query.Neg = r.FormValue("neg") == "true"
	query.Unt = r.FormValue("unt") == "true"
	query.Head = r.FormValue("head") == "true"
	query.IncludeIgnores = r.FormValue("include") == "true"
	query.Issue = r.FormValue("issue")
	query.IncludeMaster = r.FormValue("master") == "true"

	return nil
}
