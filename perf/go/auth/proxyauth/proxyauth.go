// Package proxyauth implements Auth when letting a reverse proxy handle
// authentication
package proxyauth

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/auth"
)

// proxyAuth implements Auth by relying on a reverse proxy doing the
// authentication and then passing the user's logged in status via header value.
//
// See https://grafana.com/docs/grafana/latest/auth/auth-proxy/ and
// https://cloud.google.com/iap/docs/identity-howto#getting_the_users_identity_with_signed_headers
type proxyAuth struct {
	// headerName is the name of the header we expect to have the users email.
	headerName string

	// emailRegex is an optional regex to extract the email address from the header value.
	emailRegex *regexp.Regexp
}

// New returns a new instance of proxyAuth.
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
func New(headerName string, emailRegex *regexp.Regexp) *proxyAuth {
	return &proxyAuth{
		headerName: headerName,
		emailRegex: emailRegex,
	}
}

// LoggedInAs implements Auth.
func (p *proxyAuth) LoggedInAs(r *http.Request) string {
	value := r.Header.Get(p.headerName)
	value = strings.TrimSpace(value)
	if p.emailRegex == nil {
		return value
	}
	submatches := p.emailRegex.FindStringSubmatch(value)
	if len(submatches) != 2 {
		sklog.Errorf("Wrong number of regex matches for %q: %q", value, submatches)
		return ""
	}
	return submatches[1]
}

// NeedsAuthentication implements Auth.
func (p *proxyAuth) NeedsAuthentication(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Forbidden", http.StatusForbidden)
}

// RegisterHandlers implements Auth.
func (p *proxyAuth) RegisterHandlers(router *mux.Router) {
	// Noop.
}

// Assert proxyAuth implements Auth.
var _ auth.Auth = (*proxyAuth)(nil)
