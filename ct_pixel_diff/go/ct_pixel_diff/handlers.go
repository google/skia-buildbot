package main

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"go.skia.org/infra/ct_pixel_diff/go/resultstore"
	"go.skia.org/infra/go/sklog"
)

// jsonRunsHandler returns the current list of CT Pixel Diff jobs.
func jsonRunsHandler(w http.ResponseWriter, r *http.Request) {
	runIDs, err := resultStore.GetRunIDs(resultstore.BeginningOfTime, time.Now())
	if err != nil {
		sklog.Errorf("Failed to retrieve runIDs: %s", err)
	}
	sendJsonResponse(w, map[string][]string{"runs": runIDs})
}

// jsonLoadHandler fills the cache with the list of diff results for a given run
// by querying the ResultStore.
func jsonLoadHandler(w http.ResponseWriter, r *http.Request) {
	runID := r.FormValue("runID")
	if resultsMap[runID] == nil {
		results, err := resultStore.GetAll(runID)
		if err != nil {
			sklog.Errorf("Failed to get results for run %s: %s", runID, err)
		}
		resultsMap[runID] = results
	}
}

// jsonRenderHandler parses a start index, end index, and job from the query and
// uses them to return results in the specified range for the specified run.
func jsonRenderHandler(w http.ResponseWriter, r *http.Request) {
	runID := r.FormValue("runID")
	results := resultsMap[runID]

	index, err := strconv.Atoi(r.FormValue("index"))
	if err != nil {
		sklog.Errorf("Failed to parse index: %s", err)
	}

	ret := []*resultstore.ResultRec{}
	for len(ret) < CHUNK_SIZE && index < len(results) {
		if results[index].DiffMetrics != nil {
			ret = append(ret, results[index])
		}
		index++
	}
	sendJsonResponse(w, map[string]interface{}{"results": ret, "index": index})
}

// jsonSortHandler sorts the list of diff results using the specified sort value
// and runID.
func jsonSortHandler(w http.ResponseWriter, r *http.Request) {
	runID := r.FormValue("runID")
	results := resultsMap[runID]

	sortVal, err := strconv.Atoi(r.FormValue("sortVal"))
	if err != nil {
		sklog.Errorf("Failed to parse sort value: %s", err)
	}

	// Sort based on the specified sort parameter. If two ResultRecs do not have
	// diff metrics or have the same value for the parameter, sort by URL.
	switch sortVal {
	case NUM_DIFF_PIXELS_DSC:
		sort.Slice(results, func(i, j int) bool {
			if !hasDiffMetrics(results, i, j) ||
				results[i].DiffMetrics.NumDiffPixels == results[j].DiffMetrics.NumDiffPixels {
				return results[i].URL < results[j].URL
			}
			return results[i].DiffMetrics.NumDiffPixels > results[j].DiffMetrics.NumDiffPixels
		})
	case NUM_DIFF_PIXELS_ASC:
		sort.Slice(results, func(i, j int) bool {
			if !hasDiffMetrics(results, i, j) ||
				results[i].DiffMetrics.NumDiffPixels == results[j].DiffMetrics.NumDiffPixels {
				return results[i].URL < results[j].URL
			}
			return results[i].DiffMetrics.NumDiffPixels < results[j].DiffMetrics.NumDiffPixels
		})
	case PER_DIFF_PIXELS_DSC:
		sort.Slice(results, func(i, j int) bool {
			if !hasDiffMetrics(results, i, j) ||
				results[i].DiffMetrics.PixelDiffPercent == results[j].DiffMetrics.PixelDiffPercent {
				return results[i].URL < results[j].URL
			}
			return results[i].DiffMetrics.PixelDiffPercent > results[j].DiffMetrics.PixelDiffPercent
		})
	case PER_DIFF_PIXELS_ASC:
		sort.Slice(results, func(i, j int) bool {
			if !hasDiffMetrics(results, i, j) ||
				results[i].DiffMetrics.PixelDiffPercent == results[j].DiffMetrics.PixelDiffPercent {
				return results[i].URL < results[j].URL
			}
			return results[i].DiffMetrics.PixelDiffPercent < results[j].DiffMetrics.PixelDiffPercent
		})
	case MAX_RED_DIFF_DSC:
		sort.Slice(results, func(i, j int) bool {
			if !hasDiffMetrics(results, i, j) ||
				results[i].DiffMetrics.MaxRGBADiffs[0] == results[j].DiffMetrics.MaxRGBADiffs[0] {
				return results[i].URL < results[j].URL
			}
			return results[i].DiffMetrics.MaxRGBADiffs[0] > results[j].DiffMetrics.MaxRGBADiffs[0]
		})
	case MAX_RED_DIFF_ASC:
		sort.Slice(results, func(i, j int) bool {
			if !hasDiffMetrics(results, i, j) ||
				results[i].DiffMetrics.MaxRGBADiffs[0] == results[j].DiffMetrics.MaxRGBADiffs[0] {
				return results[i].URL < results[j].URL
			}
			return results[i].DiffMetrics.MaxRGBADiffs[0] < results[j].DiffMetrics.MaxRGBADiffs[0]
		})
	case MAX_GREEN_DIFF_DSC:
		sort.Slice(results, func(i, j int) bool {
			if !hasDiffMetrics(results, i, j) ||
				results[i].DiffMetrics.MaxRGBADiffs[1] == results[j].DiffMetrics.MaxRGBADiffs[1] {
				return results[i].URL < results[j].URL
			}
			return results[i].DiffMetrics.MaxRGBADiffs[1] > results[j].DiffMetrics.MaxRGBADiffs[1]
		})
	case MAX_GREEN_DIFF_ASC:
		sort.Slice(results, func(i, j int) bool {
			if !hasDiffMetrics(results, i, j) ||
				results[i].DiffMetrics.MaxRGBADiffs[1] == results[j].DiffMetrics.MaxRGBADiffs[1] {
				return results[i].URL < results[j].URL
			}
			return results[i].DiffMetrics.MaxRGBADiffs[1] < results[j].DiffMetrics.MaxRGBADiffs[1]
		})
	case MAX_BLUE_DIFF_DSC:
		sort.Slice(results, func(i, j int) bool {
			if !hasDiffMetrics(results, i, j) ||
				results[i].DiffMetrics.MaxRGBADiffs[2] == results[j].DiffMetrics.MaxRGBADiffs[2] {
				return results[i].URL < results[j].URL
			}
			return results[i].DiffMetrics.MaxRGBADiffs[2] > results[j].DiffMetrics.MaxRGBADiffs[2]
		})
	case MAX_BLUE_DIFF_ASC:
		sort.Slice(results, func(i, j int) bool {
			if !hasDiffMetrics(results, i, j) ||
				results[i].DiffMetrics.MaxRGBADiffs[2] == results[j].DiffMetrics.MaxRGBADiffs[2] {
				return results[i].URL < results[j].URL
			}
			return results[i].DiffMetrics.MaxRGBADiffs[2] < results[j].DiffMetrics.MaxRGBADiffs[2]
		})
	case SITE_RANK_DSC:
		sort.Slice(results, func(i, j int) bool {
			return results[i].Rank < results[j].Rank
		})
	case SITE_RANK_ASC:
		sort.Slice(results, func(i, j int) bool {
			return results[i].Rank > results[j].Rank
		})
	}
}

// makeResourceHandler creates a static file handler that sets a caching policy.
func makeResourceHandler(resourceDir string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(resourceDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}
