// Package alogin defines the Login interface for handling login in web
// applications.
//
// The implementations of Login should be used with the
// //infra-sk/modules/alogin-sk control.
package alogin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/sklog"
)

var (
	// loginCtxKey is used to store login information in the request context.
	loginCtxKey = &struct{}{}

	errNotLoggedIn = errors.New("not logged in")
)

// EMail is an email address.
type EMail string

// String returns the email address as a string.
func (e EMail) String() string {
	return string(e)
}

// NotLoggedIn is the EMail value used to indicate a user is not logged in.
const NotLoggedIn EMail = ""

// Status describes the logged in status for a user. Email will be empty if the
// user is not logged in.
type Status struct {
	// EMail is the email address of the logged in user, or the empty string if
	// they are not logged in.
	EMail EMail `json:"email"`

	// All the Roles of the current user.
	Roles roles.Roles `json:"roles"`
}

// Login provides information about the logged in status of http.Requests.
type Login interface {
	// LoggedInAs returns the email of the logged in user, or the empty string
	// of they are not logged in.
	LoggedInAs(r *http.Request) EMail

	// Status returns the logged in status and other details about the current
	// user.
	Status(r *http.Request) Status

	// All the authorized Roles for a user.
	Roles(r *http.Request) roles.Roles

	// Returns true if the currently logged in user has the given Role.
	HasRole(r *http.Request, role roles.Role) bool
}

// LoginStatusHandler returns an http.HandlerFunc that should be used to handle
// requests to "/_/login/status", which is the default location of the status
// handler in the alogin-sk element.
func LoginStatusHandler(login Login) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(login.Status(r)); err != nil {
			sklog.Errorf("Failed to send response: %s", err)
		}
	}
}

// StatusMiddleware is middleware which attaches login info to the request
// context. This allows handler to use GetSession() to retrieve the Session
// information even if the don't have access to the original http.Request
// object, like in a twirp handler.
func StatusMiddleware(login Login) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session := &Status{
				EMail: login.LoggedInAs(r),
				Roles: login.Roles(r),
			}
			ctx := context.WithValue(r.Context(), loginCtxKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetStatus returns the loggined in users email and roles from the context. If
// the user is not logged in then the empty session is returned, with an empty
// EMail address and empty Roles.
func GetStatus(ctx context.Context) *Status {
	session := ctx.Value(loginCtxKey)
	if session != nil {
		return session.(*Status)
	}
	return &Status{}
}

// FakeStatus is to be used by unit tests which want to fake that a user is logged in.
func FakeStatus(ctx context.Context, s *Status) context.Context {
	return context.WithValue(ctx, loginCtxKey, s)
}

// ForceRole is middleware that enforces the logged in user has the specified
// role before the wrapped handler is called.
func ForceRole(h http.Handler, login Login, role roles.Role) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !login.HasRole(r, role) {
			httputils.ReportError(w, errNotLoggedIn, fmt.Sprintf("You must be logged in as a(n) %s to complete this action.", role), http.StatusUnauthorized)
			return
		}

		h.ServeHTTP(w, r)
	})
}

// ForceRoleMiddleware returns a mux.MiddlewareFunc that restricts access to
// only those users that have the given role.
func ForceRoleMiddleware(login Login, role roles.Role) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !login.HasRole(r, role) {
				httputils.ReportError(w, errNotLoggedIn, fmt.Sprintf("You must be logged in as a(n) %s to complete this action.", role), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
