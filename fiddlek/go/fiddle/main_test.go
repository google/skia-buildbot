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

const (
	code = "void draw(SkCanvas* canvas) {}"
	hash = "1234567890"
)

var (
	errMyMockError = fmt.Errorf("my mock error")
)

func TestScrapHandler_HappyPath(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("GET", "/scrap/svg/@smiley", nil)
	w := httptest.NewRecorder()

	// Create a test server that mocks out the scrap exchange.
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprint(w, code)
		require.NoError(t, err)
	}))
	defer func() { testServer.Close() }()
	u, err := url.Parse(testServer.URL)
	require.NoError(t, err)

	// Configure main to point at the test server.
	*scrapExchange = u.Host
	httpClient = testServer.Client()

	// Mock the fiddle store.
	store := &mocks.Store{}
	store.On("Put", code, defaultFiddle.Options, (*types.Result)(nil)).Return(hash, nil)
	defer store.AssertExpectations(t)
	fiddleStore = store

	scrapHandler(w, r)

	// Should redirect to the newly created fiddle.
	require.Equal(t, 307, w.Code)
	require.Equal(t, fmt.Sprintf("/c/%s", hash), w.Header().Get("Location"))
}

func TestScrapHandler_ScrapExchangeFails_ReturnsInternalServerError(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("GET", "/scrap/svg/@smiley", nil)
	w := httptest.NewRecorder()

	// Create a test server that mocks out the scrap exchange.
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Failure", http.StatusInternalServerError)
	}))
	defer func() { testServer.Close() }()
	u, err := url.Parse(testServer.URL)
	require.NoError(t, err)

	// Configure main to point at the test server.
	*scrapExchange = u.Host
	httpClient = testServer.Client()

	scrapHandler(w, r)

	require.Equal(t, 500, w.Code)
	require.Contains(t, w.Body.String(), "Failed to load")
}

func TestScrapHandler_FiddleStorePutFails_ReturnsInternalServerError(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("GET", "/scrap/svg/@smiley", nil)
	w := httptest.NewRecorder()

	// Create a test server that mocks out the scrap exchange.
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprint(w, code)
		require.NoError(t, err)
	}))
	defer func() { testServer.Close() }()
	u, err := url.Parse(testServer.URL)
	require.NoError(t, err)

	// Configure main to point at the test server.
	*scrapExchange = u.Host
	httpClient = testServer.Client()

	// Mock the fiddle store.
	store := &mocks.Store{}
	store.On("Put", code, defaultFiddle.Options, (*types.Result)(nil)).Return(hash, errMyMockError)
	defer store.AssertExpectations(t)
	fiddleStore = store

	scrapHandler(w, r)

	require.Equal(t, 500, w.Code)
	require.Contains(t, w.Body.String(), "Failed to write")
}
