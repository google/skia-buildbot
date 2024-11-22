package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/favorites"
	favoriteMocks "go.skia.org/infra/perf/go/favorites/mocks"
)

func TestFrontendFavoritesHandler_Success(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/_/favorites/", nil)

	configFileBytes := testutils.ReadFileBytes(t, "config.json")
	err := json.Unmarshal(configFileBytes, &config.Config)
	require.NoError(t, err)

	login := mocks.NewLogin(t)
	login.On("LoggedInAs", r).Return(alogin.EMail("nobody@example.org"))

	favMocks := favoriteMocks.NewStore(t)
	fakeUserFavs := []*favorites.Favorite{
		{ID: "12345", UserId: "nobody@example.org"},
		{ID: "23456", UserId: "nobody@example.org"},
	}
	favMocks.On("List", testutils.AnyContext, "nobody@example.org").Return(fakeUserFavs, nil)

	f := NewFavoritesApi(login, favMocks)

	f.favoritesHandler(w, r)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	var favResp *config.Favorites
	err = json.Unmarshal(w.Body.Bytes(), &favResp)
	require.NoError(t, err)

	require.Equal(t, favResp.Sections[0].Name, "Section 1")
	require.Len(t, favResp.Sections[0].Links, 2)

	require.Equal(t, favResp.Sections[1].Name, "Section 2")
	require.Len(t, favResp.Sections[1].Links, 1)

	require.Equal(t, favResp.Sections[2].Name, "My Favorites")
	require.Len(t, favResp.Sections[2].Links, 2)
}

func TestFrontendNewFavoriteHandler_Success(t *testing.T) {
	w := httptest.NewRecorder()
	createFavReq := CreateFavRequest{
		Name:        "Fav1",
		Description: "Fav1 desc",
		Url:         "fav.com",
	}
	favBody, _ := json.Marshal(createFavReq)
	body := bytes.NewReader(favBody)
	r := httptest.NewRequest("POST", "/_/favorites/new", body)

	favMocks := favoriteMocks.NewStore(t)
	favMocks.On("Create", testutils.AnyContext, mock.Anything).Return(nil)

	login := mocks.NewLogin(t)
	login.On("LoggedInAs", r).Return(alogin.EMail("nobody@example.org"))

	f := NewFavoritesApi(login, favMocks)

	f.newFavoriteHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
}

func TestFrontendEditFavoriteHandler_Success(t *testing.T) {
	w := httptest.NewRecorder()
	updateFavReq := UpdateFavRequest{
		Id:          "12345",
		Name:        "Fav1",
		Description: "Fav1 desc",
		Url:         "fav.com",
	}
	favBody, _ := json.Marshal(updateFavReq)
	body := bytes.NewReader(favBody)
	r := httptest.NewRequest("POST", "/_/favorites/edit", body)

	favMocks := favoriteMocks.NewStore(t)
	favMocks.On("Update", testutils.AnyContext, mock.Anything, updateFavReq.Id).Return(nil)
	favMocks.On("Get", testutils.AnyContext, updateFavReq.Id).Return(&favorites.Favorite{ID: "12345", UserId: "nobody@example.org"}, nil)

	login := mocks.NewLogin(t)
	login.On("LoggedInAs", r).Return(alogin.EMail("nobody@example.org"))

	f := NewFavoritesApi(login, favMocks)

	f.updateFavoriteHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
}

func TestFrontendDeleteFavoriteHandler_Success(t *testing.T) {
	w := httptest.NewRecorder()
	deleteFavReq := DeleteFavRequest{
		Id: "12345",
	}
	favBody, _ := json.Marshal(deleteFavReq)
	body := bytes.NewReader(favBody)
	r := httptest.NewRequest("POST", "/_/favorites/delete", body)

	favMocks := favoriteMocks.NewStore(t)
	favMocks.On("Delete", testutils.AnyContext, "nobody@example.org", deleteFavReq.Id).Return(nil)

	login := mocks.NewLogin(t)
	login.On("LoggedInAs", r).Return(alogin.EMail("nobody@example.org"))

	f := NewFavoritesApi(login, favMocks)

	f.deleteFavoriteHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
}
