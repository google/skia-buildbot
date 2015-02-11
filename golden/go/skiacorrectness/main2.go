package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"skia.googlesource.com/buildbot.git/go/login"
	"skia.googlesource.com/buildbot.git/go/timer"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/golden/go/summary"
	"skia.googlesource.com/buildbot.git/golden/go/types"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// ignoresTemplate is the page for setting up ignore filters.
	ignoresTemplate *template.Template = nil

	// compareTemplate is the page for setting up ignore filters.
	compareTemplate *template.Template = nil
)

// polyMainHandler is the main page for the Polymer based frontend.
func polyMainHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Poly Main Handler: %q\n", r.URL.Path)
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	if err := indexTemplate.Execute(w, struct{}{}); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

func loadTemplates() {
	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
	ignoresTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/ignores.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
	compareTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/compare.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

type SummarySlice []*summary.Summary

func (p SummarySlice) Len() int           { return len(p) }
func (p SummarySlice) Less(i, j int) bool { return p[i].Name < p[j].Name }
func (p SummarySlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// polyListTestsHandler returns a JSON list with high level information about
// each test.
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
	sumMap := summaries.Get()
	sumSlice := make([]*summary.Summary, 0, len(sumMap))
	for _, s := range sumMap {
		sumSlice = append(sumSlice, s)
	}
	sort.Sort(SummarySlice(sumSlice))
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(sumSlice); err != nil {
		util.ReportError(w, r, err, "Failed to encode result")
	}
}

// polyIgnoresJSONHandler returns the current ignore rules in JSON format.
func polyIgnoresJSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ignores, err := analyzer.ListIgnoreRules()
	if err != nil {
		util.ReportError(w, r, err, "Failed to retrieve ignored traces.")
	}

	// TODO(stephana): Wrap in response envelope if it makes sense !
	enc := json.NewEncoder(w)
	if err := enc.Encode(ignores); err != nil {
		util.ReportError(w, r, err, "Failed to encode result")
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

	if err := analyzer.DeleteIgnoreRule(int(id), user); err != nil {
		util.ReportError(w, r, err, "Unable to delete ignore rule.")
	} else {
		// If delete worked just list the current ignores and return them.
		polyIgnoresJSONHandler(w, r)
	}
}

type IgnoresAddRequest struct {
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
	req := &IgnoresAddRequest{}
	if err := parseJson(r, req); err != nil {
		util.ReportError(w, r, err, "Failed to parse submitted data.")
		return
	}
	parsed := durationRe.FindStringSubmatch(req.Duration)
	if len(parsed) != 3 {
		util.ReportError(w, r, fmt.Errorf("Rejected duration: %s", req.Duration), "Failed to parse duration")
		return
	}
	// TODO break out the following into its own func, add tests.
	n, err := strconv.ParseInt(parsed[1], 10, 32)
	if err != nil {
		util.ReportError(w, r, err, "Failed to parse duration")
		return
	}
	d := time.Second
	switch parsed[2][0] {
	case 's':
		d = time.Duration(n) * time.Second
	case 'm':
		d = time.Duration(n) * time.Minute
	case 'h':
		d = time.Duration(n) * time.Hour
	case 'd':
		d = time.Duration(n) * 24 * time.Hour
	case 'w':
		d = time.Duration(n) * 7 * 24 * time.Hour
	}
	ignoreRule := types.NewIgnoreRule(user, time.Now().Add(d), req.Filter, req.Note)
	if err != nil {
		util.ReportError(w, r, err, "Failed to create ignore rule.")
		return
	}

	if err := analyzer.AddIgnoreRule(ignoreRule); err != nil {
		util.ReportError(w, r, err, "Failed to create ignore rule.")
		return
	}

	polyIgnoresJSONHandler(w, r)
}

// polyIgnoresHandler is for setting up ignores rules.
func polyIgnoresHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Poly Ignores Handler: %q\n", r.URL.Path)
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	if err := ignoresTemplate.Execute(w, struct{}{}); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

// polyCompareHandler is for serving the main compare page.
func polyCompareHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Poly Compare Handler: %q\n", r.URL.Path)
	w.Header().Set("Content-Type", "text/html")
	if *local {
		loadTemplates()
	}
	if err := compareTemplate.Execute(w, struct{}{}); err != nil {
		glog.Errorln("Failed to expand template:", err)
	}
}

