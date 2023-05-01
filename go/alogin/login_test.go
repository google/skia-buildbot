// Package alogin defines the Login interface for handling login in web
// applications.
//
// The implementations of Login should be used with the
// //infra-sk/modules/alogin-sk control.
package alogin_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/mocks"
	"go.skia.org/infra/go/roles"
)

const (
	email alogin.EMail = "user@example.com"
)

func TestSessionMiddleware_UserIsLoggedIn_SessionIsReturedWithEmailAndRoles(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	login := mocks.NewLogin(t)
	login.On("Roles", r).Return(roles.Roles{roles.Editor})
	login.On("LoggedInAs", r).Return(email)

	called := false
	m := alogin.StatusMiddleware(login)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := alogin.GetStatus(r.Context())
		require.Equal(t, &alogin.Status{
			EMail: email,
			Roles: roles.Roles{roles.Editor},
		}, s)
		called = true
	}))

	m.ServeHTTP(w, r)
	require.True(t, called)
}

func TestSessionMiddleware_UserIsNotLoggedIn_EmptySessionIsRetured(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	s := alogin.GetStatus(r.Context())
	require.Equal(t, &alogin.Status{}, s)
}

func TestLoginStatusHandler(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	login := mocks.NewLogin(t)
	login.On("Status", r).Return(alogin.Status{
		EMail: email,
		Roles: roles.Roles{roles.Editor},
	})

	alogin.LoginStatusHandler(login)(w, r)
	var status alogin.Status
	err := json.Unmarshal(w.Body.Bytes(), &status)
	require.NoError(t, err)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestForceRole_UserIsLoggedIn_HandlerIsCalled(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	login := mocks.NewLogin(t)
	login.On("HasRole", r, roles.Editor).Return(true)

	called := false
	m := alogin.ForceRole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}), login, roles.Editor)

	m.ServeHTTP(w, r)
	require.True(t, called)
	require.Equal(t, w.Code, http.StatusOK)
}

func TestForceRole_UserIsNotLoggedIn_HandlerIsNotCalled(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	login := mocks.NewLogin(t)
	login.On("HasRole", r, roles.Viewer).Return(false)

	called := false
	m := alogin.ForceRole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}), login, roles.Viewer)

	m.ServeHTTP(w, r)
	require.False(t, called)
	require.Equal(t, w.Code, http.StatusUnauthorized)
}

func TestForceRoleMiddleware_UserIsLoggedIn_HandlerCalled(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	login := mocks.NewLogin(t)
	login.On("HasRole", r, roles.Viewer).Return(true)

	called := false
	m := alogin.ForceRoleMiddleware(login, roles.Viewer)
	h := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	h.ServeHTTP(w, r)
	require.True(t, called)
	require.Equal(t, w.Code, http.StatusOK)
}

func TestForceRoleMiddleware_UserIsNotLoggedIn_HandleIsNotCalled(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	login := mocks.NewLogin(t)
	login.On("HasRole", r, roles.Viewer).Return(false)

	called := false
	m := alogin.ForceRoleMiddleware(login, roles.Viewer)
	h := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	h.ServeHTTP(w, r)
	require.False(t, called)
	require.Equal(t, w.Code, http.StatusUnauthorized)
}
