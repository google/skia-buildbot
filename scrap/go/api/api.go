// Package api implements the REST API for the scrap exchange service.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/scrap/go/scrap"
)

// Api supplies the handlers for the scrap exchange REST API.
type Api struct {
	scrapExchange scrap.ScrapExchange
}

// New returns a new Api instance.
func New(client *gcsclient.StorageClient) *Api {
	return &Api{
		scrapExchange: scrap.New(client),
	}
}

// The URL variable names used in the mux Path handlers. See ApplyHandlers.
const (
	hashOrNameVar = "hashOrName"
	nameVar       = "name"
	typeVar       = "type"
	langVar       = "lang"
)

var (
	errUnknownType = errors.New("Uknown Type.")
	errUnknownLang = errors.New("Uknown Lang.")
)

// ApplyHandlers hooks up the scrap exchange REST API to the given router.
//
// If exposeProtectedEndpoints is true then the mutating endpoints, such as the
// one for creating scraps, are added, otherwise they are skipped.
func (a *Api) ApplyHandlers(r *mux.Router, exposeProtectedEndpoints bool) {
	if exposeProtectedEndpoints {
		r.HandleFunc("/_/scraps/", a.scrapCreateHandler).Methods("POST")
	}

	scraps := r.Path("/_/scraps/{type:[a-z]+}/{hashOrName:[@0-9a-zA-Z-_]+}").Subrouter()
	scraps.Methods("GET").HandlerFunc(a.scrapGetHandler)
	if exposeProtectedEndpoints {
		scraps.Methods("DELETE").HandlerFunc(a.scrapDeleteHandler)
	}

	r.HandleFunc("/_/raw/{type:[a-z]+}/{hashOrName:[@0-9a-zA-Z-_]+}", a.rawGetHandler).Methods("GET")
	r.HandleFunc("/_/tmpl/{type:[a-z]+}/{hashOrName:[@0-9a-zA-Z-_]+}/{lang:[a-z]+}", a.templateGetHandler).Methods("GET")

	names := r.Path("/_/names/{type:[a-z]+}/{name:[0-9a-zA-Z-_]+}").Subrouter()
	names.Methods("GET").HandlerFunc(a.nameGetHandler)
	if exposeProtectedEndpoints {
		names.Methods("PUT").HandlerFunc(a.namePutHandler)
	}
	r.HandleFunc("/_/names/{type:[a-z]+}/", a.namesListHandler).Methods("GET")
}

func writeJSON(w http.ResponseWriter, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(body); err != nil {
		httputils.ReportError(w, err, "Failed to encode JSON response.", http.StatusInternalServerError)
		return

	}
	if _, err := w.Write(b.Bytes()); err != nil {
		sklog.Errorf("Failed to write JSON response.")
	}
}

func (a *Api) scrapCreateHandler(w http.ResponseWriter, r *http.Request) {
	var body scrap.ScrapBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httputils.ReportError(w, err, "Failed to decode ScrapBody", http.StatusBadRequest)
		return
	}

	id, err := a.scrapExchange.CreateScrap(r.Context(), body)
	if err != nil {
		httputils.ReportError(w, err, "Failed to store scrap", http.StatusInternalServerError)
		return
	}

	writeJSON(w, id)
}

// getType returns the Type specified in the URL, or false if the type was
// invalid or not present.
//
// This function will also report the error on the http.ResponseWriter.
func (a *Api) getType(w http.ResponseWriter, r *http.Request) (scrap.Type, bool) {
	vars := mux.Vars(r)
	t := scrap.ToType(vars[typeVar])
	if t == scrap.UnknownType {
		httputils.ReportError(w, errUnknownType, "Unknown type.", http.StatusBadRequest)
		return scrap.UnknownType, false
	}
	return t, true
}

func (a *Api) scrapGetHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hashOrName := vars[hashOrNameVar]
	t, ok := a.getType(w, r)
	if !ok {
		return
	}

	scrapBody, err := a.scrapExchange.LoadScrap(r.Context(), t, hashOrName)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load scrap.", http.StatusBadRequest)
		return
	}

	writeJSON(w, scrapBody)
}

func (a *Api) scrapDeleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hashOrName := vars[hashOrNameVar]
	t, ok := a.getType(w, r)
	if !ok {
		return
	}

	err := a.scrapExchange.DeleteScrap(r.Context(), t, hashOrName)
	if err != nil {
		httputils.ReportError(w, err, "Failed to delete scrap.", http.StatusBadRequest)
		return
	}
}

func (a *Api) rawGetHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hashOrName := vars[hashOrNameVar]
	t, ok := a.getType(w, r)
	if !ok {
		return
	}

	scrapBody, err := a.scrapExchange.LoadScrap(r.Context(), t, hashOrName)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load scrap.", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", scrap.MimeTypes[t])
	if _, err := w.Write([]byte(scrapBody.Body)); err != nil {
		sklog.Errorf("Failed to write result: %s", err)
	}
}

func (a *Api) templateGetHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hashOrName := vars[hashOrNameVar]
	t, ok := a.getType(w, r)
	if !ok {
		return
	}

	l := scrap.ToLang(vars[langVar])
	if l == scrap.UnknownLang {
		httputils.ReportError(w, errUnknownLang, "Unknown language.", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	if err := a.scrapExchange.Expand(r.Context(), t, hashOrName, l, w); err != nil {
		httputils.ReportError(w, err, "Failed to expand scrap.", http.StatusBadRequest)
		return
	}
}

func (a *Api) namePutHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars[nameVar]
	t, ok := a.getType(w, r)
	if !ok {
		return
	}

	var nameBody scrap.Name
	if err := json.NewDecoder(r.Body).Decode(&nameBody); err != nil {
		httputils.ReportError(w, err, "Failed to decode Name", http.StatusBadRequest)
		return
	}

	if err := a.scrapExchange.PutName(r.Context(), t, name, nameBody); err != nil {
		httputils.ReportError(w, err, "Failed to write name.", http.StatusInternalServerError)
		return
	}
}

func (a *Api) nameGetHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars[nameVar]
	t, ok := a.getType(w, r)
	if !ok {
		return
	}

	nameBody, err := a.scrapExchange.GetName(r.Context(), t, name)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve Name", http.StatusInternalServerError)
	}

	writeJSON(w, nameBody)
}

func (a *Api) namesListHandler(w http.ResponseWriter, r *http.Request) {
	t, ok := a.getType(w, r)
	if !ok {
		return
	}

	names, err := a.scrapExchange.ListNames(r.Context(), t)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load names.", http.StatusInternalServerError)
		return
	}

	writeJSON(w, names)
}
