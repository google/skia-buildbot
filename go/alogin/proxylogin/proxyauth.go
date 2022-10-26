// Package proxylogin implements alogin.Login when letting a reverse proxy
// handle authentication.
package proxylogin

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/authproxy"
)

const (
	// DefaultLoginURL is the default URL to use for logging in.
	DefaultLoginURL = "https://skia.org/login/"

	// DefaultLogoutURL is the default URL to use for logging out.
	DefaultLogoutURL = "https://skia.org/logout/"
)

var (
	errNotLoggedIn = errors.New("not logged in")
)

// ProxyLogin implements alogin.Login by relying on a reverse proxy doing the
// authentication and then passing the user's logged in status via header value.
//
// See https://grafana.com/docs/grafana/latest/auth/auth-proxy/ and
// https://cloud.google.com/iap/docs/identity-howto#getting_the_users_identity_with_signed_headers
type ProxyLogin struct {
	// headerName is the name of the header we expect to have the users email.
	headerName string

	// emailRegex is an optional regex to extract the email address from the header value.
	emailRegex *regexp.Regexp

	// loginURL is the URL to visit to log in.
	loginURL string

	// logoutURL is the URL to visit to log out.
	logoutURL string
}

// New returns a new instance of proxyLogin.
//
// headerName is the name of the header that contains the proxy authentication
// information.
//
// emailRegex is a regex to extract the email address from the header value.
// This value can be nil. This is useful for reverse proxies that include other
// information in the header in addition to the email address, such as
// https://cloud.google.com/iap/docs/identity-howto#getting_the_users_identity_with_signed_headers
//
// If supplied, the Regex must have a single subexpression that matches the email
// address.
func New(headerName, emailRegex, loginURL, logoutURL string) (*ProxyLogin, error) {
	var compiledRegex *regexp.Regexp = nil
	var err error
	if emailRegex != "" {
		compiledRegex, err = regexp.Compile(emailRegex)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to compile email regex %q", emailRegex)
		}
	}

	return &ProxyLogin{
		headerName: headerName,
		emailRegex: compiledRegex,
		loginURL:   loginURL,
		logoutURL:  logoutURL,
	}, nil
}

// NewWithDefaults calls New() with reasonable default values.
func NewWithDefaults() *ProxyLogin {
	return &ProxyLogin{
		headerName: authproxy.WebAuthHeaderName,
		emailRegex: nil,
		loginURL:   DefaultLoginURL,
		logoutURL:  DefaultLogoutURL,
	}
}

// LoggedInAs implements alogin.Login.
func (p *ProxyLogin) LoggedInAs(r *http.Request) alogin.EMail {
	value := r.Header.Get(p.headerName)
	value = strings.TrimSpace(value)
	if p.emailRegex == nil {
		return alogin.EMail(value)
	}
	submatches := p.emailRegex.FindStringSubmatch(value)
	if len(submatches) != 2 {
		sklog.Errorf("Wrong number of regex matches for %q: %q", value, submatches)
		return ""
	}
	return alogin.EMail(submatches[1])
}

// NeedsAuthentication implements alogin.Login.
func (p *ProxyLogin) NeedsAuthentication(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Forbidden", http.StatusForbidden)
}

// Status implements alogin.Login.
func (p *ProxyLogin) Status(r *http.Request) alogin.Status {
	return alogin.Status{
		EMail:     p.LoggedInAs(r),
		LoginURL:  p.loginURL,
		LogoutURL: p.logoutURL,
	}
}

// Roles implements alogin.Login.
func (p *ProxyLogin) Roles(r *http.Request) roles.Roles {
	return roles.FromHeader(r.Header.Get(authproxy.WebAuthRoleHeaderName))
}

// HasRole implements alogin.Login.
func (p *ProxyLogin) HasRole(r *http.Request, wantedRole roles.Role) bool {
	for _, role := range p.Roles(r) {
		if role == wantedRole {
			return true
		}
	}
	return false
}

// Assert proxyLogin implements alogin.Login.
var _ alogin.Login = (*ProxyLogin)(nil)

// ForceRole is middleware that enforces the logged in user has the specified
// role before the wrapped handler is called.
func ForceRole(h http.Handler, login alogin.Login, role roles.Role) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if login.HasRole(r, role) {
			httputils.ReportError(w, errNotLoggedIn, fmt.Sprintf("You must be logged in as a(n) %s to complete this action.", role), http.StatusUnauthorized)
			return
		}

		h.ServeHTTP(w, r)
	})
}

// ForceRoleMiddleware returns a mux.MiddlewareFunc that restricts access to
// only those users that have the given role.
func ForceRoleMiddleware(login alogin.Login, role roles.Role) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !login.HasRole(r, role) {
				httputils.ReportError(w, errNotLoggedIn, fmt.Sprintf("You must be logged in as a(n) %s to complete this action.", role), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
