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

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/scrap/go/scrap"
	"go.skia.org/infra/scrap/go/scrap/mocks"
)

// hash used by some tests.
const hash = "01234567890abcdef"

const scrapName = "@MyScrapName"

var errMyMockError = errors.New("my error returned from mock ScrapExchange")

// validScrapBody returns an io.Reader that contains a valid serialized
// scrap.ScrapBody.
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

// validScrapName returns an io.Reader that contains a valid serialized
// scrap.Name.
func validScrapName(t *testing.T) io.Reader {
	var b bytes.Buffer
	scrapName := scrap.Name{
		Hash: hash,
	}
	err := json.NewEncoder(&b).Encode(scrapName)
	require.NoError(t, err)
	return &b
}

// makeRequest makes the HTTP request to Api with the given scrapExchange.
func makeRequest(scrapExchange *mocks.ScrapExchange, w http.ResponseWriter, r *http.Request) {
	a := &Api{
		scrapExchange: scrapExchange,
	}

	// Make the request through a mux.Router so the URL paths get parsed and
	// routed correctly.
	router := mux.NewRouter()
	a.AddHandlers(router, AddProtectedEndpoints)

	router.ServeHTTP(w, r)
}

func testMethodAndPathReturnExpectedStatusCode(t *testing.T, method string, path string, expectedCode int) {
	a, err := New(nil)
	require.NoError(t, err)

	// Make the request through a mux.Router so the URL paths get parsed and
	// routed correctly.
	router := mux.NewRouter()
	a.AddHandlers(router, DoNotAddProtectedEndpoints)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path, nil)

	router.ServeHTTP(w, r)
	require.Equal(t, expectedCode, w.Code, "path: %s method: %s", path, method)

}

func TestAddHandlers_DoNotEnableProtectedEndpoints_ProtectedEndpointsReturn4xxStatusCodes(t *testing.T) {
	unittest.SmallTest(t)
	testMethodAndPathReturnExpectedStatusCode(t, "POST", "/_/scraps/", http.StatusNotFound)
	testMethodAndPathReturnExpectedStatusCode(t, "DELETE", fmt.Sprintf("/_/scraps/svg/%s", hash), http.StatusMethodNotAllowed)
	testMethodAndPathReturnExpectedStatusCode(t, "PUT", fmt.Sprintf("/_/names/svg/%s", scrapName), http.StatusMethodNotAllowed)
	testMethodAndPathReturnExpectedStatusCode(t, "DELETE", fmt.Sprintf("/_/names/svg/%s", scrapName), http.StatusMethodNotAllowed)
}

func TestWriteJSON_InvalidJSON_ReportsError(t *testing.T) {
	unittest.SmallTest(t)
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

func TestScrapCreateHandler_InvalidJSON_ReturnsBadRequest(t *testing.T) {
	unittest.SmallTest(t)
	w := httptest.NewRecorder()
	b := bytes.NewBufferString("]This is not valid json[")
	r := httptest.NewRequest("POST", "/_/scraps/", b)

	makeRequest(nil, w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestScrapCreateHandler_WriteSucceeds_Success(t *testing.T) {
	unittest.SmallTest(t)
	callMetric := metrics2.GetCounter(scrapsCreateCallMetric)
	callMetric.Reset()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/_/scraps/", validScrapBody(t))

	scrapExchange := &mocks.ScrapExchange{}
	scrapID := scrap.ScrapID{
		Hash: hash,
	}
	scrapExchange.On("CreateScrap", testutils.AnyContext, mock.AnythingOfType("scrap.ScrapBody")).Return(scrapID, nil)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, fmt.Sprintf("{\"Hash\":\"%s\"}\n", hash), w.Body.String())
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.Equal(t, int64(1), callMetric.Get())
}

func TestScrapCreateHandler_WriteFails_ReturnsInternalServerError(t *testing.T) {
	unittest.SmallTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/_/scraps/", validScrapBody(t))

	scrapExchange := &mocks.ScrapExchange{}
	scrapExchange.On("CreateScrap", testutils.AnyContext, mock.AnythingOfType("scrap.ScrapBody")).Return(scrap.ScrapID{}, errMyMockError)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestScrapGetHandler_UnknownType_ReturnsBadRequest(t *testing.T) {
	unittest.SmallTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", fmt.Sprintf("/_/scraps/unknowntype/%s", hash), nil)

	makeRequest(nil, w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, "Unknown type.\n", w.Body.String())
}

func TestScrapGetHandler_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	callMetric := metrics2.GetCounter(scrapsGetCallMetric)
	callMetric.Reset()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", fmt.Sprintf("/_/scraps/svg/%s", hash), nil)

	scrapExchange := &mocks.ScrapExchange{}
	scrapBody := scrap.ScrapBody{
		Type: scrap.SVG,
		Body: "<svg></svg>",
	}
	scrapExchange.On("LoadScrap", testutils.AnyContext, scrap.SVG, hash).Return(scrapBody, nil)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.Equal(t, int64(1), callMetric.Get())
}

func TestScrapGetHandler_LoadFails_ReturnsBadRequest(t *testing.T) {
	unittest.SmallTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", fmt.Sprintf("/_/scraps/svg/%s", hash), nil)

	scrapExchange := &mocks.ScrapExchange{}
	scrapExchange.On("LoadScrap", testutils.AnyContext, scrap.SVG, hash).Return(scrap.ScrapBody{}, errMyMockError)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, "Failed to load scrap.\n", w.Body.String())
}

func TestScrapDeleteHandler_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	callMetric := metrics2.GetCounter(scrapsDeleteCallMetric)
	callMetric.Reset()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", fmt.Sprintf("/_/scraps/svg/%s", hash), nil)

	scrapExchange := &mocks.ScrapExchange{}
	scrapExchange.On("DeleteScrap", testutils.AnyContext, scrap.SVG, hash).Return(nil)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(1), callMetric.Get())
}

