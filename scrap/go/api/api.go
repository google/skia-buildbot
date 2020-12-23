// Package api implements the REST API for the scrap exchange service.
package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/gcs/gcsclient"
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

// ApplyHandlers hooks up the scrap exchange REST API to the given router.
//
// If exposeProtectedEndpoints is true then the mutating endpoints, such as the
// one for creating scraps, are added, otherwise they are skipped.
func (a *Api) ApplyHandlers(r *mux.Router, exposeProtectedEndpoints bool) {
	if exposeProtectedEndpoints {
		r.HandleFunc("/_/scraps/", a.scrapCreateHandler).Methods("POST")
	}

	scraps := r.Path("/_/scraps/{type:[a-z]+}/{hash:[0-9a-f]+}").Subrouter()
	scraps.Methods("GET").HandlerFunc(a.scrapGetHandler)
	if exposeProtectedEndpoints {
		scraps.Methods("DELETE").HandlerFunc(a.scrapDeleteHandler)
	}

	r.HandleFunc("/_/raw/{type:[a-z]+}/{hash:[0-9a-f]+}", a.rawGetHandler).Methods("GET")

	r.HandleFunc("/_/tmpl/{type:[a-z]+}/{hash:[0-9a-f]+}/{lang:[a-z]+}", a.templateGetHandler).Methods("GET")

	names := r.Path("/_/names/{type:[a-z]+}/{name:[0-9a-zA-Z-_]+}")
	names.Methods("GET").HandlerFunc(a.nameGetHandler)
	if exposeProtectedEndpoints {
		names.Methods("PUT").HandlerFunc(a.namePutHandler)
	}
	r.HandleFunc("/_/names/{type:[a-z]+}/", a.namesListHandler).Methods("GET")
}

func (a *Api) scrapCreateHandler(w http.ResponseWriter, r *http.Request) {
}

func (a *Api) scrapGetHandler(w http.ResponseWriter, r *http.Request) {}

func (a *Api) scrapDeleteHandler(w http.ResponseWriter, r *http.Request) {}

func (a *Api) rawGetHandler(w http.ResponseWriter, r *http.Request) {}

func (a *Api) templateGetHandler(w http.ResponseWriter, r *http.Request) {}

func (a *Api) namePutHandler(w http.ResponseWriter, r *http.Request) {}

func (a *Api) nameGetHandler(w http.ResponseWriter, r *http.Request) {}

func (a *Api) namesListHandler(w http.ResponseWriter, r *http.Request) {}
