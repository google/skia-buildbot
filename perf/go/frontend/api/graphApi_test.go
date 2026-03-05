package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
	alogin_mock "go.skia.org/infra/go/alogin/mocks"
	anomalyMock "go.skia.org/infra/perf/go/anomalies/mock"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	df_mock "go.skia.org/infra/perf/go/dataframe/mocks"
	git_mock "go.skia.org/infra/perf/go/git/mocks"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/graphsshortcut"
	gs_mock "go.skia.org/infra/perf/go/graphsshortcut/mocks"
	shortcut_mock "go.skia.org/infra/perf/go/shortcut/mocks"
	ts_mock "go.skia.org/infra/perf/go/tracestore/mocks"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

func TestFrontendDetailsHandler_InvalidTraceID_ReturnsErrorMessage(t *testing.T) {
	api := graphApi{}
	w := httptest.NewRecorder()

	req := CommitDetailsRequest{
		CommitNumber: 0,
		TraceID:      `calc("this is not a trace id, but a calculation")`,
	}
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(req)
	require.NoError(t, err)

	r := httptest.NewRequest("POST", "/_/details", &b)
	api.detailsHandler(w, r)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.Contains(t, w.Body.String(), "version\":0")
}

func TestGetGraphsShortcutDataHandler_Success(t *testing.T) {
	config.Config = &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			CommitNumberRegex: ".*",
		},
	}
	gsStore := gs_mock.NewStore(t)
	perfGit := git_mock.NewGit(t)
	dfBuilder := df_mock.NewDataFrameBuilder(t)
	traceStore := ts_mock.NewTraceStore(t)
	metadataStore := ts_mock.NewMetadataStore(t)
	shortcutStore := shortcut_mock.NewStore(t)
	anomalyStore := anomalyMock.NewStore(t)
	loginProvider := alogin_mock.NewLogin(t)

	api := NewGraphApi(2, 100, 10, loginProvider, dfBuilder, perfGit, traceStore, metadataStore, nil, shortcutStore, gsStore, anomalyStore, nil, nil)

	shortcutID := "test-shortcut-id"
	begin := 1000
	end := 2000
	requestURL := fmt.Sprintf("/_/shortcut/graphs?id=%s&begin=%d&end=%d&request_type=%d&include_metadata=true", shortcutID, begin, end, frame.REQUEST_TIME_RANGE)

	sc := &graphsshortcut.GraphsShortcut{
		Graphs: []graphsshortcut.GraphConfig{
			{
				Queries: []string{"name=query1"},
			},
		},
	}
	gsStore.On("GetShortcut", mock.Anything, shortcutID).Return(sc, nil)
	loginProvider.On("LoggedInAs", mock.Anything).Return(alogin.EMail("nobody@example.org"))

	sourceInfo := types.NewTraceSourceInfo()
	sourceInfo.Add(1, 123)
	expectedDF := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 1, Timestamp: 1500},
		},
		TraceSet: types.TraceSet{
			"trace1": {1.0},
		},
		ParamSet:   map[string][]string{},
		SourceInfo: map[string]*types.TraceSourceInfo{"trace1": sourceInfo},
	}
	dfBuilder.On("NewFromQueryAndRange", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(expectedDF, nil)
	metadataStore.On("GetMetadataForSourceFileIDs", mock.Anything, mock.Anything).Return(map[int64]map[string]string{123: {"link": "http://foo"}}, nil)
	anomalyStore.On("GetAnomalies", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(chromeperf.AnomalyMap{}, nil)

	req := httptest.NewRequest("GET", requestURL, nil)
	w := httptest.NewRecorder()

	api.getGraphsShortcutDataHandler(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp GetGraphsShortcutDataResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.Len(t, resp.Graphs, 1)
	require.NotNil(t, resp.Graphs[0])
	require.Equal(t, expectedDF.TraceSet, resp.Graphs[0].DataFrame.TraceSet)
}

func TestGetGraphsShortcutDataHandler_DefaultTiles(t *testing.T) {
	config.Config = &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			CommitNumberRegex: ".*",
		},
	}
	gsStore := gs_mock.NewStore(t)
	perfGit := git_mock.NewGit(t)
	dfBuilder := df_mock.NewDataFrameBuilder(t)
	traceStore := ts_mock.NewTraceStore(t)
	metadataStore := ts_mock.NewMetadataStore(t)
	shortcutStore := shortcut_mock.NewStore(t)
	anomalyStore := anomalyMock.NewStore(t)
	loginProvider := alogin_mock.NewLogin(t)

	api := NewGraphApi(2, 100, 10, loginProvider, dfBuilder, perfGit, traceStore, metadataStore, nil, shortcutStore, gsStore, anomalyStore, nil, nil)

	shortcutID := "test-shortcut-id"
	requestURL := fmt.Sprintf("/_/shortcut/graphs?id=%s&request_type=%d", shortcutID, frame.REQUEST_TIME_RANGE)

	sc := &graphsshortcut.GraphsShortcut{
		Graphs: []graphsshortcut.GraphConfig{
			{
				Queries: []string{"name=query1"},
			},
		},
	}
	gsStore.On("GetShortcut", mock.Anything, shortcutID).Return(sc, nil)
	loginProvider.On("LoggedInAs", mock.Anything).Return(alogin.EMail("nobody@example.org"))

	traceStore.On("GetLatestTile", mock.Anything).Return(types.TileNumber(10), nil)
	traceStore.On("TileSize").Return(int32(256))
	traceStore.On("CommitNumberOfTileStart", mock.Anything).Return(types.CommitNumber(256 * 9))
	perfGit.On("CommitFromCommitNumber", mock.Anything, mock.Anything).Return(provider.Commit{Timestamp: 5000}, nil)

	expectedDF := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 1, Timestamp: 5500},
		},
		TraceSet: types.TraceSet{
			"trace1": {1.0},
		},
		ParamSet:   map[string][]string{},
		SourceInfo: map[string]*types.TraceSourceInfo{},
	}
	dfBuilder.On("NewFromQueryAndRange", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(expectedDF, nil)
	anomalyStore.On("GetAnomalies", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(chromeperf.AnomalyMap{}, nil)

	req := httptest.NewRequest("GET", requestURL, nil)
	w := httptest.NewRecorder()

	api.getGraphsShortcutDataHandler(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestGetGraphsShortcutDataHandler_NoMetadata(t *testing.T) {
	config.Config = &config.InstanceConfig{
		GitRepoConfig: config.GitRepoConfig{
			CommitNumberRegex: ".*",
		},
	}
	gsStore := gs_mock.NewStore(t)
	perfGit := git_mock.NewGit(t)
	dfBuilder := df_mock.NewDataFrameBuilder(t)
	traceStore := ts_mock.NewTraceStore(t)
	metadataStore := ts_mock.NewMetadataStore(t)
	shortcutStore := shortcut_mock.NewStore(t)
	anomalyStore := anomalyMock.NewStore(t)
	loginProvider := alogin_mock.NewLogin(t)

	api := NewGraphApi(2, 100, 10, loginProvider, dfBuilder, perfGit, traceStore, metadataStore, nil, shortcutStore, gsStore, anomalyStore, nil, nil)

	shortcutID := "test-shortcut-id"
	begin := 1000
	end := 2000
	requestURL := fmt.Sprintf("/_/shortcut/graphs?id=%s&begin=%d&end=%d&request_type=%d&include_metadata=false", shortcutID, begin, end, frame.REQUEST_TIME_RANGE)

	sc := &graphsshortcut.GraphsShortcut{
		Graphs: []graphsshortcut.GraphConfig{
			{
				Queries: []string{"name=query1"},
			},
		},
	}
	gsStore.On("GetShortcut", mock.Anything, shortcutID).Return(sc, nil)
	loginProvider.On("LoggedInAs", mock.Anything).Return(alogin.EMail("nobody@example.org"))

	sourceInfo := types.NewTraceSourceInfo()
	sourceInfo.Add(1, 123)
	expectedDF := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 1, Timestamp: 1500},
		},
		TraceSet: types.TraceSet{
			"trace1": {1.0},
		},
		ParamSet:   map[string][]string{},
		SourceInfo: map[string]*types.TraceSourceInfo{"trace1": sourceInfo},
	}
	dfBuilder.On("NewFromQueryAndRange", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(expectedDF, nil)
	// metadataStore should NOT be called.
	anomalyStore.On("GetAnomalies", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(chromeperf.AnomalyMap{}, nil)

	req := httptest.NewRequest("GET", requestURL, nil)
	w := httptest.NewRecorder()

	api.getGraphsShortcutDataHandler(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	metadataStore.AssertNotCalled(t, "GetMetadataForSourceFileIDs", mock.Anything, mock.Anything)
}
