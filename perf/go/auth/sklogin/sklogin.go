// Package sklogin implmements //perf/go/auth using the //go/login package.
package sklogin

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/auth"
)

// sklogin implements Auth using the //go/login package.
type sklogin struct{}

// New returns a new sklogin instance.
func New(port string, local bool, authBypassList string) (*sklogin, error) {
	redirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", port)
	if !local {
		redirectURL = login.DEFAULT_REDIRECT_URL
	}
	if authBypassList == "" {
		authBypassList = login.DEFAULT_ALLOWED_DOMAINS
	}
	if err := login.Init(redirectURL, authBypassList, ""); err != nil {
		return nil, skerr.Wrap(err)
	}

	return &sklogin{}, nil
}

// LoggedInAs implements Auth.
func (_ *sklogin) LoggedInAs(r *http.Request) string {
	return login.LoggedInAs(r)
}

// NeedsAuthentication implements Auth.
func (_ *sklogin) NeedsAuthentication(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, login.LoginURL(w, r), http.StatusTemporaryRedirect)
}

// RegisterHandlers implements Auth.
func (_ *sklogin) RegisterHandlers(router *mux.Router) {
	router.HandleFunc("/logout/", login.LogoutHandler)
	router.HandleFunc("/loginstatus/", login.StatusHandler)
	router.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
}

// Assert sklogin implements Auth.
var _ auth.Auth = (*sklogin)(nil)
