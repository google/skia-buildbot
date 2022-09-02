package sklogin

import (
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
)

func TestRegisterHandlers_HandlersAreRegistered(t *testing.T) {

	router := mux.NewRouter()
	(&sklogin{}).RegisterHandlers(router)
	var out mux.RouteMatch
	r := httptest.NewRequest("GET", "/logout/", nil)
	require.True(t, router.Match(r, &out))
}

func TestStatus_CookiesAreNotPresent_EMailIsNotReturnedInStatus(t *testing.T) {

	r := httptest.NewRequest("GET", "/", nil)
	expected := alogin.Status{
		EMail:     alogin.NotLoggedIn,
		LoginURL:  loginPath,
		LogoutURL: logoutPath,
	}
	require.Equal(t, expected, (&sklogin{}).Status(r))
}
