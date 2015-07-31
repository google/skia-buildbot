package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digesttools"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
)

const (
	// DEFAULT_PAGE_SIZE is the default page size used for pagination.
	DEFAULT_PAGE_SIZE = 20

	// MAX_PAGE_SIZE is the maximum page size used for pagination.
	MAX_PAGE_SIZE = 100
)

var (
	templates *template.Template
)

func loadTemplates() {
	templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*resourcesDir, "templates/byblame.html"),
		filepath.Join(*resourcesDir, "templates/cluster.html"),
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/ignores.html"),
		filepath.Join(*resourcesDir, "templates/compare.html"),
		filepath.Join(*resourcesDir, "templates/single.html"),
		filepath.Join(*resourcesDir, "templates/diff.html"),
		filepath.Join(*resourcesDir, "templates/help.html"),
		filepath.Join(*resourcesDir, "templates/triagelog.html"),
		filepath.Join(*resourcesDir, "templates/search.html"),
		filepath.Join(*resourcesDir, "templates/search2.html"),
		// Sub templates used by other templates.
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func templateHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if *local {
			loadTemplates()
		}
		if err := templates.ExecuteTemplate(w, name, struct{}{}); err != nil {
			glog.Errorln("Failed to expand template:", err)
		}
	}
}

type SummarySlice []*summary.Summary

func (p SummarySlice) Len() int           { return len(p) }
func (p SummarySlice) Less(i, j int) bool { return p[i].Untriaged > p[j].Untriaged }
func (p SummarySlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// polyListTestsHandler returns a JSON list with high level information about
// each test.
//
// Takes two query parameters:
//  include - True if ignored digests should be included. (true, false)
//  query   - A query to restrict the responses to, encoded as a URL encoded paramset.
//  head    - True if only digest that appear at head should be included.
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
func polyListTestsHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		util.ReportError(w, r, err, "Failed to parse form data.")
		return
	}
	// If the query only includes source_type parameters, and include==false, then we can just
	// filter the response from summaries.Get(). If the query is broader than that, or
	// include==true, then we need to call summaries.CalcSummaries().
	if err := r.ParseForm(); err != nil {
		util.ReportError(w, r, err, "Invalid request.")
		return
	}
	q, err := url.ParseQuery(r.FormValue("query"))
	if err != nil {
		util.ReportError(w, r, err, "Invalid query in request.")
	}
	_, hasSourceType := q["source_type"]
	sumSlice := []*summary.Summary{}
	if r.FormValue("include") == "false" && r.FormValue("head") == "true" && len(q) == 1 && hasSourceType {
		sumMap := summaries.Get()
		corpus := q["source_type"]
		for _, s := range sumMap {
			if util.In(s.Corpus, corpus) {
				sumSlice = append(sumSlice, s)
			}
		}
	} else {
		glog.Infof("%q %q %q", r.FormValue("query"), r.FormValue("include"), r.FormValue("head"))
		sumMap, err := summaries.CalcSummaries(nil, r.FormValue("query"), r.FormValue("include") == "true", r.FormValue("head") == "true")
		if err != nil {
			util.ReportError(w, r, err, "Failed to calculate summaries.")
		}
		for _, s := range sumMap {
			sumSlice = append(sumSlice, s)
		}
	}
	sort.Sort(SummarySlice(sumSlice))
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(sumSlice); err != nil {
		glog.Errorf("Failed to write or encode result: %s", err)
	}
}

// polyTestStatusHandler returns the status of the requested test.
func polyTestStatusHandler(w http.ResponseWriter, r *http.Request) {
	test := mux.Vars(r)["test"]
	var summary *summary.Summary
	var ok bool
	if summary, ok = summaries.Get()[test]; !ok {
		util.ReportError(w, r, fmt.Errorf("Unknown test: %q", test), "No summaries for test.")
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(summary); err != nil {
		glog.Errorf("Failed to write or encode result: %s", err)
	}
}

// polyIgnoresJSONHandler returns the current ignore rules in JSON f ormat.
func polyIgnoresJSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ignores := []*ignore.IgnoreRule{}
	var err error
	ignores, err = storages.IgnoreStore.List()
	if err != nil {
		util.ReportError(w, r, err, "Failed to retrieve ignored traces.")
	}

	// TODO(stephana): Wrap in response envelope if it makes sense !
	enc := json.NewEncoder(w)
	if err := enc.Encode(ignores); err != nil {
		glog.Errorf("Failed to write or encode result: %s", err)
	}
}

