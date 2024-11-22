package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/graphsshortcut"
	"go.skia.org/infra/perf/go/shortcut"
)

// shortcutsApi provides a struct for handling shortcut api endpoints.
type shortcutsApi struct {
	shortcutStore       shortcut.Store
	graphsShortcutStore graphsshortcut.Store
}

// NewShortCutsApi returns a new instance of the shortcutsApi struct.
func NewShortCutsApi(shortcutStore shortcut.Store, graphsShortcutStore graphsshortcut.Store) shortcutsApi {
	return shortcutsApi{
		shortcutStore:       shortcutStore,
		graphsShortcutStore: graphsShortcutStore,
	}
}

// RegisterHandlers registers the api handlers for their respective routes.
func (api shortcutsApi) RegisterHandlers(router *chi.Mux) {
	router.Post("/_/keys/", api.keysHandler)
	router.Post("/_/shortcut/get", api.getGraphsShortcutHandler)
	router.Post("/_/shortcut/update", api.createGraphsShortcutHandler)
}

// keysHandler handles the POST requests of a list of keys.
//
//	{
//	   "keys": [
//	        ",arch=x86,...",
//	        ",arch=x86,...",
//	   ]
//	}
//
// And returns the ID of the new shortcut to that list of keys:
//
//	{
//	  "id": 123456,
//	}
func (api shortcutsApi) keysHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	id, err := api.shortcutStore.Insert(ctx, r.Body)
	if err != nil {
		httputils.ReportError(w, err, "Error inserting shortcut.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"id": id}); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

type GetGraphsShortcutRequest struct {
	ID string `json:"id"`
}

// getGraphsShortcutHandler returns the graphsShortcut details for the given shortcut id.
func (api shortcutsApi) getGraphsShortcutHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	var ggsr GetGraphsShortcutRequest
	if err := json.NewDecoder(r.Body).Decode(&ggsr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	sc, err := api.graphsShortcutStore.GetShortcut(ctx, ggsr.ID)

	if err != nil {
		httputils.ReportError(w, err, "Failed to get keys shortcut.", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(sc); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

// createGraphsShortcutHandler creates a new graphsShortcut from the supplied information.
func (api shortcutsApi) createGraphsShortcutHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	shortcut := &graphsshortcut.GraphsShortcut{}
	if err := json.NewDecoder(r.Body).Decode(shortcut); err != nil {
		httputils.ReportError(w, err, "Unable to read shortcut body.", http.StatusInternalServerError)
		return
	}

	id, err := api.graphsShortcutStore.InsertShortcut(ctx, shortcut)
	if err != nil {
		httputils.ReportError(w, err, "Error inserting graphs shortcut.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"id": id}); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}
