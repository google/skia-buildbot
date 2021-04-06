// Package sklogin implmements //go/alogin.Login using the //go/login package.
package sklogin

import (
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestRegisterHandlers_HandlersAreRegistered(t *testing.T) {
	unittest.SmallTest(t)

	router := mux.NewRouter()
	(&sklogin{}).RegisterHandlers(router)
	var out mux.RouteMatch
	r := httptest.NewRequest("GET", "/logout/", nil)
	require.True(t, router.Match(r, &out))
}