func polyIgnoresUpdateHandler(w http.ResponseWriter, r *http.Request) {
	user := login.LoggedInAs(r)
	if user == "" {
		util.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to update an ignore rule.")
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 0)
	if err != nil {
		util.ReportError(w, r, err, "ID must be valid integer.")
		return
	}
	req := &IgnoresRequest{}
	if err := parseJson(r, req); err != nil {
		util.ReportError(w, r, err, "Failed to parse submitted data.")
		return
	}
	if req.Filter == "" {
		util.ReportError(w, r, fmt.Errorf("Invalid Filter: %q", req.Filter), "Filters can't be empty.")
		return
	}
	d, err := human.ParseDuration(req.Duration)
	if err != nil {
		util.ReportError(w, r, err, "Failed to parse duration")
		return
	}
	ignoreRule := ignore.NewIgnoreRule(user, time.Now().Add(d), req.Filter, req.Note)
	if err != nil {
		util.ReportError(w, r, err, "Failed to create ignore rule.")
		return
	}
	ignoreRule.ID = int(id)

	err = storages.IgnoreStore.Update(int(id), ignoreRule)
	if err != nil {
		util.ReportError(w, r, err, "Unable to update ignore rule.")
	} else {
		// If update worked just list the current ignores and return them.
		polyIgnoresJSONHandler(w, r)
	}
}

func polyIgnoresDeleteHandler(w http.ResponseWriter, r *http.Request) {
	user := login.LoggedInAs(r)
	if user == "" {
		util.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to add an ignore rule.")
		return
	}
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 0)
	if err != nil {
		util.ReportError(w, r, err, "ID must be valid integer.")
		return
	}

	if _, err = storages.IgnoreStore.Delete(int(id), user); err != nil {
		util.ReportError(w, r, err, "Unable to delete ignore rule.")
	} else {
		// If delete worked just list the current ignores and return them.
		polyIgnoresJSONHandler(w, r)
	}
}

type IgnoresRequest struct {
	Duration string `json:"duration"`
	Filter   string `json:"filter"`
	Note     string `json:"note"`
}

var durationRe = regexp.MustCompile("([0-9]+)([smhdw])")

// polyIgnoresAddHandler is for adding a new ignore rule.
func polyIgnoresAddHandler(w http.ResponseWriter, r *http.Request) {
	user := login.LoggedInAs(r)
	if user == "" {
		util.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to add an ignore rule.")
		return
	}
	req := &IgnoresRequest{}
	if err := parseJson(r, req); err != nil {
		util.ReportError(w, r, err, "Failed to parse submitted data.")
		return
	}
	if req.Filter == "" {
		util.ReportError(w, r, fmt.Errorf("Invalid Filter: %q", req.Filter), "Filters can't be empty.")
		return
	}
	d, err := human.ParseDuration(req.Duration)
	if err != nil {
		util.ReportError(w, r, err, "Failed to parse duration")
		return
	}
	ignoreRule := ignore.NewIgnoreRule(user, time.Now().Add(d), req.Filter, req.Note)
	if err != nil {
		util.ReportError(w, r, err, "Failed to create ignore rule.")
		return
	}

	if err = storages.IgnoreStore.Create(ignoreRule); err != nil {
		util.ReportError(w, r, err, "Failed to create ignore rule.")
		return
	}

	polyIgnoresJSONHandler(w, r)
}

// polyDiffJSONDigestHandler takes three parameters (top, left, and test), and
// returns a JSON serialized PolyTestDiffInfo as the response.
func polyDiffJSONDigestHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		util.ReportError(w, r, err, "Failed to parse form values")
		return
	}
	top := r.Form.Get("top")
	left := r.Form.Get("left")
	test := r.Form.Get("test")
	if top == "" || left == "" || test == "" {
		util.ReportError(w, r, fmt.Errorf("Some query parameters are missing: %q %q %q", top, left, test), "Missing query parameters.")
		return
	}

	diffs, err := storages.DiffStore.Get(left, []string{top})
	if err != nil {
		util.ReportError(w, r, err, "Failed to do diffs")
		return
	}
	full := storages.DiffStore.AbsPath([]string{top, left})
	d, ok := diffs[top]
	if !ok {
		util.ReportError(w, r, fmt.Errorf("Failed to calculate diff."), "Failed to calculate diff.")
		return
	}

	ret := PolyTestDiffInfo{
		Test:             test,
		TopDigest:        top,
		LeftDigest:       left,
		NumDiffPixels:    d.NumDiffPixels,
		PixelDiffPercent: d.PixelDiffPercent,
		MaxRGBADiffs:     d.MaxRGBADiffs,
		DiffImg:          pathToURLConverter(d.PixelDiffFilePath),
		TopImg:           pathToURLConverter(full[top]),
		LeftImg:          pathToURLConverter(full[left]),
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(ret); err != nil {
		glog.Errorf("Failed to write or encode result: %s", err)
	}
}

func polySearchJSONHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		util.ReportError(w, r, err, "Invalid request.")
		return
	}
	di, err := summaries.Search(r.FormValue("query"), r.FormValue("include") == "true", r.FormValue("head") == "true", r.FormValue("pos") == "true", r.FormValue("neg") == "true", r.FormValue("unt") == "true")
	if err != nil {
		util.ReportError(w, r, err, "Search failed.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(di); err != nil {
		glog.Errorf("Failed to write or encode result: %s", err)
	}
}

