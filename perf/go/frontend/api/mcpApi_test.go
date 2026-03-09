package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	dfmock "go.skia.org/infra/perf/go/dataframe/mocks"
	mdmock "go.skia.org/infra/perf/go/tracestore/mocks"
	"go.skia.org/infra/perf/go/types"
)

func TestGetTraceDataHandler_Success(t *testing.T) {
	dfb := &dfmock.DataFrameBuilder{}
	mdb := &mdmock.MetadataStore{}
	api := NewMcpApi(dfb, mdb)

	// Mock the DataFrameBuilder response.
	expectedDF := &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			",arch=x86,config=8888,": {1.0, 2.0, 3.0},
		},
		Header: []*dataframe.ColumnHeader{
			{Offset: 1, Timestamp: 1672531200},
			{Offset: 2, Timestamp: 1672531260},
			{Offset: 3, Timestamp: 1672531320},
		},
	}
	dfb.On("NewFromQueryAndRange", testutils.AnyContext, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(expectedDF, nil)

	// Create the request.
	req := httptest.NewRequest("GET", "/mcp/data?query=benchmark%3Dspeedometer%26test%3DTotal&begin=1672531200&end=1675468800", nil)
	w := httptest.NewRecorder()

	// Create a router to serve the handler.
	router := chi.NewRouter()
	api.RegisterHandlers(router)
	router.ServeHTTP(w, req)

	// Check the response.
	require.Equal(t, http.StatusOK, w.Code)
	var actualDF dataframe.DataFrame
	err := json.Unmarshal(w.Body.Bytes(), &actualDF)
	require.NoError(t, err)
	require.Equal(t, expectedDF, &actualDF)

	dfb.AssertExpectations(t)
}

func TestGetTraceDataHandler_WithMetadata(t *testing.T) {
	dfb := &dfmock.DataFrameBuilder{}
	mdb := &mdmock.MetadataStore{}
	api := NewMcpApi(dfb, mdb)

	traceID := ",arch=x86,config=8888,"
	sourceInfo := types.NewTraceSourceInfo()
	sourceInfo.Add(1, 101)
	sourceInfo.Add(2, 102)

	// Mock the DataFrameBuilder response.
	expectedDF := &dataframe.DataFrame{
		TraceSet: types.TraceSet{
			traceID: {1.0, 2.0, 3.0},
		},
		Header: []*dataframe.ColumnHeader{
			{Offset: 1, Timestamp: 1672531200},
			{Offset: 2, Timestamp: 1672531260},
			{Offset: 3, Timestamp: 1672531320},
		},
		SourceInfo: map[string]*types.TraceSourceInfo{
			traceID: sourceInfo,
		},
	}
	dfb.On("NewFromQueryAndRange", testutils.AnyContext, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(expectedDF, nil)

	// Mock MetadataStore
	links101 := map[string]string{"log": "http://log/101"}
	links102 := map[string]string{"log": "http://log/102"}
	mdb.On("GetMetadataForSourceFileIDs", testutils.AnyContext, []int64{101, 102}).Return(map[int64]map[string]string{
		101: links101,
		102: links102,
	}, nil)

	config.Config = &config.InstanceConfig{
		DataPointConfig: config.DataPointConfig{
			KeysForUsefulLinks: []string{"log"},
		},
	}

	// Create the request.
	req := httptest.NewRequest("GET", "/mcp/data?query=benchmark%3Dspeedometer&begin=1672531200&end=1675468800&metadata=true", nil)
	w := httptest.NewRecorder()

	// Create a router to serve the handler.
	router := chi.NewRouter()
	api.RegisterHandlers(router)
	router.ServeHTTP(w, req)

	// Check the response.
	require.Equal(t, http.StatusOK, w.Code)
	var actualDF dataframe.DataFrame
	err := json.Unmarshal(w.Body.Bytes(), &actualDF)
	require.NoError(t, err)

	require.Len(t, actualDF.TraceMetadata, 1)
	require.Equal(t, traceID, actualDF.TraceMetadata[0].TraceID)
	require.Len(t, actualDF.TraceMetadata[0].CommitLinks, 2)
	require.Equal(t, "http://log/101", actualDF.TraceMetadata[0].CommitLinks[1]["log"].Href)
	require.Equal(t, "http://log/102", actualDF.TraceMetadata[0].CommitLinks[2]["log"].Href)

	dfb.AssertExpectations(t)
	mdb.AssertExpectations(t)
}

func TestGetTraceDataHandler_MissingParams(t *testing.T) {
	dfb := &dfmock.DataFrameBuilder{}
	mdb := &mdmock.MetadataStore{}
	api := NewMcpApi(dfb, mdb)
	router := chi.NewRouter()
	api.RegisterHandlers(router)

	testCases := []struct {
		name    string
		url     string
		message string
	}{
		{
			name:    "missing query",
			url:     "/mcp/data?begin=1672531200&end=1675468800",
			message: "query, begin, and end are required",
		},
		{
			name:    "missing begin",
			url:     "/mcp/data?query=benchmark%3Dspeedometer&end=1675468800",
			message: "query, begin, and end are required",
		},
		{
			name:    "missing end",
			url:     "/mcp/data?query=benchmark%3Dspeedometer&begin=1672531200",
			message: "query, begin, and end are required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.url, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), tc.message)
		})
	}
}

func TestGetTraceDataHandler_InvalidParams(t *testing.T) {
	dfb := &dfmock.DataFrameBuilder{}
	mdb := &mdmock.MetadataStore{}
	api := NewMcpApi(dfb, mdb)
	router := chi.NewRouter()
	api.RegisterHandlers(router)

	testCases := []struct {
		name    string
		url     string
		message string
	}{
		{
			name:    "invalid begin timestamp",
			url:     "/mcp/data?query=a%3Db&begin=not-a-number&end=1675468800",
			message: "invalid 'begin' timestamp",
		},
		{
			name:    "invalid end timestamp",
			url:     "/mcp/data?query=a%3Db&begin=1672531200&end=not-a-number",
			message: "invalid 'end' timestamp",
		},
		{
			name:    "invalid query",
			url:     "/mcp/data?query=invalid&begin=1672531200&end=1675468800",
			message: "invalid query",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.url, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), tc.message)
		})
	}
}

func TestGetTraceDataHandler_DataFrameBuilderError(t *testing.T) {
	dfb := &dfmock.DataFrameBuilder{}
	mdb := &mdmock.MetadataStore{}
	api := NewMcpApi(dfb, mdb)

	// Mock the DataFrameBuilder to return an error.
	expectedErr := errors.New("something went wrong")
	dfb.On("NewFromQueryAndRange", testutils.AnyContext, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, expectedErr)

	// Create the request.
	req := httptest.NewRequest("GET", "/mcp/data?query=benchmark%3Dspeedometer&begin=1672531200&end=1675468800", nil)
	w := httptest.NewRecorder()

	// Create a router to serve the handler.
	router := chi.NewRouter()
	api.RegisterHandlers(router)
	router.ServeHTTP(w, req)

	// Check the response.
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "Failed to build dataframe.")

	dfb.AssertExpectations(t)
}
