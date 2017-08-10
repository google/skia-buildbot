package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/ct_pixel_diff/go/resultstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/golden/go/diffstore"
)

// jsonRunsHandler returns the current list of CT Pixel Diff jobs as a
// serialized list of strings.
func jsonRunsHandler(w http.ResponseWriter, r *http.Request) {
	runIDs, err := resultStore.GetRunIDs(resultstore.BeginningOfTime, time.Now())
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to retrieve runIDs")
		return
	}
	sendJsonResponse(w, map[string][]string{"runs": runIDs})
}

// jsonDeleteHandler deletes the data for the specified runID from the server.
func jsonDeleteHandler(w http.ResponseWriter, r *http.Request) {
	runID := r.FormValue("runID")

	// Extract the username from the runID and the cookie to make sure they match.
	runUser := strings.Split(runID, "-")[0]
	loggedInUser := strings.Split(login.LoggedInAs(r), "@")[0]
	if !login.IsAdmin(r) && runUser != loggedInUser {
		httputils.ReportError(w, r, nil, "You must be logged on as an admin to delete other users' runs.")
		return
	}

	// Remove ResultStore data.
	err := resultStore.RemoveRun(runID)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to remove run %s from server", runID))
		return
	}

	// TODO(lchoi): Create a storage container class that has an aggregate remove
	// function and call that here to simplify the handler logic. PurgeDigests in
	// MemDiffStore must first be refactored to also remove diff images.

	// Remove screenshots and diff images from the DiffStore.
	imagePath := filepath.Join(*imageDir, diffstore.DEFAULT_IMG_DIR_NAME, runID)
	diffPath := filepath.Join(*imageDir, diffstore.DEFAULT_DIFFIMG_DIR_NAME, runID)
	err = os.RemoveAll(imagePath)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to remove screenshots for run %s from DiffStore", runID))
		return
	}
	err = os.RemoveAll(diffPath)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to remove diff images for run %s from DiffStore", runID))
		return
	}
}

// jsonRenderHandler parses a start index, end index, and runID from the query
// and uses them to return results in the specified range for the specified run
// from the ResultStore cache.
func jsonRenderHandler(w http.ResponseWriter, r *http.Request) {
	runID := r.FormValue("runID")

	startIdx, err := strconv.Atoi(r.FormValue("startIdx"))
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to parse start index")
		return
	}

	minPercent, err := strconv.ParseFloat(r.FormValue("minPercent"), 64)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to parse minimum percent")
		return
	}

	maxPercent, err := strconv.ParseFloat(r.FormValue("maxPercent"), 64)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to parse maximum percent")
		return
	}

	if minPercent > maxPercent || minPercent < 0 || maxPercent > 100 {
		httputils.ReportError(w, r, err, "Invalid bounds")
		return
	}

	// If the runID does not exist in the cache, this will return an error.
	results, nextIdx, err := resultStore.GetFiltered(runID, startIdx, float32(minPercent), float32(maxPercent))
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to get cached results for run %s", runID))
		return
	}
	if len(results) == 0 {
		httputils.ReportError(w, r, err, fmt.Sprintf("No more results for run %s", runID))
		return
	}
	sendJsonResponse(w, map[string]interface{}{"results": results, "nextIdx": nextIdx})
}

// jsonSortHandler sorts the ResultStore's cached list of diff results using the
// specified sort field, sort order, and runID.
func jsonSortHandler(w http.ResponseWriter, r *http.Request) {
	runID := r.FormValue("runID")
	sortField := r.FormValue("sortField")
	sortOrder := r.FormValue("sortOrder")

	// If the runID does not exist in the cache, this will return an error.
	err := resultStore.SortRun(runID, sortField, sortOrder)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to sort cached results for run %s", runID))
		return
	}
}

// jsonURLsHandler returns all the urls in the ResultStore's cache for the
// specified runID.
func jsonURLsHandler(w http.ResponseWriter, r *http.Request) {
	runID := r.FormValue("runID")
	urls, err := resultStore.GetURLs(runID)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to retrieve URLs for run %s", runID))
	}
	sendJsonResponse(w, map[string][]map[string]string{"urls": urls})
}

// jsonSearchHandler parses a runID and url from the query and uses them to
// return the correct ResultRec from the ResultStore.
func jsonSearchHandler(w http.ResponseWriter, r *http.Request) {
	runID := r.FormValue("runID")
	url := r.FormValue("url")
	result, err := resultStore.Get(runID, url)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to retrieve search result for run %s, url %s", runID, url))
	}
	sendJsonResponse(w, map[string]*resultstore.ResultRec{"result": result})
}

// jsonStatsHandler parses a runID from the query and uses it to return various
// statistics about the run's cached results.
func jsonStatsHandler(w http.ResponseWriter, r *http.Request) {
	runID := r.FormValue("runID")
	stats, histogram, err := resultStore.GetStats(runID)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to retrieve stats for run %s", runID))
	}
	sendJsonResponse(w, map[string]map[string]int{"stats": stats, "histogram": histogram})
}

// makeResourceHandler creates a static file handler that sets a caching policy.
func makeResourceHandler(resourceDir string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(resourceDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}
