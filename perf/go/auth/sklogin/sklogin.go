// Package sklogin implmements //perf/go/auth using the //go/login package.
package sklogin

import (
	"net/http"

	"go.skia.org/infra/go/login"
)

type sklogin struct{}

// New returns a new sklogin instance.
func New() sklogin {
	return sklogin{}
}

// LoggedInAs implements Auth.
func (_ sklogin) LoggedInAs(r *http.Request) string {
	return login.LoggedInAs(r)
}

// NeedsAuthentication implements Auth.
func (_ sklogin) NeedsAuthentication(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, login.LoginURL(w, r), http.StatusTemporaryRedirect)
}
