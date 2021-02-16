package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestProxyServeHTTP_AllowPostNotAuthenticated_WebAuthHeaderValueIsEmptyString(t *testing.T) {
	unittest.SmallTest(t)
	*allowPost = true

	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Note that if the header webAuthHeaderName hadn't been set then the value would be nil.
		require.Equal(t, []string{""}, r.Header.Values(webAuthHeaderName))
		require.Equal(t, []string(nil), r.Header.Values("X-SOME-UNSET-HEADER"))
		called = true
	}))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	proxy := newProxy(u)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", ts.URL, nil)
	proxy.ServeHTTP(w, r)
	require.True(t, called)
}
