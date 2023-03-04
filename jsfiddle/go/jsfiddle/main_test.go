package main

import (
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/jsfiddle/go/store/mocks"
	"go.skia.org/infra/scrap/go/scrap"
	scrapMocks "go.skia.org/infra/scrap/go/scrap/mocks"
)

const (
	code = "void draw(SkCanvas* canvas) {}"
	hash = "1234567890"
)

var (
	errMyMockError = fmt.Errorf("my mock error")
)

func TestScrapHandler_ValidScrap_RedirectsToNewlyCreatedJSFiddle(t *testing.T) {

	r := httptest.NewRequest("GET", "/scrap/sksl/@smiley", nil)
	w := httptest.NewRecorder()

	// Mock the scrapClient.
	scrapClientMock := &scrapMocks.ScrapExchange{}
	scrapClientMock.On("Expand", testutils.AnyContext, scrap.SKSL, "@smiley", scrap.JS, mock.Anything).Run(func(args mock.Arguments) {
		_, err := args.Get(4).(io.Writer).Write([]byte(code))
		require.NoError(t, err)
	}).Return(nil)
	defer scrapClientMock.AssertExpectations(t)
	scrapClient = scrapClientMock

	// Mock the jsfiddle store.
	store := &mocks.Store{}
	store.On("PutCode", code, "canvaskit").Return(hash, nil)
	defer store.AssertExpectations(t)
	fiddleStore = store

	// Make the request through a mux.Router so the URL paths get parsed and
	// routed correctly.
	router := mux.NewRouter()
	addHandlers(router)

	router.ServeHTTP(w, r)

	// Should redirect to the newly created jsfiddle.
	require.Equal(t, 307, w.Code)
	require.Equal(t, fmt.Sprintf("/canvaskit/%s", hash), w.Header().Get("Location"))
}

func TestScrapHandler_ScrapExchangeFails_ReturnsInternalServerError(t *testing.T) {

	r := httptest.NewRequest("GET", "/scrap/sksl/@smiley", nil)
	w := httptest.NewRecorder()

	// Mock the scrapClient.
	scrapClientMock := &scrapMocks.ScrapExchange{}
	scrapClientMock.On("Expand", testutils.AnyContext, scrap.SKSL, "@smiley", scrap.JS, mock.Anything).Return(errMyMockError)
	defer scrapClientMock.AssertExpectations(t)
	scrapClient = scrapClientMock

	// Make the request through a mux.Router so the URL paths get parsed and
	// routed correctly.
	router := mux.NewRouter()
	addHandlers(router)

	router.ServeHTTP(w, r)

	require.Equal(t, 500, w.Code)
}

func TestScrapHandler_FiddleStorePutCodeFails_ReturnsInternalServerError(t *testing.T) {

	r := httptest.NewRequest("GET", "/scrap/sksl/@smiley", nil)
	w := httptest.NewRecorder()

	// Mock the scrapClient.
	scrapClientMock := &scrapMocks.ScrapExchange{}
	scrapClientMock.On("Expand", testutils.AnyContext, scrap.SKSL, "@smiley", scrap.JS, mock.Anything).Run(func(args mock.Arguments) {
		_, err := args[4].(io.Writer).Write([]byte(code))
		require.NoError(t, err)
	}).Return(nil)
	defer scrapClientMock.AssertExpectations(t)
	scrapClient = scrapClientMock

	// Mock the jsfiddle store.
	store := &mocks.Store{}
	store.On("PutCode", code, "canvaskit").Return("", errMyMockError)
	defer store.AssertExpectations(t)
	fiddleStore = store

	// Make the request through a mux.Router so the URL paths get parsed and
	// routed correctly.
	router := mux.NewRouter()
	addHandlers(router)

	router.ServeHTTP(w, r)

	require.Equal(t, 500, w.Code)
}

func TestScrapHandler_WithInvalidType_ReturnsBadRequest(t *testing.T) {

	r := httptest.NewRequest("GET", "/scrap/NotAType/@smiley", nil)
	w := httptest.NewRecorder()

	// Make the request through a mux.Router so the URL paths get parsed and
	// routed correctly.
	router := mux.NewRouter()
	addHandlers(router)

	router.ServeHTTP(w, r)

	require.Equal(t, 404, w.Code)
}