func TestScrapDeleteHandler_DeleteFails_ReturnsBadRequest(t *testing.T) {
	unittest.SmallTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", fmt.Sprintf("/_/scraps/svg/%s", hash), nil)

	scrapExchange := &mocks.ScrapExchange{}
	scrapExchange.On("DeleteScrap", testutils.AnyContext, scrap.SVG, hash).Return(errMyMockError)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, "Failed to delete scrap.\n", w.Body.String())
}

func TestRawGetHandler_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	callMetric := metrics2.GetCounter(rawGetCallMetric)
	callMetric.Reset()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", fmt.Sprintf("/_/raw/svg/%s", hash), nil)

	scrapExchange := &mocks.ScrapExchange{}
	scrapBody := scrap.ScrapBody{
		Type: scrap.SVG,
		Body: "<svg></svg>",
	}
	scrapExchange.On("LoadScrap", testutils.AnyContext, scrap.SVG, hash).Return(scrapBody, nil)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, scrapBody.Body, w.Body.String())
	require.Equal(t, "image/svg+xml", w.Header().Get("Content-Type"))
	require.Equal(t, int64(1), callMetric.Get())
}

func TestRawGetHandler_LoadFails_ReturnsBadRequest(t *testing.T) {
	unittest.SmallTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", fmt.Sprintf("/_/raw/svg/%s", hash), nil)

	scrapExchange := &mocks.ScrapExchange{}
	scrapExchange.On("LoadScrap", testutils.AnyContext, scrap.SVG, hash).Return(scrap.ScrapBody{}, errMyMockError)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, "Failed to load scrap.\n", w.Body.String())
}

func TestTemplateGetHandler_LoadFails_ReturnsBadRequest(t *testing.T) {
	unittest.SmallTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", fmt.Sprintf("/_/tmpl/svg/%s/js", hash), nil)

	scrapExchange := &mocks.ScrapExchange{}
	scrapExchange.On("Expand", testutils.AnyContext, scrap.SVG, hash, scrap.JS, w).Return(errMyMockError)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, "Failed to expand scrap.\n", w.Body.String())
}

func TestTemplateGetHandler_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	callMetric := metrics2.GetCounter(templateGetCallMetric)
	callMetric.Reset()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", fmt.Sprintf("/_/tmpl/svg/%s/js", hash), nil)

	const code = "// This is JS code!"
	scrapExchange := &mocks.ScrapExchange{}
	scrapExchange.On("Expand", testutils.AnyContext, scrap.SVG, hash, scrap.JS, w).Run(func(args mock.Arguments) {
		_, err := w.Write([]byte(code))
		assert.NoError(t, err)
	}).Return(nil)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "text/plain", w.Header().Get("Content-Type"))
	require.Equal(t, code, w.Body.String())
	require.Equal(t, int64(1), callMetric.Get())
}

