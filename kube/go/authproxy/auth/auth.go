// Package auth provides an interface for handling authenticated users.
package auth

import (
	"net/http"
)

// Auth is an abstraction of the functionality we use out of the go/login
// package.
type Auth interface {
	Init(port string, local bool) error
	LoggedInAs(r *http.Request) string
	LoginURL(w http.ResponseWriter, r *http.Request) string
}
