package auth

import (
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
func (l authImpl) IsViewer(r *http.Request) bool {
	return login.IsViewer(r)
}

// LoggedInAs implements Auth.
func (l authImpl) LoginURL(w http.ResponseWriter, r *http.Request) string {
	return login.LoginURL(w, r)
}

func (l authImpl) Init(port string, local bool) error {
	redirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", port)
	if !local {
		redirectURL = login.DEFAULT_REDIRECT_URL
	}

	return login.Init(redirectURL,
		"", /* Empty means accept all signed in domain. */
		"", /* Get secrets from Secret Manager*/
	)
}

// Confirm authImpl implements Auth.
var _ Auth = authImpl{}
