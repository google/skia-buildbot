// Package api implements the REST API for the scrap exchange service.
//
// See also the Scrap Exchange Design Doc http://go/scrap-exchange.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/gcs/gcsclient"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/scrap/go/scrap"
)

// Endpoint names used for metrics.
const (
	scrapsCreateCallMetric = "scrap_exchange_api_scraps_create"
	scrapsGetCallMetric    = "scrap_exchange_api_scraps_get"
	scrapsDeleteCallMetric = "scrap_exchange_api_scraps_delete"
	rawGetCallMetric       = "scrap_exchange_api_raw_get"
	templateGetCallMetric  = "scrap_exchange_api_template_get"
	namesPutCallMetric     = "scrap_exchange_api_names_put"
	namesGetCallMetric     = "scrap_exchange_api_names_get"
	namesDeleteCallMetric  = "scrap_exchange_api_names_delete"
	namesListCallMetric    = "scrap_exchange_api_names_list"
)

// Api supplies the handlers for the scrap exchange REST API.
type Api struct {
	scrapExchange scrap.ScrapExchange
}

// New returns a new Api instance.
func New(client *gcsclient.StorageClient) (*Api, error) {
	se, err := scrap.New(client)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create ScrapExchange.")
	}
	return &Api{
		scrapExchange: se,
	}, nil
}

// The URL variable names used in the mux Path handlers. See ApplyHandlers.
const (
	hashOrNameVar = "hashOrName"
	nameVar       = "name"
	typeVar       = "type"
	langVar       = "lang"
)

var (
	errUnknownType = errors.New("Unknown Type.")
	errUnknownLang = errors.New("Unknown Lang.")
)

// Option controls which endpoints get added in AddHandlers.
type Option int

const (
	DoNotAddProtectedEndpoints Option = iota
	AddProtectedEndpoints
)

// AddHandlers hooks up the Scrap Exchange REST API to the given router.
//
// The value of 'option' controls if Protected endpoints, such as the one for
// creating scraps, are added to the router.
//
//
//    +----------------------------------------+--------+---------+---------------+-----------------------------------+------+
//    | URL                                    | Method | Request | Response      | Description                       | Prot |
//    +----------------------------------------+--------+---------+---------------+-----------------------------------+------+
//    | /_/scraps/                             | POST   | Scrap   | ScrapID       | Creates a new scrap.              | Y    |
//    +----------------------------------------+--------+---------+---------------+-----------------------------------+------+
//    | /_/scraps/{type}/({hash}|{name})       | GET    |         | ScrapBody     | Returns the scrap.                |      |
//    +----------------------------------------+--------+---------+---------------+-----------------------------------+------+
//    | /_/scraps/{type}/({hash}|{name})       | DELETE |         |               | Removes the scrap.                | Y    |
//    +----------------------------------------+--------+---------+---------------+-----------------------------------+------+
//    | /_/raw/{type}/({hash}|{name})          | GET    |         | text/plain    | Returns the raw scrap.            |      |
//    |                                        |        |         | image/svg+xml |                                   |      |
//    +----------------------------------------+--------+---------+---------------+-----------------------------------+------+
//    | /_/tmpl/{type}/({hash}|{name})/{lang} | GET    |         | text/plain    | Templated scrap.                  |      |
//    +----------------------------------------+--------+---------+---------------+-----------------------------------+------+
//    | /_/names/{type}/{name}                 | PUT    | Name    |               | Creates/Updates a named scrap.    | Y    |
//    +----------------------------------------+--------+---------+---------------+-----------------------------------+------+
//    | /_/names/{type}/{name}                 | GET    |         | Name          | Retrieves a single named scrap    |      |
//    +----------------------------------------+--------+---------+---------------+-----------------------------------+------+
//    | /_/names/{type}/{name}                 | DELETE |         |               | Deletes the scrap name.           | Y    |
//    +----------------------------------------+--------+---------+---------------+-----------------------------------+------+
//    | /_/_names/{type}/                      | GET    |         | []string      | Returns all named scraps of type. |      |
//    +----------------------------------------+--------+---------+---------------+-----------------------------------+------+
//
func (a *Api) AddHandlers(r *mux.Router, option Option) {
	if option == AddProtectedEndpoints {
		r.HandleFunc("/_/scraps/", a.scrapCreateHandler).Methods("POST")
	}

	scraps := r.Path("/_/scraps/{type:[a-z]+}/{hashOrName:[@0-9a-zA-Z-_]+}").Subrouter()
	scraps.Methods("GET").HandlerFunc(a.scrapGetHandler)
	if option == AddProtectedEndpoints {
		scraps.Methods("DELETE").HandlerFunc(a.scrapDeleteHandler)
	}

	r.HandleFunc("/_/raw/{type:[a-z]+}/{hashOrName:[@0-9a-zA-Z-_]+}", a.rawGetHandler).Methods("GET")
	r.HandleFunc("/_/tmpl/{type:[a-z]+}/{hashOrName:[@0-9a-zA-Z-_]+}/{lang:[a-z]+}", a.templateGetHandler).Methods("GET")

	names := r.Path("/_/names/{type:[a-z]+}/{name:@[0-9a-zA-Z-_]+}").Subrouter()
	names.Methods("GET").HandlerFunc(a.nameGetHandler)
	if option == AddProtectedEndpoints {
		names.Methods("PUT").HandlerFunc(a.namePutHandler)
		names.Methods("DELETE").HandlerFunc(a.nameDeleteHandler)
	}
	r.HandleFunc("/_/names/{type:[a-z]+}/", a.namesListHandler).Methods("GET")
}