// polyTriageLogHandler returns the entries in the triagelog paginated
// in reverse chronological order.
func polyTriageLogHandler(w http.ResponseWriter, r *http.Request) {
	// Get the pagination params.
	var logEntries []*expstorage.TriageLogEntry
	var total int

	q := r.URL.Query()
	offset, size, err := util.PaginationParams(q, 0, DEFAULT_PAGE_SIZE, MAX_PAGE_SIZE)
	if err == nil {
		details := q.Get("details") == "true"
		logEntries, total, err = storages.ExpectationsStore.QueryLog(offset, size, details)
	}

	if err != nil {
		util.ReportError(w, r, err, "Unable to retrieve triage log.")
		return
	}

	pagination := &util.ResponsePagination{
		Offset: offset,
		Size:   size,
		Total:  total,
	}

	sendResponse(w, logEntries, http.StatusOK, pagination)
}

// triageUndoHandler performs an "undo" for a given change id.
// The change id's are returned in the result of polyTriageLogHandler.
// It accepts one query parameter 'id' which is the id if the change
// that should be reversed.
// If successful it retunrs the same result as a call to polyTriageLogHandler
// to reflect the changed triagelog.
func triageUndoHandler(w http.ResponseWriter, r *http.Request) {
	// Get the user and make sure they are logged in.
	user := login.LoggedInAs(r)
	if user == "" {
		util.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to change expectations.")
		return
	}

	// Extract the id to undo.
	changeID, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		util.ReportError(w, r, err, "Invalid change id.")
		return
	}

	// Do the undo procedure.
	_, err = storages.ExpectationsStore.UndoChange(changeID, user)
	if err != nil {
		util.ReportError(w, r, err, "Unable to undo.")
		return
	}

	// Send the same response as a query for the first page.
	polyTriageLogHandler(w, r)
}

// PolyTestRequest is the POST'd request body handled by polyTestHandler.
type PolyTestRequest struct {
	Test               string `json:"test"`
	TopFilter          string `json:"topFilter"`
	LeftFilter         string `json:"LeftFilter"`
	TopQuery           string `json:"topQuery"`
	LeftQuery          string `json:"leftQuery"`
	TopIncludeIgnores  bool   `json:"topIncludeIgnores"`
	LeftIncludeIgnores bool   `json:"leftIncludeIgnores"`
	TopN               int    `json:"topN"`
	LeftN              int    `json:"leftN"`
	Sort               string `json:"sort"`   // Which side to sort, "top" or "left".
	Dir                string `json:"dir"`    // Direction to sort, ["", "asc", "desc"]
	Digest             string `json:"digest"` // The digest to sort against.
	Head               bool   `json:"head"`   // If true only return digests at head.
}

// PolyTestImgInfo info about a single source digest. Used in PolyTestGUI.
type PolyTestImgInfo struct {
	Digest           string  `json:"digest"`
	N                int     `json:"n"`    // The number of images with this digest.
	PixelDiffPercent float32 `json:"diff"` // Diff from the given digest to compare against, otherwise zero.
}

// PolyTestImgInfoSlice is for sorting slices of PolyTestImgInfo.
type PolyTestImgInfoSlice []*PolyTestImgInfo

