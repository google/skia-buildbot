package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func testLoginURL(t *testing.T, expected, domain string) {
	t.Helper()

	l := New()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Host = domain

	require.Equal(t, expected, l.LoginURL(w, r))

}

func TestAuthImpl_LoginURL(t *testing.T) {
	testLoginURL(t, "https://skia.org/login/", "foo.skia.org")
	testLoginURL(t, "https://luci.app/login/", "perf.luci.app")
}
