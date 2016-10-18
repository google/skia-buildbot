package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/digesttools"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
)

func polyDetailsHandler(w http.ResponseWriter, r *http.Request) {
	var tile *tiling.Tile = nil
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse form values")
		return
	}
	top := r.Form.Get("top")
	left := r.Form.Get("left")
	if top == "" || left == "" {
		httputils.ReportError(w, r, fmt.Errorf("Missing the top or left query parameter: %s %s", top, left), "No digests specified.")
		return
	}
	test := r.Form.Get("test")
	if test == "" {
		httputils.ReportError(w, r, fmt.Errorf("Missing the test query parameter."), "No test name specified.")
		return
	}

	exp, err := storages.ExpectationsStore.Get()
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to load expectations.")
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

	// var paramsetSum *summary.Summaries = nil

	// topParamSet := paramsetSum.Get()
	// leftParamSet := paramsetSum.Get()

	var tallies *tally.Tallies = nil
	traceNames := []string{}
	tally := tallies.ByTrace()
	for id, tr := range tile.Traces {
		if tr.Params()[types.PRIMARY_KEY_FIELD] == test {
			traceNames = append(traceNames, id)
		}
	}

	// keys := util.NewStringSet(util.KeysOfParamSet(topParamSet), util.KeysOfParamSet(leftParamSet)).Keys()
	// sort.Strings(keys)
	// for _, k := range keys {
	// 	ret.Params = append(ret.Params, &PerParamCompare{
	// 		Name: k,
	// 		Top:  safeGet(topParamSet, k),
	// 		Left: safeGet(leftParamSet, k),
	// 	})
	// }

	// Now build the trace data.
	if graphs {
		ret.Traces, ret.OtherDigests = buildTraceData(top, traceNames, tile, tally, exp)
		ret.Commits = tile.Commits
		var blamer *blame.Blamer
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
