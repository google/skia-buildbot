// Package proxyauth implements Auth when letting a reverse proxy handle
// authentication
package proxyauth

import (
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	goodHeaderName    = "X-AUTH-USER"
	unknownHeaderName = "X-SOME-UNKNOWN-HEADER"
	email             = "someone@example.org"
)

func TestLoggedInAs_HeaderIsMissing_ReturnsEmptyString(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("GET", "/", nil)
	require.Equal(t, "", New(unknownHeaderName, nil).LoggedInAs(r))
}

func TestLoggedInAs_HeaderPresent_ReturnsUserEmail(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(goodHeaderName, email)
	require.Equal(t, email, New(goodHeaderName, nil).LoggedInAs(r))
}

func TestLoggedInAs_RegexProvided_ReturnsUserEmail(t *testing.T) {
	unittest.SmallTest(t)

	reg := regexp.MustCompile("accounts.google.com:(.*)")
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set(goodHeaderName, "accounts.google.com:"+email)
	require.Equal(t, email, New(goodHeaderName, reg).LoggedInAs(r))
}
