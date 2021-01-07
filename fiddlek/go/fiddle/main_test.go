// fiddle is the web server for fiddle.
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/fiddlek/go/store/mocks"
	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestScrapHandler_HappyPath(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("GET", "/scrap/svg/@smiley", nil)
	w := httptest.NewRecorder()

	const code = "void draw(SkCanvas* canvas) {}"
	const hash = "1234567890"

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, code)
	}))
	defer func() { testServer.Close() }()

	httpClient = testServer.Client()
	u, err := url.Parse(testServer.URL)
	require.NoError(t, err)
	*scrapExchange = u.Host

	store := &mocks.Store{}
	store.On("Put", code, defaultFiddle.Options, (*types.Result)(nil)).Return(hash, nil)

	fiddleStore = store

	scrapHandler(w, r)

	require.Equal(t, 307, w.Code)
	require.Equal(t, fmt.Sprintf("/c/%s", hash), w.Header().Get("Location"))
}
