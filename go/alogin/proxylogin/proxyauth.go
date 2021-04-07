// Package proxylogin implements alogin.Login when letting a reverse proxy handle
// authentication
package proxylogin

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/sklog"
)

// proxyLogin implements alogin.Login by relying on a reverse proxy doing the
// authentication and then passing the user's logged in status via header value.
//
// See https://grafana.com/docs/grafana/latest/auth/auth-proxy/ and
// https://cloud.google.com/iap/docs/identity-howto#getting_the_users_identity_with_signed_headers
type proxyLogin struct {
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
func New(headerName string, emailRegex *regexp.Regexp, loginURL, logoutURL string) *proxyLogin {
	return &proxyLogin{
		headerName: headerName,
		emailRegex: emailRegex,
		loginURL:   loginURL,
		logoutURL:  logoutURL,
	}
}

// LoggedInAs implements alogin.Login.
func (p *proxyLogin) LoggedInAs(r *http.Request) alogin.EMail {
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
func (p *proxyLogin) NeedsAuthentication(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Forbidden", http.StatusForbidden)
}

// RegisterHandlers implements alogin.Login.
func (p *proxyLogin) RegisterHandlers(router *mux.Router) {
	// Noop.
}

func (p *proxyLogin) Status(r *http.Request) alogin.Status {
	return alogin.Status{
		EMail:     p.LoggedInAs(r),
		LoginURL:  p.loginURL,
		LogoutURL: p.logoutURL,
	}
}

// Assert proxyLogin implements alogin.Login.
var _ alogin.Login = (*proxyLogin)(nil)
