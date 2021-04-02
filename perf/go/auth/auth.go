package auth

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Auth is an abstraction of the functionality we use out of the go/login
// package.
type Auth interface {
	// LoggedInAs returns the email of the logged in user, or the empty string
	// of they are not logged in.
	LoggedInAs(r *http.Request) string

	// NeedsAuthentication will send the right response to the user if they
	// attempt to use a resource that requires authentication, such as
	// redirecting them to a login URL or returning an http.StatusForbidden
	// response code.
	NeedsAuthentication(w http.ResponseWriter, r *http.Request)

	// RegisterHandlers registers HTTP handlers for any endpoints that need
	// handling.
	RegisterHandlers(router *mux.Router)
}
