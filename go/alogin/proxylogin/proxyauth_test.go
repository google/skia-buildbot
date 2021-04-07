package proxylogin

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/testutils/unittest"
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
	unittest.SmallTest(t)

	r := httptest.NewRequest("GET", "/", nil)
	require.Equal(t, alogin.NotLoggedIn, New(unknownHeaderName, nil, loginURL, logoutURL).LoggedInAs(r))
}

func TestLoggedInAs_HeaderPresent_ReturnsUserEmail(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(goodHeaderName, emailAsString)
	require.Equal(t, email, New(goodHeaderName, nil, loginURL, logoutURL).LoggedInAs(r))
}

func TestLoggedInAs_RegexProvided_ReturnsUserEmail(t *testing.T) {
	unittest.SmallTest(t)

	reg := regexp.MustCompile("accounts.google.com:(.*)")
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(goodHeaderName, "accounts.google.com:"+emailAsString)
	require.Equal(t, email, New(goodHeaderName, reg, loginURL, logoutURL).LoggedInAs(r))
}

func TestLoggedInAs_RegexHasTooManySubGroups_ReturnsEmptyString(t *testing.T) {
	unittest.SmallTest(t)

	reg := regexp.MustCompile("(too)(many)(subgroups)")
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(goodHeaderName, emailAsString)
	require.Equal(t, alogin.NotLoggedIn, New(goodHeaderName, reg, loginURL, logoutURL).LoggedInAs(r))
}

func TestNeedsAuthentication_EmitsStatusForbidden(t *testing.T) {
	unittest.SmallTest(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	New(goodHeaderName, nil, loginURL, logoutURL).NeedsAuthentication(w, r)
	require.Equal(t, http.StatusForbidden, w.Result().StatusCode)
}

func TestStatus_HeaderPresent_ReturnsUserEmail(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(goodHeaderName, emailAsString)
	expected := alogin.Status{
		EMail:     email,
		LoginURL:  loginURL,
		LogoutURL: logoutURL,
	}
	require.Equal(t, expected, New(goodHeaderName, nil, loginURL, logoutURL).Status(r))
}