func (p PolyTestImgInfoSlice) Len() int { return len(p) }
func (p PolyTestImgInfoSlice) Less(i, j int) bool {
	if p[i].N != p[j].N {
		return p[i].N > p[j].N
	} else {
		return p[i].Digest > p[j].Digest
	}
}
func (p PolyTestImgInfoSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// PolyTestImgInfoDiffAscSlice is for sorting slices of PolyTestImgInfo by PixelDiffPercent.
type PolyTestImgInfoDiffAscSlice []*PolyTestImgInfo

func (p PolyTestImgInfoDiffAscSlice) Len() int { return len(p) }
func (p PolyTestImgInfoDiffAscSlice) Less(i, j int) bool {
	if p[i].PixelDiffPercent != p[j].PixelDiffPercent {
		return p[i].PixelDiffPercent < p[j].PixelDiffPercent
	} else {
		return p[i].Digest < p[j].Digest
	}
}
func (p PolyTestImgInfoDiffAscSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// PolyTestImgInfo info about a single diff between two source digests. Used in
// PolyTestGUI.
type PolyTestDiffInfo struct {
	Test             string  `json:"test"`
	TopDigest        string  `json:"topDigest"`
	LeftDigest       string  `json:"leftDigest"`
	NumDiffPixels    int     `json:"numDiffPixels"`
	PixelDiffPercent float32 `json:"pixelDiffPercent"`
	MaxRGBADiffs     []int   `json:"maxRGBADiffs"`
	DiffImg          string  `json:"diffImgUrl"`
	TopImg           string  `json:"topImgUrl"`
	LeftImg          string  `json:"leftImgUrl"`
}

// PolyTestGUI serialized as JSON is the response body from polyTestHandler.
type PolyTestGUI struct {
	Top       []*PolyTestImgInfo    `json:"top"`
	Left      []*PolyTestImgInfo    `json:"left"`
	Grid      [][]*PolyTestDiffInfo `json:"grid"`
	Message   string                `json:"message"`
	TopTotal  int                   `json:"topTotal"`
	LeftTotal int                   `json:"leftTotal"`
}

// imgInfo returns a populated slice of PolyTestImgInfo based on the filter and
// queryString passed in.
//
// max maybe set to -1, which means to not truncate the response digest slice.
// If sortAgainstHash is true then the result will be sorted in direction 'dir' versus the given 'digest',
// otherwise the results will be sorted in terms of ascending N.
//
// If head is true then only return digests that appear at head.
func imgInfo(filter, queryString, testName string, e types.TestClassification, max int, includeIgnores bool, sortAgainstHash bool, dir string, digest string, head bool) ([]*PolyTestImgInfo, int, error) {
	query, err := url.ParseQuery(queryString)
	if err != nil {
		return nil, 0, fmt.Errorf("Failed to parse Query in imgInfo: %s", err)
	}
	query[types.PRIMARY_KEY_FIELD] = []string{testName}

	t := timer.New("finding digests")
	digests := map[string]int{}
	if head {
		tile, err := storages.GetLastTileTrimmed(includeIgnores)
		if err != nil {
			return nil, 0, fmt.Errorf("Failed to retrieve tallies in imgInfo: %s", err)
		}
		lastCommitIndex := tile.LastCommitIndex()
		for _, tr := range tile.Traces {
			if tiling.Matches(tr, query) {
				for i := lastCommitIndex; i >= 0; i-- {
					if tr.IsMissing(i) {
						continue
					} else {
						digests[tr.(*types.GoldenTrace).Values[i]] = 1
						break
					}
				}
			}
		}
	} else {
		digests, err = tallies.ByQuery(query, includeIgnores)
		if err != nil {
			return nil, 0, fmt.Errorf("Failed to retrieve tallies in imgInfo: %s", err)
		}
	}
	glog.Infof("Num Digests: %d", len(digests))
	t.Stop()

	// If we are going to sort against a digest then we need to calculate
	// the diff metrics against that digest.
	diffMetrics := map[string]*diff.DiffMetrics{}
	if sortAgainstHash {
		digestSlice := make([]string, len(digests))
		for d, _ := range digests {
			digestSlice = append(digestSlice, d)
		}
		var err error
		diffMetrics, err = storages.DiffStore.Get(digest, digestSlice)
		if err != nil {
			return nil, 0, fmt.Errorf("Failed to calculate diffs to sort against: %s", err)
		}
	}

	label := types.LabelFromString(filter)
	// Now filter digests by their expectations status here.
	t = timer.New("apply expectations")
	ret := []*PolyTestImgInfo{}
	for digest, n := range digests {
		if e[digest] != label {
			continue
		}
		p := &PolyTestImgInfo{
			Digest: digest,
			N:      n,
		}
		if sortAgainstHash {
			p.PixelDiffPercent = diffMetrics[digest].PixelDiffPercent
		}
		ret = append(ret, p)
	}
	t.Stop()

	if sortAgainstHash {
		if dir == "asc" {
			sort.Sort(PolyTestImgInfoDiffAscSlice(ret))
		} else {
			sort.Sort(sort.Reverse(PolyTestImgInfoDiffAscSlice(ret)))
		}
	} else {
		sort.Sort(PolyTestImgInfoSlice(ret))
	}

	total := len(ret)
	if max > 0 && len(ret) > max {
		ret = ret[:max]
	}
	return ret, total, nil
}

// polyTestHandler returns a JSON description for the given test.
//
// Takes an JSON encoded POST body of the following form:
//
//   {
//      test: The name of the test.
//      topFilter=["positive", "negative", "untriaged"]
//      leftFilter=["positive", "negative", "untriaged"]
//      topQuery: "",
//      leftQuery: "",
//      topIncludeIgnores: bool,
//      leftIncludeIgnores: bool,
//      topN: topN,
//      leftN: leftN,
//      head: [true, false],
//   }
//
//
// The return format looks like:
//
// {
//   "top": [img1, img2, ...]
//   "left": [imgA, imgB, ...]
//   "grid": [
//     [diff1A, diff2A, ...],
//     [diff1B, diff2B, ...],
//   ],
//   info: "Could be error or warning.",
// }
//
// Where imgN is serialized PolyTestImgInfo, and
//       diffN is a serialized PolyTestDiffInfo struct.
// Note that this format is what res/imp/grid expects to
// receive.
//
func polyTestHandler(w http.ResponseWriter, r *http.Request) {
	req := &PolyTestRequest{}
	if err := parseJson(r, req); err != nil {
		util.ReportError(w, r, err, "Failed to parse JSON request.")
		return
	}
	exp, err := storages.ExpectationsStore.Get()
	if err != nil {
		util.ReportError(w, r, err, "Failed to load expectations.")
		return
	}
	e := exp.Tests[req.Test]

	topDigests, topTotal, err := imgInfo(req.TopFilter, req.TopQuery, req.Test, e, req.TopN, req.TopIncludeIgnores, req.Sort == "top", req.Dir, req.Digest, req.Head)
	leftDigests, leftTotal, err := imgInfo(req.LeftFilter, req.LeftQuery, req.Test, e, req.LeftN, req.LeftIncludeIgnores, req.Sort == "left", req.Dir, req.Digest, req.Head)

	// Extract out string slices of digests to pass to *AbsPath and storages.DiffStore.Get().
	allDigests := map[string]bool{}
	topDigestMap := map[string]bool{}
	for _, d := range topDigests {
		allDigests[d.Digest] = true
		topDigestMap[d.Digest] = true
	}
	for _, d := range leftDigests {
		allDigests[d.Digest] = true
	}

	topDigestSlice := util.KeysOfStringSet(topDigestMap)
	allDigestsSlice := util.KeysOfStringSet(allDigests)
	full := storages.DiffStore.AbsPath(allDigestsSlice)

	grid := [][]*PolyTestDiffInfo{}
	for _, l := range leftDigests {
		row := []*PolyTestDiffInfo{}
		diffs, err := storages.DiffStore.Get(l.Digest, topDigestSlice)
		if err != nil {
			glog.Errorf("Failed to do diffs: %s", err)
			continue
		}
		for _, t := range topDigests {
			d, ok := diffs[t.Digest]
			if !ok {
				glog.Errorf("Failed to find expected diff for: %s", t.Digest)
				d = &diff.DiffMetrics{
					MaxRGBADiffs: []int{},
				}
			}
			row = append(row, &PolyTestDiffInfo{
				Test:             req.Test,
				TopDigest:        t.Digest,
				LeftDigest:       l.Digest,
				NumDiffPixels:    d.NumDiffPixels,
				PixelDiffPercent: d.PixelDiffPercent,
				MaxRGBADiffs:     d.MaxRGBADiffs,
				DiffImg:          pathToURLConverter(d.PixelDiffFilePath),
				TopImg:           pathToURLConverter(full[t.Digest]),
				LeftImg:          pathToURLConverter(full[l.Digest]),
			})
		}
		grid = append(grid, row)
	}

	p := PolyTestGUI{
		Top:       topDigests,
		Left:      leftDigests,
		Grid:      grid,
		TopTotal:  topTotal,
		LeftTotal: leftTotal,
	}
	if len(p.Top) == 0 || len(p.Left) == 0 {
		p.Message = "Failed to find images that match those filters."
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(p); err != nil {
		glog.Errorf("Failed to write or encode result: %s", err)
	}
}

// PolyTriageRequest is the form of the JSON posted to polyTriageHandler.
type PolyTriageRequest struct {
	Test    string   `json:"test"`
	Digest  []string `json:"digest"`
	Status  string   `json:"status"`
	All     bool     `json:"all"` // Ignore Digest and instead use the query, filter, and include.
	Query   string   `json:"query"`
	Filter  string   `json:"filter"`
	Include bool     `json:"include"` // Include ignored digests.
	Head    bool     `json:"head"`    // Only include digests at head if true.
}

// polyTriageHandler handles a request to change the triage status of one or more
// digests of one test.
//
// It accepts a POST'd JSON serialization of PolyTriageRequest and updates
// the expectations.
func polyTriageHandler(w http.ResponseWriter, r *http.Request) {
	req := &PolyTriageRequest{}
	if err := parseJson(r, req); err != nil {
		util.ReportError(w, r, err, "Failed to parse JSON request.")
		return
	}
	glog.Infof("Triage request: %#v", req)
	user := login.LoggedInAs(r)
	if user == "" {
		util.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to triage.")
		return
	}

	// Build the expecations change request from the list of digests passed in.
	digests := req.Digest

	// Or build the expectations change request from filter, query, and include.
	if req.All {
		exp, err := storages.ExpectationsStore.Get()
		if err != nil {
			util.ReportError(w, r, err, "Failed to load expectations.")
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
		util.ReportError(w, r, err, "Failed to store the updated expectations.")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(map[string]string{}); err != nil {
		glog.Errorf("Failed to write or encode result: %s", err)
	}
}

func safeGet(paramset map[string][]string, key string) []string {
	if ret, ok := paramset[key]; ok {
		sort.Strings(ret)
		return ret
	} else {
		return []string{}
	}
}

// PerParamCompare is used in PolyDetailsGUI
type PerParamCompare struct {
	Name string   `json:"name"` // Name of the parameter.
	Top  []string `json:"top"`  // All the parameter values that appear for the top digest.
	Left []string `json:"left"` // All the parameter values that appear for the left digest.
}

// Point is a single point. Used in Plot.
type Point struct {
	X int `json:"x"` // The commit index [0-49].
	Y int `json:"y"`
	S int `json:"s"` // Status of the digest: 0 if the digest matches our search, 1-8 otherwise.
}

type PointSlice []Point

func (p PointSlice) Len() int           { return len(p) }
func (p PointSlice) Less(i, j int) bool { return p[i].X < p[j].X }
func (p PointSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Trace represents a single test over time.
type Trace struct {
	Data   []Point           `json:"data"`  // One Point for each test result.
	Label  string            `json:"label"` // The id of the trace.
	Params map[string]string `json:"params"`
}

// DigestStatus is a digest and its status, used in PolyDetailsGUI.
type DigestStatus struct {
	Digest string `json:"digest"`
	Status string `json:"status"`
}

// PolyDetailsGUI is used in the JSON returned from polyDetailsHandler. It
// represents the known information about a single digest for a given test.
type PolyDetailsGUI struct {
	TopStatus    string                   `json:"topStatus"`
	LeftStatus   string                   `json:"leftStatus"`
	Params       []*PerParamCompare       `json:"params"`
	Traces       []*Trace                 `json:"traces"`
	Commits      []*tiling.Commit         `json:"commits"`
	OtherDigests []*DigestStatus          `json:"otherDigests"`
	TileSize     int                      `json:"tileSize"`
	PosClosest   *digesttools.Closest     `json:"posClosest"`
	NegClosest   *digesttools.Closest     `json:"negClosest"`
	Blame        *blame.BlameDistribution `json:"blame"`

	// Issues is only populated if this is a query for a single digest, i.e. top==left.
	Issues []issues.Issue `json:"issues"`
}

// polyDetailsHandler handles requests about individual digests in a test.
//
// It expects a request with the following query parameters:
//
//   test - The name of the test.
//   top  - A digest in the test.
//   left - A digest in the test.
//   graphs - Boolean that's true if graph data should be returned.
//   closest - Boolean that's true if the closest positive and negative digests should be returned.
//
// The response looks like:
//   {
//     topStatus: "untriaged",
//     leftStatus: "positive",
//     params: [
//       {
//         "name": "config",
//         "top" : ["8888", "565"],
//         "left": ["gpu"],
//       },
//       ...
//     ],
//     traces: [
//        {
//          data: {x: 1, y: 1, s: true}, {x: 5, y: 1, s: true},
//          label: "key1",
//        },
//        ...
//     ]
//   }
//
// TODO(jcgregorio) Add unit tests.
func polyDetailsHandler(w http.ResponseWriter, r *http.Request) {
	tile, err := storages.GetLastTileTrimmed(true)
	if err != nil {
		util.ReportError(w, r, err, "Failed to load tile")
		return
	}
	if err := r.ParseForm(); err != nil {
		util.ReportError(w, r, err, "Failed to parse form values")
		return
	}
	top := r.Form.Get("top")
	left := r.Form.Get("left")
	if top == "" || left == "" {
		util.ReportError(w, r, fmt.Errorf("Missing the top or left query parameter: %s %s", top, left), "No digests specified.")
		return
	}
	test := r.Form.Get("test")
	if test == "" {
		util.ReportError(w, r, fmt.Errorf("Missing the test query parameter."), "No test name specified.")
		return
	}

	exp, err := storages.ExpectationsStore.Get()
	if err != nil {
		util.ReportError(w, r, err, "Failed to load expectations.")
		return
	}

	ret := buildDetailsGUI(tile, exp, test, top, left, r.Form.Get("graphs") == "true", r.Form.Get("closest") == "true", true)

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(ret); err != nil {
		glog.Errorf("Failed to write or encode result: %s", err)
	}
}

func buildDetailsGUI(tile *tiling.Tile, exp *expstorage.Expectations, test string, top string, left string, graphs bool, closest bool, includeIgnores bool) *PolyDetailsGUI {
	ret := &PolyDetailsGUI{
		TopStatus:  exp.Classification(test, top).String(),
		LeftStatus: exp.Classification(test, left).String(),
		Params:     []*PerParamCompare{},
		Traces:     []*Trace{},
		TileSize:   len(tile.Commits),
	}

	topParamSet := paramsetSum.Get(test, top, includeIgnores)
	leftParamSet := paramsetSum.Get(test, left, includeIgnores)

	traceNames := []string{}
	tally := tallies.ByTrace()
	for id, tr := range tile.Traces {
		if tr.Params()[types.PRIMARY_KEY_FIELD] == test {
			traceNames = append(traceNames, id)
		}
	}

	keys := util.UnionStrings(util.KeysOfParamSet(topParamSet), util.KeysOfParamSet(leftParamSet))
	sort.Strings(keys)
	for _, k := range keys {
		ret.Params = append(ret.Params, &PerParamCompare{
			Name: k,
			Top:  safeGet(topParamSet, k),
			Left: safeGet(leftParamSet, k),
		})
	}

	// Now build the trace data.
	if graphs {
		ret.Traces, ret.OtherDigests = buildTraceData(top, traceNames, tile, tally, exp)
		ret.Commits = tile.Commits
		ret.Blame = blamer.GetBlame(test, top, ret.Commits)
	}

	// Now find the closest positive and negative digests.
	t := tallies.ByTest()[test]
	if closest && t != nil {
		ret.PosClosest = digesttools.ClosestDigest(test, top, exp, t, storages.DiffStore, types.POSITIVE)
		ret.NegClosest = digesttools.ClosestDigest(test, top, exp, t, storages.DiffStore, types.NEGATIVE)
	}

	if top == left {
		var err error
		// Search is only done on the digest. Codesite can't seem to extract the
		// name of the test reliably from the URL in comment text, yet can get the
		// digest just fine. This issue should be revisited once we switch to
		// Monorail.
		ret.Issues, err = issueTracker.FromQuery(top)
		if err != nil {
			glog.Errorf("Failed to load issues for [%s, %s]: %s", test, top, err)
		}
	}

	return ret
}

// digestIndex returns the index of the digest d in digestInfo, or -1 if not found.
func digestIndex(d string, digestInfo []*DigestStatus) int {
	for i, di := range digestInfo {
		if di.Digest == d {
			return i
		}
	}
	return -1
}

// buildTraceData returns a populated []*Trace for all the traces that contain 'digest'.
func buildTraceData(digest string, traceNames []string, tile *tiling.Tile, traceTally map[string]tally.Tally, exp *expstorage.Expectations) ([]*Trace, []*DigestStatus) {
	sort.Strings(traceNames)
	ret := []*Trace{}
	last := tile.LastCommitIndex()
	y := 0

	// Keep track of the first 7 non-matching digests we encounter so we can color them differently.
	otherDigests := []*DigestStatus{}
	// Populate otherDigests with all the digests, including the one we are comparing against.
	if len(traceNames) > 0 {
		// Find the test name so we can look up the triage status.
		trace := tile.Traces[traceNames[0]].(*types.GoldenTrace)
		test := trace.Params()[types.PRIMARY_KEY_FIELD]
		otherDigests = append(otherDigests, &DigestStatus{
			Digest: digest,
			Status: exp.Classification(test, digest).String(),
		})
	}
	for _, id := range traceNames {
		t, ok := traceTally[id]
		if !ok {
			continue
		}
		if count, ok := t[digest]; !ok || count == 0 {
			continue
		}
		trace := tile.Traces[id].(*types.GoldenTrace)
		p := &Trace{
			Data:   []Point{},
			Label:  id,
			Params: trace.Params(),
		}
		for i := last; i >= 0; i-- {
			if trace.IsMissing(i) {
				continue
			}
			// s is the status of the digest, it is either 0 for a match, or [1-8] if not.
			s := 0
			if trace.Values[i] != digest {
				if index := digestIndex(trace.Values[i], otherDigests); index != -1 {
					s = index
				} else {
					if len(otherDigests) < 9 {
						d := trace.Values[i]
						test := trace.Params()[types.PRIMARY_KEY_FIELD]
						otherDigests = append(otherDigests, &DigestStatus{
							Digest: d,
							Status: exp.Classification(test, d).String(),
						})
						s = len(otherDigests) - 1
					} else {
						s = 8
					}
				}
			}
			p.Data = append(p.Data, Point{
				X: i,
				Y: y,
				S: s,
			})
		}
		sort.Sort(PointSlice(p.Data))
		ret = append(ret, p)
		y += 1
	}

	return ret, otherDigests
}

func polyParamsHandler(w http.ResponseWriter, r *http.Request) {
	tile, err := storages.GetLastTileTrimmed(false)
	if err != nil {
		util.ReportError(w, r, err, "Failed to load tile")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(tile.ParamSet); err != nil {
		glog.Errorf("Failed to write or encode result: %s", err)
	}
}

// polyAllHashesHandler returns the list of all hashes we currently know about regardless of triage status.
//
// Endpoint used by the Android buildbots to avoid transferring already known images.
func polyAllHashesHandler(w http.ResponseWriter, r *http.Request) {
	byTest := tallies.ByTest()
	hashes := map[string]bool{}
	for _, test := range byTest {
		for k, _ := range test {
			hashes[k] = true
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	for k, _ := range hashes {
		if _, err := w.Write([]byte(k)); err != nil {
			glog.Errorf("Failed to write or encode result: %s", err)
			return
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			glog.Errorf("Failed to write or encode result: %s", err)
			return
		}
	}
}

// polyStatusHandler returns the current status of with respect to
// HEAD.
func polyStatusHandler(w http.ResponseWriter, r *http.Request) {
	sendJsonResponse(w, statusWatcher.GetStatus())
}

// allUntriagedSummaries returns a tile and summaries for all untriaged GMs.
//
// TODO(jcgregorio) Make source_type selectable.
func allUntriagedSummaries() (*tiling.Tile, map[string]*summary.Summary, error) {
	tile, err := storages.GetLastTileTrimmed(true)
	if err != nil {
		return nil, nil, fmt.Errorf("Couldn't load tile: %s", err)
	}
	// Get a list of all untriaged images by test.
	sum, err := summaries.CalcSummaries([]string{}, "source_type=gm", false, true)
	if err != nil {
		return nil, nil, fmt.Errorf("Couldn't load summaries: %s", err)
	}
	return tile, sum, nil
}

// ByBlame describes a single digest and it's blames.
type ByBlame struct {
	Test          string                   `json:"test"`
	Digest        string                   `json:"digest"`
	Blame         *blame.BlameDistribution `json:"blame"`
	CommitIndices []int                    `json:"commit_indices"`
	Key           string
}

// lookUpCommits returns the commit hashes for the commit indices in 'freq'.
func lookUpCommits(freq []int, commits []*tiling.Commit) []string {
	ret := []string{}
	for _, index := range freq {
		ret = append(ret, commits[index].Hash)
	}
	return ret
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

// byBlameHandler returns a page with the digests to be triaged grouped by blamelist.
func byBlameHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	tile, sum, err := allUntriagedSummaries()
	commits := tile.Commits
	if err != nil {
		util.ReportError(w, r, err, "Failed to load summaries.")
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

	// The Commit info needs to be accessed via Javascript, so serialize it into
	// JSON here.
	commitinfojs, err := json.MarshalIndent(commitinfo, "", "  ")
	if err != nil {
		util.ReportError(w, r, err, "Failed to encode response data.")
		return
	}

	keys := []string{}
	for groupid, _ := range grouped {
		keys = append(keys, groupid)
	}
	sort.Strings(keys)

	if err := templates.ExecuteTemplate(w, "byblame.html",
		struct {
			Keys      []string
			ByBlame   map[string][]*ByBlame
			CommitsJS template.JS

			// map [groupid] [testname]
			TestRollups map[string]map[string]*TestRollup
		}{
			Keys:        keys,
			ByBlame:     grouped,
			CommitsJS:   template.JS(string(commitinfojs)),
			TestRollups: rollups,
		}); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

func search2Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	digests, numMatches, commits, err := search.Search(queryFromRequest(r), storages, tallies, blamer, paramsetSum)
	if err != nil {
		util.ReportError(w, r, err, "Search for digests failed.")
	}
	js, err := json.MarshalIndent(digests, "", "  ")
	if err != nil {
		util.ReportError(w, r, err, "Failed to encode response data.")
		return
	}
	commitsjs, err := json.MarshalIndent(commits, "", "  ")
	if err != nil {
		util.ReportError(w, r, err, "Failed to encode commits.")
		return
	}

	context := struct {
		Digests    []*search.Digest
		JS         template.JS
		CommitsJS  template.JS
		NumMatches int
	}{
		Digests:    digests,
		JS:         template.JS(string(js)),
		CommitsJS:  template.JS(string(commitsjs)),
		NumMatches: numMatches,
	}

	if err := templates.ExecuteTemplate(w, "search2.html", context); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

func queryFromRequest(r *http.Request) *search.Query {
	Limit := 50
	if l := r.FormValue("limit"); l != "" {
		if li, err := strconv.Atoi(l); err != nil {
			glog.Errorf("Unable to parse a limit of: %s", l)
		} else {
			Limit = li
		}
	}
	return &search.Query{
		BlameGroupID:   r.FormValue("blame"),
		Pos:            r.FormValue("pos") == "true",
		Neg:            r.FormValue("neg") == "true",
		Unt:            r.FormValue("unt") == "true",
		Head:           r.FormValue("head") == "true",
		IncludeIgnores: r.FormValue("include") == "true",
		Query:          r.FormValue("query"),
		Limit:          Limit,
	}
}

// Node represents a single node in a d3 diagram. Used in D3.
type Node struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Link represents a link between d3 nodes, used in D3.
type Link struct {
	Source int     `json:"source"`
	Target int     `json:"target"`
	Value  float32 `json:"value"`
}

// D3 represents the data structure to pass to a d3 force layout object.
type D3 struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`

	// map [digest] paramset
	Paramsets map[string]map[string][]string `json:"paramsets"`
	Paramset  map[string][]string            `json:"paramset"`
}

// nxnJSONHandler calculates the NxN diffs of all the digests that match
// the incoming query and returns the data in a format appropriate for
// handling in d3.
func nxnJSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	digestsInfo, _, _, err := search.Search(queryFromRequest(r), storages, tallies, blamer, paramsetSum)
	if err != nil {
		util.ReportError(w, r, err, "Search for digests failed.")
	}

	digests := []string{}
	for _, digest := range digestsInfo {
		digests = append(digests, digest.Digest)
	}

	digestIndex := map[string]int{}
	for i, d := range digests {
		digestIndex[d] = i
	}

	d3 := D3{
		Nodes:     []Node{},
		Links:     []Link{},
		Paramsets: map[string]map[string][]string{},
		Paramset:  map[string][]string{},
	}
	for i, d := range digestsInfo {
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
		d3.Paramsets[d.Digest] = paramsetSum.Get(d.Test, d.Digest, false)
		for _, p := range d3.Paramsets[d.Digest] {
			sort.Strings(p)
		}
		d3.Paramset = util.AddParamSetToParamSet(d3.Paramset, d3.Paramsets[d.Digest])
	}

	for _, p := range d3.Paramset {
		sort.Strings(p)
	}

	sendJsonResponse(w, d3)
}

// sendJsonResponse serializes resp to JSON. If an error occurs
// a text based error code is send to the client.
func sendJsonResponse(w http.ResponseWriter, resp interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		glog.Errorf("Failed to write or encode result: %s", err)
	}
}

// makeResourceHandler creates a static file handler that sets a caching policy.
func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", string(300))
		fileServer.ServeHTTP(w, r)
	}
}

// Init figures out where the resources are and then loads the templates.
func Init() {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	loadTemplates()
}
