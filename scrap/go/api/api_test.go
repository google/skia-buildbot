// Package api implements the REST API for the scrap exchange service.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/scrap/go/scrap"
	"go.skia.org/infra/scrap/go/scrap/mocks"
)

// hash used by some tests.
const hash = "01234567890abcdef"

var errMyMockError = errors.New("my error returned from mock ScrapExchange")

func validScrapBody(t *testing.T) io.Reader {
	var b bytes.Buffer
	scrapBody := scrap.ScrapBody{
		Type: scrap.SVG,
		Body: "<svg></svg>",
	}
	err := json.NewEncoder(&b).Encode(scrapBody)
	require.NoError(t, err)
	return &b
}

func TestScrapCreateHandler_InvalidJSON_ReturnsBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	b := bytes.NewBufferString("]This is not valid json[")
	r := httptest.NewRequest("POST", "/_/scraps/", b)
	a := &Api{}

	a.scrapCreateHandler(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestScrapCreateHandler_WriteSucceeds_Success(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/_/scraps/", validScrapBody(t))

	scrapExchange := &mocks.ScrapExchange{}
	scrapID := scrap.ScrapID{
		Hash: hash,
	}
	scrapExchange.On("CreateScrap", testutils.AnyContext, mock.AnythingOfType("scrap.ScrapBody")).Return(scrapID, nil)

	a := &Api{
		scrapExchange: scrapExchange,
	}

	a.scrapCreateHandler(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, fmt.Sprintf("{\"Hash\":\"%s\"}\n", hash), w.Body.String())
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestScrapCreateHandler_WriteFails_ReturnsInternalServerError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/_/scraps/", validScrapBody(t))

	scrapExchange := &mocks.ScrapExchange{}
	scrapExchange.On("CreateScrap", testutils.AnyContext, mock.AnythingOfType("scrap.ScrapBody")).Return(scrap.ScrapID{}, errMyMockError)

	a := &Api{
		scrapExchange: scrapExchange,
	}

	a.scrapCreateHandler(w, r)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestWriteJSON_InvalidJSON_ReportsError(t *testing.T) {
	w := httptest.NewRecorder()
	notSerializable := struct {
		C complex128
	}{
		C: 12 + 3i,
	}
	writeJSON(w, notSerializable)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, "Failed to encode JSON response.\n", w.Body.String())
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
}
