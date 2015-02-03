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
	Test string `json:"test"`
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
	DiffImgUrl       string  `json:"diffImgUrl"`
	PosDigest        string  `json:"posDigest"`
}

// PolyTestGUI serialized as JSON is the response body from polyTestHandler.
type PolyTestGUI struct {
	Top  []*PolyTestImgInfo    `json:"top"`
	Left []*PolyTestImgInfo    `json:"left"`
	Grid [][]*PolyTestDiffInfo `json:"grid"`
}

// polyTestHandler returns a JSON description for the given test.
//
// Takes an JSON encoded POST body of the following form:
//
//   {
//      test: The name of the test.
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
//   ]
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

	// For now sorting is done by string compare of the digest, just so we
	// have a stable display.
	sort.Strings(untriaged)
	sort.Strings(positives)

	if len(untriaged) == 0 {
		untriaged = positives
	}
	if len(positives) == 0 {
		positives = untriaged
	}

	// Limit to 5x5 grids for now.
	if len(untriaged) > 5 {
		untriaged = untriaged[:5]
	}
	if len(positives) > 5 {
		positives = positives[:5]
	}
	glog.Infof("%#v", untriaged)
	glog.Infof("%#v", positives)

	// Fill in our GUI response struct.
	top := []*PolyTestImgInfo{}
	thumbs := diffStore.ThumbAbsPath(untriaged)
	for _, u := range untriaged {
		top = append(top, &PolyTestImgInfo{
			Thumb:  pathToURLConverter(thumbs[u]),
			Digest: u,
		})
	}

	left := []*PolyTestImgInfo{}
	thumbs = diffStore.ThumbAbsPath(positives)
	for _, p := range positives {
		left = append(left, &PolyTestImgInfo{
			Thumb:  pathToURLConverter(thumbs[p]),
			Digest: p,
		})
	}

	grid := [][]*PolyTestDiffInfo{}
	for _, pos := range positives {
		row := []*PolyTestDiffInfo{}
		diffs, err := diffStore.Get(pos, untriaged)
		if err != nil {
			glog.Errorf("Failed to do diffs: %s", err)
			continue
		}
		glog.Infof("%#v", diffs)
		for _, u := range untriaged {
			d := diffs[u]
			row = append(row, &PolyTestDiffInfo{
				Thumb:            pathToURLConverter(d.ThumbnailPixelDiffFilePath),
				TopDigest:        u,
				LeftDigest:       pos,
				NumDiffPixels:    d.NumDiffPixels,
				PixelDiffPercent: d.PixelDiffPercent,
				MaxRGBADiffs:     d.MaxRGBADiffs,
				DiffImgUrl:       pathToURLConverter(d.PixelDiffFilePath),
			})

		}
		grid = append(grid, row)
	}

	p := PolyTestGUI{
		Top:  top,
		Left: left,
		Grid: grid,
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(p); err != nil {
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
