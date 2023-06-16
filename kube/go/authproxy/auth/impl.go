package auth

import (
	"context"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/login"
)

// authImpl implements Auth using the login package.
type authImpl struct{}

// New returns a new authImpl.
func New() authImpl {
	return authImpl{}
}

// LoggedInAs implements Auth.
func (l authImpl) LoggedInAs(r *http.Request) string {
	return login.LoggedInAs(r)
}

// LoggedInAs implements Auth.
func (l authImpl) LoginURL(w http.ResponseWriter, r *http.Request) string {
	return login.LoginURL(w, r)
}

func (l authImpl) Init(ctx context.Context, port string, local bool) error {
	redirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", port)
	if !local {
		redirectURL = login.GetDefaultRedirectURL()
	}

	return login.Init(
		ctx,
		redirectURL,
	)
}

// Confirm authImpl implements Auth.
var _ Auth = authImpl{}
