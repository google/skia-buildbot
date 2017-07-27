package main

import (
	"encoding/json"
	"net/http"

	"go.skia.org/infra/ct_pixel_diff/go/resultstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/golden/go/diff"
)

const (
	NUM_DIFF_PIXELS_DSC = iota
	NUM_DIFF_PIXELS_ASC
	PER_DIFF_PIXELS_DSC
	PER_DIFF_PIXELS_ASC
	MAX_RED_DIFF_DSC
	MAX_RED_DIFF_ASC
	MAX_GREEN_DIFF_DSC
	MAX_GREEN_DIFF_ASC
	MAX_BLUE_DIFF_DSC
	MAX_BLUE_DIFF_ASC
	SITE_RANK_DSC
	SITE_RANK_ASC
)

var (
	// Module level variables that need to be accessible to handler.go.
	diffStore   diff.DiffStore
	resultStore resultstore.ResultStore
	resultsMap  map[string][]*resultstore.ResultRec
)

// setJSONHeaders sets secure headers for JSON responses.
func setJSONHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Content-Type", "application/json")
	h.Set("X-Content-Type-Options", "nosniff")
}

// sendJsonResponse serializes resp to JSON. If an error occurs
// a text based error code is send to the client.
func sendJsonResponse(w http.ResponseWriter, resp interface{}) {
	setJSONHeaders(w)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httputils.ReportError(w, nil, err, "Failed to encode JSON response.")
	}
}

// hasDiffMetrics verifies that the two ResultRec instances at indices i and j
// of results have diff metrics data. Used for sorting based on metrics values.
func hasDiffMetrics(results []*resultstore.ResultRec, i, j int) bool {
	return results[i].DiffMetrics != nil && results[j].DiffMetrics != nil
}
