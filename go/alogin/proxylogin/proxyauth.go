// Package proxylogin implements alogin.Login when letting a reverse proxy handle
// authentication
package proxylogin

import (
	"net/http"
	"regexp"
	"strings"

	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/authproxy"
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
func New(headerName, emailRegex, loginURL, logoutURL string) (*proxyLogin, error) {
	var compiledRegex *regexp.Regexp = nil
	var err error
	if emailRegex != "" {
		compiledRegex, err = regexp.Compile(emailRegex)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to compile email regex %q", emailRegex)
		}
	}

	return &proxyLogin{
		headerName: headerName,
		emailRegex: compiledRegex,
		loginURL:   loginURL,
		logoutURL:  logoutURL,
	}, nil
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

func (p *proxyLogin) Status(r *http.Request) alogin.Status {
	return alogin.Status{
		EMail:     p.LoggedInAs(r),
		LoginURL:  p.loginURL,
		LogoutURL: p.logoutURL,
	}
}

// All the authorized Roles for a user.
func (p *proxyLogin) Roles(r *http.Request) roles.Roles {
	return roles.FromHeader(r.Header.Get(authproxy.WebAuthRoleHeaderName))
}

// Returns true if the currently logged in user has the given Role.
func (p *proxyLogin) HasRole(r *http.Request, wantedRole roles.Role) bool {
	for _, role := range p.Roles(r) {
		if role == wantedRole {
			return true
		}
	}
	return false
}

// Assert proxyLogin implements alogin.Login.
var _ alogin.Login = (*proxyLogin)(nil)
