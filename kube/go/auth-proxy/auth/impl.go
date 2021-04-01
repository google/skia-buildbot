package auth

import (
	"net/http"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/login"
)

// authImpl implements Auth using the login package.
type authImpl struct{}

func New() authImpl {
	return authImpl{}
}

// LoggedInAs implements Auth.
func (l authImpl) LoggedInAs(r *http.Request) string {
	return login.LoggedInAs(r)
}

// LoggedInAs implements Auth.
func (l authImpl) IsViewer(r *http.Request) bool {
	return login.IsViewer(r)
}

// LoggedInAs implements Auth.
func (l authImpl) LoginURL(w http.ResponseWriter, r *http.Request) string {
	return login.LoginURL(w, r)
}

func (l authImpl) SimpleInitWithAllow(port string, local bool, admin, edit, view allowed.Allow) {
	login.SimpleInitWithAllow(port, local, admin, edit, view)
}
