package main

import (
	"encoding/json"
	"net/http"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/status"
	"go.skia.org/infra/golden/go/storage"
)

// TODO(stephana): Simplify
// the ResponseEnvelope and use it solely to wrap JSON arrays.
// Remove sendResponse and sendErrorResponse in favor of sendJsonResponse
// and httputils.ReportError.

// ResponseEnvelope wraps all responses. Some fields might be empty depending
// on context or whether there was an error or not.
type ResponseEnvelope struct {
	Data       *interface{}                  `json:"data"`
	Err        *string                       `json:"err"`
	Status     int                           `json:"status"`
	Pagination *httputils.ResponsePagination `json:"pagination"`
}

var (
	// Module level variables that need to be accessible to handler.go.
	storages      *storage.Storage
	statusWatcher *status.StatusWatcher
	ixr           *indexer.Indexer
	issueTracker  issues.IssueTracker
)

// setJSONHeaders sets secure headers for JSON responses.
func setJSONHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Content-Type", "application/json")
	h.Set("X-Content-Type-Options", "nosniff")
}

// sendResponse wraps the data of a succesful response in a response envelope
// and sends it to the client.
func sendResponse(w http.ResponseWriter, data interface{}, status int, pagination *httputils.ResponsePagination) {
	resp := ResponseEnvelope{&data, nil, status, pagination}
	setJSONHeaders(w)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// sendJsonResponse serializes resp to JSON. If an error occurs
// a text based error code is send to the client.
func sendJsonResponse(w http.ResponseWriter, resp interface{}) {
	setJSONHeaders(w)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httputils.ReportError(w, nil, err, "Failed to encode JSON response.")
	}
}

// parseJson extracts the body from the request and parses it into the
// provided interface.
func parseJson(r *http.Request, v interface{}) error {
	// TODO (stephana): validate the JSON against a schema. Might not be necessary !
	defer util.Close(r.Body)
	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(v)
}
