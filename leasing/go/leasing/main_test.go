package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func TestAddHandlers_UnauthorizedGetToTaskStatusURI_DoesNotReturn401(t *testing.T) {
	srv := &Server{}
	router := chi.NewRouter()
	srv.AddHandlers(router)

	r := httptest.NewRequest("GET", getTaskStatusURI, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	// Expect an internal server error, which means we actually made it into the
	// handler. The 500 is expected because we passed nil as the body.
	require.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)

	// Test the test, confirm the a request to "/" DOES return a 401.
	r = httptest.NewRequest("GET", "/", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	require.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)

}
