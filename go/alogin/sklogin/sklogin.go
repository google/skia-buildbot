// Package sklogin implmements alogin.Login using the //go/login package.
package sklogin

import (
	"fmt"
	"net/http"

	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/skerr"
)

const (
	loginPath  = "/login/"
	logoutPath = "/logout/"
)

// sklogin implements alogin.Login using the //go/login package.
type sklogin struct{}

// New returns a new sklogin instance.
func New(port string, local bool, authBypassList string) (*sklogin, error) {
	redirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", port)
	if !local {
		redirectURL = login.DEFAULT_REDIRECT_URL
	}
	if err := login.Init(redirectURL, authBypassList, ""); err != nil {
		return nil, skerr.Wrap(err)
	}

	return &sklogin{}, nil
}

// LoggedInAs implements alogin.Login.
func (_ *sklogin) LoggedInAs(r *http.Request) alogin.EMail {
	return alogin.EMail(login.LoggedInAs(r))
}

// NeedsAuthentication implements alogin.Login.
func (_ *sklogin) NeedsAuthentication(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, login.LoginURL(w, r), http.StatusTemporaryRedirect)
}

func (s *sklogin) Status(r *http.Request) alogin.Status {
	return alogin.Status{
		EMail:     s.LoggedInAs(r),
		LoginURL:  loginPath,
		LogoutURL: logoutPath,
	}
}

// All the authorized Roles for a user.
func (s *sklogin) Roles(r *http.Request) roles.Roles {
	panic("sklogin does not support Roles.")
}

// Returns true if the currently logged in user has the given Role.
func (s *sklogin) HasRole(r *http.Request, role roles.Role) bool {
	panic("sklogin does not support Roles.")
}

// Assert sklogin implements alogin.Login.
var _ alogin.Login = (*sklogin)(nil)