// PolyTestRequest is the POST'd request body handled by polyTestHandler.
type PolyTestRequest struct {
	Test       string `json:"test"`
	TopFilter  string `json:"topFilter"`
	LeftFilter string `json:"LeftFilter"`
}

// PolyTestImgInfo info about a single source digest. Used in PolyTestGUI.
type PolyTestImgInfo struct {
	Thumb  string `json:"thumb"`
	Digest string `json:"digest"`
}

// PolyTestImgInfo info about a single diff between two source digests. Used in
// PolyTestGUI.
type PolyTestDiffInfo struct {
	Thumb            string  `json:"thumb"`
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
	Top     []*PolyTestImgInfo    `json:"top"`
	Left    []*PolyTestImgInfo    `json:"left"`
	Grid    [][]*PolyTestDiffInfo `json:"grid"`
	Message string                `json:"message"`
}

// polyTestHandler returns a JSON description for the given test.
//
// Takes an JSON encoded POST body of the following form:
//
//   {
//      test: The name of the test.
//      topFilter=["positive", "negative", "untriaged"]
//      leftFilter=["positive", "negative", "untriaged"]
//   }
//
// TODO
//   * add querying
//   * return all the data
//   * add sorting based on query params
//   * query dialog
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

	if req.TopFilter == "" {
		req.TopFilter = "untriaged"
	}
	if req.LeftFilter == "" {
		req.LeftFilter = "positive"
	}

	// Get all the digests. This should get generalized in a later CL to accept
	// queries, sorting, and to allow choosing which types of digests go on the
	// top and the left of the grid.
	t := timer.New("tallies.ByQuery")
	digests, _ := tallies.ByQuery(url.Values{"name": []string{req.Test}})
	glog.Infof("Num Digests: %d", len(digests))
	t.Stop()

	// Now filter digests by their expectations status here.
	t = timer.New("apply expectations")
	exp, err := expStore.Get(false)
	if err != nil {
		util.ReportError(w, r, err, "Failed to load expectations.")
		return
	}
	e := exp.Tests[req.Test]
	untriaged := []string{}
	positives := []string{}
	negatives := []string{}
	for digest, _ := range digests {
		switch e[digest] {
		case types.UNTRIAGED:
			untriaged = append(untriaged, digest)
		case types.POSITIVE:
			positives = append(positives, digest)
		case types.NEGATIVE:
			negatives = append(negatives, digest)
		}
	}
	t.Stop()
	digestsByStatus := map[string][]string{
		"untriaged": untriaged,
		"positive":  positives,
		"negative":  negatives,
	}

	topDigests := digestsByStatus[req.TopFilter]
	leftDigests := digestsByStatus[req.LeftFilter]

	// For now sorting is done by string compare of the digest, just so we
	// have a stable display.
	sort.Strings(topDigests)
	sort.Strings(leftDigests)

	// Limit to 5x5 grids for now.
	if len(topDigests) > 5 {
		topDigests = topDigests[:5]
	}
	if len(leftDigests) > 5 {
		leftDigests = leftDigests[:5]
	}

	allDigests := util.UnionStrings(topDigests, leftDigests)
	thumbs := diffStore.ThumbAbsPath(allDigests)
	full := diffStore.AbsPath(allDigests)

	// Fill in our GUI response struct.
	top := []*PolyTestImgInfo{}
	for _, t := range topDigests {
		top = append(top, &PolyTestImgInfo{
			Thumb:  pathToURLConverter(thumbs[t]),
			Digest: t,
		})
	}

	left := []*PolyTestImgInfo{}
	for _, l := range leftDigests {
		left = append(left, &PolyTestImgInfo{
			Thumb:  pathToURLConverter(thumbs[l]),
			Digest: l,
		})
	}

	grid := [][]*PolyTestDiffInfo{}
	for _, l := range leftDigests {
		row := []*PolyTestDiffInfo{}
		diffs, err := diffStore.Get(l, topDigests)
		if err != nil {
			glog.Errorf("Failed to do diffs: %s", err)
			continue
		}
		for _, t := range topDigests {
			d := diffs[t]
			row = append(row, &PolyTestDiffInfo{
				Thumb:            pathToURLConverter(d.ThumbnailPixelDiffFilePath),
				TopDigest:        t,
				LeftDigest:       l,
				NumDiffPixels:    d.NumDiffPixels,
				PixelDiffPercent: d.PixelDiffPercent,
				MaxRGBADiffs:     d.MaxRGBADiffs,
				DiffImg:          pathToURLConverter(d.PixelDiffFilePath),
				TopImg:           pathToURLConverter(full[t]),
				LeftImg:          pathToURLConverter(full[l]),
			})

		}
		grid = append(grid, row)
	}

	p := PolyTestGUI{
		Top:  top,
		Left: left,
		Grid: grid,
	}
	if len(p.Top) == 0 || len(p.Left) == 0 {
		p.Message = "Failed to find images that match those filters."
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(p); err != nil {
		util.ReportError(w, r, err, "Failed to encode result")
	}
}

