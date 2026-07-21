package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/config"
)

func TestQueryApi_InitPageHandler_Success(t *testing.T) {
	refresher := &mockPsRefresher{}
	ps := paramtools.ReadOnlyParamSet{
		"arch": []string{"x86", "arm"},
	}
	refresher.On("GetAll").Return(ps)

	api := NewQueryApi(refresher)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/_/initpage", nil)
	api.initpageHandler(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestQueryApi_NextParamListHandler_Success(t *testing.T) {
	config.Config = &config.InstanceConfig{
		QueryConfig: config.QueryConfig{
			IncludedParams: []string{"benchmark", "bot", "measurement"},
		},
	}
	refresher := &mockPsRefresher{}
	q, err := query.NewFromString("benchmark=speed")
	require.NoError(t, err)
	u, err := url.ParseQuery("benchmark=speed")
	require.NoError(t, err)

	refresher.On("GetAll").Return(paramtools.ReadOnlyParamSet{})
	refresher.On("GetParamSetForQuery", mock.Anything, q, u).Return(
		int64(5),
		paramtools.ParamSet{
			"bot": []string{"linux", "mac"},
		},
		nil,
	)

	api := NewQueryApi(refresher)
	reqBody, err := json.Marshal(NextParamListHandlerRequest{
		Query: "benchmark=speed",
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/_/nextParamList", bytes.NewReader(reqBody))
	api.nextParamListHandler(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp NextParamListHandlerResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 5, resp.Count)
	assert.Equal(t, paramtools.ReadOnlyParamSet{"bot": {"linux", "mac"}}, resp.Paramset)
}

func TestQueryApi_NextParamListHandler_InvalidJSON_ReturnsInternalServerError(t *testing.T) {
	api := NewQueryApi(&mockPsRefresher{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/_/nextParamList", bytes.NewReader([]byte("invalid json")))
	api.nextParamListHandler(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to decode JSON.")
}

func TestQueryApi_NextParamListHandler_PreflightError_ReturnsInternalServerError(t *testing.T) {
	config.Config = &config.InstanceConfig{
		QueryConfig: config.QueryConfig{
			IncludedParams: []string{"benchmark", "bot"},
		},
	}
	refresher := &mockPsRefresher{}
	q, err := query.NewFromString("benchmark=speed")
	require.NoError(t, err)
	u, err := url.ParseQuery("benchmark=speed")
	require.NoError(t, err)

	refresher.On("GetAll").Return(paramtools.ReadOnlyParamSet{})
	refresher.On("GetParamSetForQuery", mock.Anything, q, u).Return(
		int64(0),
		paramtools.ParamSet(nil),
		errors.New("EOF"),
	)

	api := NewQueryApi(refresher)
	reqBody, err := json.Marshal(NextParamListHandlerRequest{
		Query: "benchmark=speed",
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/_/nextParamList", bytes.NewReader(reqBody))
	api.nextParamListHandler(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to Preflight the query.")
}

func TestQueryApi_CountHandler_InvalidJSON_ReturnsInternalServerError(t *testing.T) {
	api := NewQueryApi(&mockPsRefresher{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/_/count", bytes.NewReader([]byte("invalid json")))
	api.countHandler(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to decode JSON.")
}

func TestQueryApi_CountHandler_PreflightError_ReturnsInternalServerError(t *testing.T) {
	refresher := &mockPsRefresher{}
	q, err := query.NewFromString("benchmark=speed")
	require.NoError(t, err)
	u, err := url.ParseQuery("benchmark=speed")
	require.NoError(t, err)

	refresher.On("GetAll").Return(paramtools.ReadOnlyParamSet{})
	refresher.On("GetParamSetForQuery", mock.Anything, q, u).Return(
		int64(0),
		paramtools.ParamSet(nil),
		errors.New("timeout: context canceled"),
	)

	api := NewQueryApi(refresher)
	reqBody, err := json.Marshal(CountHandlerRequest{
		Q: "benchmark=speed",
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/_/count", bytes.NewReader(reqBody))
	api.countHandler(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to Preflight the query.")
}
