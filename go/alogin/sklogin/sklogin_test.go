package sklogin

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
)

func TestStatus_CookiesAreNotPresent_EMailIsNotReturnedInStatus(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	expected := alogin.Status{
		EMail:     alogin.NotLoggedIn,
		LoginURL:  loginPath,
		LogoutURL: logoutPath,
	}
	require.Equal(t, expected, (&sklogin{}).Status(r))
}