func TestTemplateGetHandler_InvalidLang_ReturnsBadRequest(t *testing.T) {
	unittest.SmallTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", fmt.Sprintf("/_/tmpl/svg/%s/unknownlanguage", hash), nil)

	scrapExchange := &mocks.ScrapExchange{}

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, "Unknown language.\n", w.Body.String())
}

func TestNamePutHandler_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	callMetric := metrics2.GetCounter(namesPutCallMetric)
	callMetric.Reset()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", fmt.Sprintf("/_/names/svg/%s", scrapName), validScrapName(t))

	scrapExchange := &mocks.ScrapExchange{}
	scrapExchange.On("PutName", testutils.AnyContext, scrap.SVG, scrapName, mock.AnythingOfType("scrap.Name")).Return(nil)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(1), callMetric.Get())
}

func TestNamePutHandler_PutFails_ReturnsInternalServerError(t *testing.T) {
	unittest.SmallTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", fmt.Sprintf("/_/names/svg/%s", scrapName), validScrapName(t))

	scrapExchange := &mocks.ScrapExchange{}
	scrapExchange.On("PutName", testutils.AnyContext, scrap.SVG, scrapName, mock.AnythingOfType("scrap.Name")).Return(errMyMockError)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, "Failed to write name.\n", w.Body.String())
}

func TestNameGetHandler_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	callMetric := metrics2.GetCounter(namesGetCallMetric)
	callMetric.Reset()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", fmt.Sprintf("/_/names/svg/%s", scrapName), nil)

	scrapExchange := &mocks.ScrapExchange{}
	scrapBody := scrap.Name{
		Hash:        hash,
		Description: "A description of the scrap.",
	}
	scrapExchange.On("GetName", testutils.AnyContext, scrap.SVG, scrapName).Return(scrapBody, nil)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, fmt.Sprintf("{\"Hash\":\"%s\",\"Description\":\"A description of the scrap.\"}\n", hash), w.Body.String())
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.Equal(t, int64(1), callMetric.Get())
}

func TestNameGetHandler_GetFails_ReturnsInternalServerError(t *testing.T) {
	unittest.SmallTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", fmt.Sprintf("/_/names/svg/%s", scrapName), nil)

	scrapExchange := &mocks.ScrapExchange{}
	scrapExchange.On("GetName", testutils.AnyContext, scrap.SVG, scrapName).Return(scrap.Name{}, errMyMockError)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, "Failed to retrieve Name.\n", w.Body.String())
}

func TestNameDeleteHandler_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	callMetric := metrics2.GetCounter(namesDeleteCallMetric)
	callMetric.Reset()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", fmt.Sprintf("/_/names/svg/%s", scrapName), nil)

	scrapExchange := &mocks.ScrapExchange{}
	scrapExchange.On("DeleteName", testutils.AnyContext, scrap.SVG, scrapName).Return(nil)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(1), callMetric.Get())
}

func TestNameDeleteHandler_DeleteNameFails_ReturnsInternalServerError(t *testing.T) {
	unittest.SmallTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", fmt.Sprintf("/_/names/svg/%s", scrapName), nil)

	scrapExchange := &mocks.ScrapExchange{}
	scrapExchange.On("DeleteName", testutils.AnyContext, scrap.SVG, scrapName).Return(errMyMockError)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, "Failed to delete Name.\n", w.Body.String())
}

func TestNameListHandler_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	callMetric := metrics2.GetCounter(namesListCallMetric)
	callMetric.Reset()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/_/names/svg/", nil)

	scrapExchange := &mocks.ScrapExchange{}
	body := []string{
		scrapName,
		"AnotherScrap",
	}
	scrapExchange.On("ListNames", testutils.AnyContext, scrap.SVG).Return(body, nil)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	expected, err := json.Marshal(body)
	require.NoError(t, err)
	require.Equal(t, string(expected)+"\n", w.Body.String())
	require.Equal(t, int64(1), callMetric.Get())
}

func TestNameListHandler_ListFails_ReturnsInternalServerError(t *testing.T) {
	unittest.SmallTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/_/names/svg/", nil)

	scrapExchange := &mocks.ScrapExchange{}
	scrapExchange.On("ListNames", testutils.AnyContext, scrap.SVG).Return(nil, errMyMockError)

	makeRequest(scrapExchange, w, r)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, "Failed to load Names.\n", w.Body.String())
}