// writeJSON writes 'body' as a JSON encoded HTTP response with the right
// mime-type, and logs errors if the body failed to write.
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

// scrapCreateHandler implements the REST API, see AddHandlers.
func (a *Api) scrapCreateHandler(w http.ResponseWriter, r *http.Request) {
	metrics2.GetCounter(scrapsCreateCallMetric).Inc(1)
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

// hashOrNameAndType extracts the hashOrName and Type from the URL, returning true if the Type was valid.
// If false is returned then an error has already been reported on the http.ResponseWriter.
func (a *Api) hashOrNameAndType(w http.ResponseWriter, r *http.Request) (string, scrap.Type, bool) {
	vars := mux.Vars(r)
	hashOrName := vars[hashOrNameVar]
	t, ok := a.getType(w, r)
	return hashOrName, t, ok
}

// nameAndType extracts the name and Type from the URL, returning true if the Type was valid.
// If false is returned then an error has already been reported on the http.ResponseWriter.
func (a *Api) nameAndType(w http.ResponseWriter, r *http.Request) (string, scrap.Type, bool) {
	vars := mux.Vars(r)
	name := vars[nameVar]
	t, ok := a.getType(w, r)
	return name, t, ok
}

// scrapGetHandler implements the REST API, see AddHandlers.
func (a *Api) scrapGetHandler(w http.ResponseWriter, r *http.Request) {
	metrics2.GetCounter(scrapsGetCallMetric).Inc(1)
	hashOrName, t, ok := a.hashOrNameAndType(w, r)
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

// scrapDeleteHandler implements the REST API, see AddHandlers.
func (a *Api) scrapDeleteHandler(w http.ResponseWriter, r *http.Request) {
	metrics2.GetCounter(scrapsDeleteCallMetric).Inc(1)
	hashOrName, t, ok := a.hashOrNameAndType(w, r)
	if !ok {
		return
	}

	err := a.scrapExchange.DeleteScrap(r.Context(), t, hashOrName)
	if err != nil {
		httputils.ReportError(w, err, "Failed to delete scrap.", http.StatusBadRequest)
		return
	}
}

// rawGetHandler implements the REST API, see AddHandlers.
func (a *Api) rawGetHandler(w http.ResponseWriter, r *http.Request) {
	metrics2.GetCounter(rawGetCallMetric).Inc(1)
	hashOrName, t, ok := a.hashOrNameAndType(w, r)
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

// templateGetHandler implements the REST API, see AddHandlers.
func (a *Api) templateGetHandler(w http.ResponseWriter, r *http.Request) {
	metrics2.GetCounter(templateGetCallMetric).Inc(1)
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

// namePutHandler implements the REST API, see AddHandlers.
func (a *Api) namePutHandler(w http.ResponseWriter, r *http.Request) {
	metrics2.GetCounter(namesPutCallMetric).Inc(1)
	name, t, ok := a.nameAndType(w, r)
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

// nameGetHandler implements the REST API, see AddHandlers.
func (a *Api) nameGetHandler(w http.ResponseWriter, r *http.Request) {
	metrics2.GetCounter(namesGetCallMetric).Inc(1)
	name, t, ok := a.nameAndType(w, r)
	if !ok {
		return
	}

	nameBody, err := a.scrapExchange.GetName(r.Context(), t, name)
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve Name.", http.StatusInternalServerError)
		return
	}

	writeJSON(w, nameBody)
}

// nameDeleteHandler implements the REST API, see AddHandlers.
func (a *Api) nameDeleteHandler(w http.ResponseWriter, r *http.Request) {
	metrics2.GetCounter(namesDeleteCallMetric).Inc(1)
	name, t, ok := a.nameAndType(w, r)
	if !ok {
		return
	}

	err := a.scrapExchange.DeleteName(r.Context(), t, name)
	if err != nil {
		httputils.ReportError(w, err, "Failed to delete Name.", http.StatusInternalServerError)
		return
	}
}

// namesListHandler implements the REST API, see AddHandlers.
func (a *Api) namesListHandler(w http.ResponseWriter, r *http.Request) {
	metrics2.GetCounter(namesListCallMetric).Inc(1)
	t, ok := a.getType(w, r)
	if !ok {
		return
	}

	names, err := a.scrapExchange.ListNames(r.Context(), t)
	if err != nil {
		httputils.ReportError(w, err, "Failed to load Names.", http.StatusInternalServerError)
		return
	}

	writeJSON(w, names)
}
