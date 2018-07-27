package httputils

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
)

func TestForceHTTPS(t *testing.T) {
	testutils.SmallTest(t)
	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.WriteString(w, "Hello World!")
		assert.NoError(t, err)
	})
	// Test w/o ForceHTTPS in place.
	r := httptest.NewRequest("GET", "http://example.com/foo", nil)
	r.Header.Set(SCHEME_AT_LOAD_BALANCER_HEADER, "http")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, 200, w.Result().StatusCode)
	assert.Equal(t, "", w.Result().Header.Get("Location"))
	b, err := ioutil.ReadAll(w.Result().Body)
	assert.NoError(t, err)
	assert.Len(t, b, 12)

	// Add in ForceHTTPS behavior.
	h = ForceHTTPS(h)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, 301, w.Result().StatusCode)
	assert.Equal(t, "https://example.com/foo", w.Result().Header.Get("Location"))

	// Test the healthcheck handling.
	r = httptest.NewRequest("GET", "http://example.com/", nil)
	r.Header.Set("User-Agent", "GoogleHC/1.0")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, 200, w.Result().StatusCode)
	assert.Equal(t, "", w.Result().Header.Get("Location"))
	b, err = ioutil.ReadAll(w.Result().Body)
	assert.NoError(t, err)
	assert.Len(t, b, 0)
}