// PolyTriageRequest is the form of the JSON posted to polyTriageHandler.
type PolyTriageRequest struct {
	Test   string `json:"test"`
	Digest string `json:"digest"`
	Status string `json:"status"`
}

// polyTriageHandler handles a request to change the triage status of a single
// digest of one test.
//
// It accepts a POST'd JSON serialization of PolyTriageRequest and updates
// the expectations.
func polyTriageHandler(w http.ResponseWriter, r *http.Request) {
	req := &PolyTriageRequest{}
	if err := parseJson(r, req); err != nil {
		util.ReportError(w, r, err, "Failed to parse JSON request.")
		return
	}
	user := login.LoggedInAs(r)
	if user == "" {
		util.ReportError(w, r, fmt.Errorf("Not logged in."), "You must be logged in to triage.")
		return
	}
	e, err := expStore.Get(true)
	if err != nil {
		util.ReportError(w, r, err, "Failed to read the current expectations.")
		return
	}

	tc := map[string]types.TestClassification{
		req.Test: map[string]types.Label{
			req.Digest: types.LabelFromString(req.Status),
		},
	}
	// If the analyzer is running then use that to update the expectations.
	if *startAnalyzer {
		_, err := analyzer.SetDigestLabels(tc, user)
		if err != nil {
			util.ReportError(w, r, err, "Failed to set the expectations.")
			return
		}
	} else {
		// Otherwise update the expectations directly.
		e.AddDigests(tc)
		if err := expStore.Put(e, user); err != nil {
			util.ReportError(w, r, err, "Failed to store the updated expectations.")
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(map[string]string{}); err != nil {
		util.ReportError(w, r, err, "Failed to encode result")
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

// PolyDetailsGUI is used in the JSON returned from polyDetailsHandler. It
// represents the known information about a single digest for a given test.
type PolyDetailsGUI struct {
	TopStatus  string             `json:"topStatus"`
	LeftStatus string             `json:"leftStatus"`
	Params     []*PerParamCompare `json:"params"`
}

// polyDetailsHandler handles requests about individual digests in a test.
//
// It expects a request with the following query parameters:
//
//   test - The name of the test.
//   top  - A digest in the test.
//   left - A digest in the test.
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
//     ]
//   }
func polyDetailsHandler(w http.ResponseWriter, r *http.Request) {
	tile, err := tileStore.Get(0, -1)
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
	exp, err := expStore.Get(false)
	if err != nil {
		util.ReportError(w, r, err, "Failed to load expectations.")
		return
	}

	ret := PolyDetailsGUI{
		TopStatus:  exp.Classification(test, top).String(),
		LeftStatus: exp.Classification(test, left).String(),
		Params:     []*PerParamCompare{},
	}

	topParamSet := map[string][]string{}
	leftParamSet := map[string][]string{}

	// Now build out the ParamSet for each digest.
	tally := tallies.ByTrace()
	for id, tr := range tile.Traces {
		traceTally, ok := tally[id]
		if !ok {
			continue
		}
		if tr.Params()[types.PRIMARY_KEY_FIELD] == test {
			if _, ok := (*traceTally)[top]; ok {
				topParamSet = util.AddParamsToParamSet(topParamSet, tr.Params())
			}
		}
		if tr.Params()[types.PRIMARY_KEY_FIELD] == test {
			if _, ok := (*traceTally)[left]; ok {
				leftParamSet = util.AddParamsToParamSet(leftParamSet, tr.Params())
			}
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

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(ret); err != nil {
		util.ReportError(w, r, err, "Failed to encode result")
	}
}

func polyParamsHandler(w http.ResponseWriter, r *http.Request) {
	tile, err := tileStore.Get(0, -1)
	if err != nil {
		util.ReportError(w, r, err, "Failed to load tile")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(tile.ParamSet); err != nil {
		util.ReportError(w, r, err, "Failed to encode result")
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
