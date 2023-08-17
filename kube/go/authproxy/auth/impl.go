package auth

import (
	"context"
	"net/http"
	"net/url"

	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/netutils"
)

// authImpl implements Auth using the login package.
type authImpl struct{}

// New returns a new authImpl.
func New() authImpl {
	return authImpl{}
}

// LoggedInAs implements Auth.
func (l authImpl) LoggedInAs(r *http.Request) string {
	return login.AuthenticatedAs(r)
}

// LoggedInAs implements Auth.
func (l authImpl) LoginURL(w http.ResponseWriter, r *http.Request) string {
	var u url.URL
	u.Host = netutils.RootDomain(r.Host)
	u.Scheme = "https"
	u.Path = login.LoginPath

	return u.String()
}

func (l authImpl) Init(ctx context.Context) error {
	return login.InitVerifyOnly(
		ctx,
		"",
	)
}

// Confirm authImpl implements Auth.
var _ Auth = authImpl{}
