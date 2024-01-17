// Package auth provides an interface for handling authenticated users.
package auth

import (
	"context"
	"net/http"
)

// Auth is an abstraction of the functionality we use out of the go/login
// package.
type Auth interface {
	Init(ctx context.Context) error
	LoggedInAs(r *http.Request) (string, error)
	LoginURL(w http.ResponseWriter, r *http.Request) string
}
