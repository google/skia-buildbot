package web

import (
	"encoding/json"
	"net/http"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
)

// ResponseEnvelope wraps all responses. Some fields might be empty depending
// on context or whether there was an error or not.
type ResponseEnvelope struct {
	Data       *interface{}                  `json:"data"`
	Status     int                           `json:"status"`
	Pagination *httputils.ResponsePagination `json:"pagination"`
}

// setJSONHeaders sets secure headers for JSON responses.
func setJSONHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Content-Type", "application/json")
	h.Set("X-Content-Type-Options", "nosniff")
}

// sendResponseWithPagination wraps the data of a successful response in a response envelope
// and sends it to the client.
func sendResponseWithPagination(w http.ResponseWriter, data interface{}, pagination *httputils.ResponsePagination) {
	resp := ResponseEnvelope{
		Data:       &data,
		Status:     http.StatusOK,
		Pagination: pagination,
	}
	setJSONHeaders(w)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// sendJSONResponse serializes resp to JSON. If an error occurs
// a text based error code is send to the client.
func sendJSONResponse(w http.ResponseWriter, resp interface{}) {
	setJSONHeaders(w)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httputils.ReportError(w, nil, err, "Failed to encode JSON response.")
	}
}

// parseJSON extracts the body from the request and parses it into the
// provided interface.
func parseJSON(r *http.Request, v interface{}) error {
	defer util.Close(r.Body)
	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(v)
}
