package proxylogin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/kube/go/authproxy"
)

const (
	goodHeaderName                 = "X-AUTH-USER"
	unknownHeaderName              = "X-SOME-UNKNOWN-HEADER"
	email             alogin.EMail = "someone@example.org"
	emailAsString     string       = string(email)
	loginURL                       = "https://example.org/login"
	logoutURL                      = "https://example.org/logout"
)

func TestLoggedInAs_HeaderIsMissing_ReturnsEmptyString(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	login, err := New(unknownHeaderName, "", loginURL, logoutURL)
	require.NoError(t, err)
	require.Equal(t, alogin.NotLoggedIn, login.LoggedInAs(r))
}

func TestLoggedInAs_HeaderPresent_ReturnsUserEmail(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(goodHeaderName, emailAsString)
	login, err := New(goodHeaderName, "", loginURL, logoutURL)
	require.NoError(t, err)
	require.Equal(t, email, login.LoggedInAs(r))
}

func TestLoggedInAs_RegexProvided_ReturnsUserEmail(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(goodHeaderName, "accounts.google.com:"+emailAsString)
	login, err := New(goodHeaderName, "accounts.google.com:(.*)", loginURL, logoutURL)
	require.NoError(t, err)
	require.Equal(t, email, login.LoggedInAs(r))
}

func TestLoggedInAs_RegexHasTooManySubGroups_ReturnsEmptyString(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(goodHeaderName, emailAsString)
	login, err := New(goodHeaderName, "(too)(many)(subgroups)", loginURL, logoutURL)
	require.NoError(t, err)
	require.Equal(t, alogin.NotLoggedIn, login.LoggedInAs(r))
}

func TestNeedsAuthentication_EmitsStatusForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	login, err := New(goodHeaderName, "", loginURL, logoutURL)
	require.NoError(t, err)
	login.NeedsAuthentication(w, r)
	require.Equal(t, http.StatusForbidden, w.Result().StatusCode)
}

func TestStatus_HeaderPresent_ReturnsUserEmail(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(goodHeaderName, emailAsString)
	expected := alogin.Status{
		EMail:     email,
		LoginURL:  loginURL,
		LogoutURL: logoutURL,
	}
	login, err := New(goodHeaderName, "", loginURL, logoutURL)
	require.NoError(t, err)
	require.Equal(t, expected, login.Status(r))
}

func TestNew_InvalidRegex_ReturnsError(t *testing.T) {
	_, err := New(goodHeaderName, "\\y", loginURL, logoutURL)
	require.Error(t, err)
}

func TestRoles_HeaderPresent_ReturnAllRoles(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(authproxy.WebAuthRoleHeaderName, roles.AllValidRoles.ToHeader())
	login, err := New(goodHeaderName, "", loginURL, logoutURL)
	require.NoError(t, err)
	require.Equal(t, roles.AllValidRoles, login.Roles(r))
}

func TestRoles_HeaderMissing_ReturnsEmptyListOfRoles(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	login, err := New(goodHeaderName, "", loginURL, logoutURL)
	require.NoError(t, err)
	require.Empty(t, login.Roles(r))
}

func TestHasRoles_HeaderPresent_ReturnsTrue(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(authproxy.WebAuthRoleHeaderName, roles.AllValidRoles.ToHeader())
	login, err := New(goodHeaderName, "", loginURL, logoutURL)
	require.NoError(t, err)
	require.True(t, login.HasRole(r, roles.Admin))
}

func TestHasRoles_HeaderMissingPresent_ReturnsFalse(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	login, err := New(goodHeaderName, "", loginURL, logoutURL)
	require.NoError(t, err)
	require.False(t, login.HasRole(r, roles.Admin))
}
