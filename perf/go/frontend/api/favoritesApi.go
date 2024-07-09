package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/favorites"
)

const (
	defaultDatabaseTimeout = time.Minute
)

// favoritesApi provides a struct for handling Favorites feature.
type favoritesApi struct {
	loginProvider alogin.Login
	favStore      favorites.Store
}

// NewFavoritesApi returns a new instance of favoritesApi.
func NewFavoritesApi(loginProvider alogin.Login, favoritesStore favorites.Store) favoritesApi {
	return favoritesApi{
		loginProvider: loginProvider,
		favStore:      favoritesStore,
	}
}

// RegisterHandlers registers the api handlers for their respective routes.
func (f favoritesApi) RegisterHandlers(router *chi.Mux) {
	router.Get("/_/favorites/", f.favoritesHandler)
	router.Post("/_/favorites/new", f.newFavoriteHandler)
	router.Post("/_/favorites/delete", f.deleteFavoriteHandler)
	router.Post("/_/favorites/edit", f.updateFavoriteHandler)
}

// favoritesHandler returns the favorites config for the instance. If a user is
// logged in this also returns the favorites specific to the logged in user.
func (f favoritesApi) favoritesHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	fav := config.Favorites{
		Sections: []config.FavoritesSectionConfig{},
	}

	if config.Config.Favorites.Sections != nil {
		fav.Sections = config.Config.Favorites.Sections
	}

	loggedInEmail := f.loginProvider.LoggedInAs(r)
	if loggedInEmail != "" {
		favsFromDb, err := f.favStore.List(ctx, loggedInEmail.String())
		if err != nil {
			httputils.ReportError(w, err, "Failed to list favorite.", http.StatusInternalServerError)
			return
		}

		favoriteList := []config.FavoritesSectionLinkConfig{}

		for _, favorite := range favsFromDb {
			favoriteList = append(favoriteList, config.FavoritesSectionLinkConfig{
				Id:          favorite.ID,
				Text:        favorite.Name,
				Href:        favorite.Url,
				Description: favorite.Description,
			})
		}

		if len(favsFromDb) > 0 {
			fav.Sections = append(fav.Sections, config.FavoritesSectionConfig{
				Name:  "My Favorites",
				Links: favoriteList,
			})
		}
	}

	if err := json.NewEncoder(w).Encode(fav); err != nil {
		sklog.Errorf("Error writing the Favorites json to response: %s", err)
	}
}

// CreateFavRequest is the request to create a new Favorite
type CreateFavRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Url         string `json:"url"`
}

// newFavoriteHandler creates a new favorite in the db
func (f *favoritesApi) newFavoriteHandler(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	loggedInEmail := f.loginProvider.LoggedInAs(r)
	if loggedInEmail == "" {
		httputils.ReportError(w, skerr.Fmt("Favorite Error:"), "Login Required", http.StatusUnauthorized)
		return
	}

	var createReq CreateFavRequest
	if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	saveReq := favorites.SaveRequest{
		UserId:      loggedInEmail.String(),
		Name:        createReq.Name,
		Description: createReq.Description,
		Url:         createReq.Url,
	}
	err := f.favStore.Create(ctx, &saveReq)
	if err != nil {
		httputils.ReportError(w, err, "Failed to create favorite.", http.StatusInternalServerError)
	}
}

// DeleteFavRequest is a request to delete an existing Favorite
type DeleteFavRequest struct {
	Id string `json:"id"`
}

// deleteFavoriteHandler deletes a favorite per id in the db
func (f *favoritesApi) deleteFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	loggedInEmail := f.loginProvider.LoggedInAs(r)
	if loggedInEmail == "" {
		httputils.ReportError(w, skerr.Fmt("Favorite Error:"), "Login Required", http.StatusUnauthorized)
		return
	}

	var deleteReq DeleteFavRequest
	if err := json.NewDecoder(r.Body).Decode(&deleteReq); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	err := f.favStore.Delete(ctx, loggedInEmail.String(), deleteReq.Id)
	if err != nil {
		httputils.ReportError(w, skerr.Fmt("Favorite Error:"), "Failed to delete favorite.", http.StatusInternalServerError)
	}
}

// UpdateFavRequest is a request to update an existing Favorite
type UpdateFavRequest struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Url         string `json:"url"`
}

// updateFavoriteHandler updates a new favorite per id in the db
func (f *favoritesApi) updateFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	loggedInEmail := f.loginProvider.LoggedInAs(r)
	if loggedInEmail == "" {
		httputils.ReportError(w, skerr.Fmt("Favorite Error:"), "Login Required", http.StatusUnauthorized)
		return
	}

	var updateReq UpdateFavRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	fav, err := f.favStore.Get(ctx, updateReq.Id)
	if err != nil {
		httputils.ReportError(w, skerr.Fmt("Favorite Error:"), "Invalid id", http.StatusNotFound)
		return
	}

	if fav.UserId != loggedInEmail.String() {
		httputils.ReportError(w, skerr.Fmt("Favorite Error:"), "Unauthorized action", http.StatusForbidden)
		return
	}

	saveReq := favorites.SaveRequest{
		UserId:      loggedInEmail.String(),
		Name:        updateReq.Name,
		Description: updateReq.Description,
		Url:         updateReq.Url,
	}
	err = f.favStore.Update(ctx, &saveReq, fav.ID)
	if err != nil {
		httputils.ReportError(w, skerr.Fmt("Favorite Error:"), "Failed to update favorite.", http.StatusInternalServerError)
	}
}
