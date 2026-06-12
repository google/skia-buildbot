package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.skia.org/infra/perf/go/config"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	anomalyMocks "go.skia.org/infra/perf/go/anomalies/mock"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/dataframe"
	dataframeMocks "go.skia.org/infra/perf/go/dataframe/mocks"
	gitMocks "go.skia.org/infra/perf/go/git/mocks"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/types"
)

func TestTraceValuesHandler_Success(t *testing.T) {
	config.Config = &config.InstanceConfig{}
	w := httptest.NewRecorder()
	req := TraceValuesRequest{
		Ids:       []string{"trace1"},
		MinCommit: 100,
		MaxCommit: 200,
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/_/trace_values", bytes.NewReader(body))

	dfBuilder := dataframeMocks.NewDataFrameBuilder(t)
	perfGit := gitMocks.NewGit(t)

	api := NewTraceValuesApi(dfBuilder, perfGit, nil, nil)

	// Mock Git calls
	perfGit.On("CommitNumberFromTime", mock.Anything, time.Time{}).Return(types.CommitNumber(200), nil)
	perfGit.On("CommitFromCommitNumber", mock.MatchedBy(func(ctx context.Context) bool {
		_, ok := ctx.Deadline()
		return ok
	}), types.CommitNumber(100)).Return(provider.Commit{Timestamp: 1000}, nil)
	perfGit.On("CommitFromCommitNumber", mock.MatchedBy(func(ctx context.Context) bool {
		_, ok := ctx.Deadline()
		return ok
	}), types.CommitNumber(200)).Return(provider.Commit{Timestamp: 2000}, nil)

	// Mock DataFrameBuilder calls
	fakeDf := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 100, Timestamp: 1000},
			{Offset: 200, Timestamp: 2000},
		},
		TraceSet: types.TraceSet{
			"trace1": []float32{1.0, 2.0},
		},
	}
	dfBuilder.On("NewFromKeysAndRange", mock.MatchedBy(func(ctx context.Context) bool {
		_, ok := ctx.Deadline()
		return ok
	}), []string{"trace1"}, time.Unix(1000, 0), time.Unix(2000, 0), mock.Anything).Return(fakeDf, nil)

	api.traceValuesHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	var resp TraceValuesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	require.Len(t, resp.Results, 1)
	require.Len(t, resp.Results["trace1"], 2)
	require.Equal(t, int64(100), resp.Results["trace1"][0].CommitNumber)
	require.Equal(t, float32(1.0), resp.Results["trace1"][0].Val)
}

func TestTraceValuesHandler_WithTimestamps(t *testing.T) {
	w := httptest.NewRecorder()
	req := TraceValuesRequest{
		Ids:   []string{"trace1"},
		Begin: 1000,
		End:   2000,
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/_/trace_values", bytes.NewReader(body))

	dfBuilder := dataframeMocks.NewDataFrameBuilder(t)
	perfGit := gitMocks.NewGit(t)

	api := NewTraceValuesApi(dfBuilder, perfGit, nil, nil)

	fakeDf := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 100, Timestamp: 1000},
			{Offset: 200, Timestamp: 2000},
		},
		TraceSet: types.TraceSet{
			"trace1": []float32{1.0, 2.0},
		},
	}
	dfBuilder.On("NewFromKeysAndRange", mock.Anything, []string{"trace1"}, time.Unix(1000, 0), time.Unix(2000, 0), mock.Anything).Return(fakeDf, nil)

	api.traceValuesHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	var resp TraceValuesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	require.Len(t, resp.Results, 1)
	require.Len(t, resp.Results["trace1"], 2)
}

func TestTraceValuesHandler_WithAnomalies(t *testing.T) {
	config.Config = &config.InstanceConfig{
		FetchAnomaliesFromSql: true,
	}
	w := httptest.NewRecorder()
	req := TraceValuesRequest{
		Ids:   []string{"trace1"},
		Begin: 1000,
		End:   2000,
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/_/trace_values", bytes.NewReader(body))

	dfBuilder := dataframeMocks.NewDataFrameBuilder(t)
	perfGit := gitMocks.NewGit(t)
	anomalyStore := &anomalyMocks.Store{}

	api := NewTraceValuesApi(dfBuilder, perfGit, anomalyStore, nil)

	fakeDf := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 100, Timestamp: 1000},
			{Offset: 200, Timestamp: 2000},
		},
		TraceSet: types.TraceSet{
			"trace1": []float32{1.0, 2.0},
		},
	}
	dfBuilder.On("NewFromKeysAndRange", mock.Anything, []string{"trace1"}, time.Unix(1000, 0), time.Unix(2000, 0), mock.Anything).Return(fakeDf, nil)

	fakeAnomalyMap := chromeperf.AnomalyMap{
		"trace1": {
			types.CommitNumber(100): {Id: "anomaly1", State: "untriaged"},
		},
	}
	anomalyStore.On("GetAnomaliesInTimeRange", mock.Anything, []string{"trace1"}, time.Unix(1000, 0), time.Unix(2000, 0)).Return(fakeAnomalyMap, nil)

	api.traceValuesHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	var resp TraceValuesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	require.Len(t, resp.Results, 1)
	require.NotNil(t, resp.AnomalyMap)
	require.Equal(t, "anomaly1", resp.AnomalyMap["trace1"][100].Id)
}

func TestTraceValuesHandler_CappedMaxCommit(t *testing.T) {
	config.Config = &config.InstanceConfig{}
	w := httptest.NewRecorder()
	req := TraceValuesRequest{
		Ids:       []string{"trace1"},
		MinCommit: 100,
		MaxCommit: 200,
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/_/trace_values", bytes.NewReader(body))

	dfBuilder := dataframeMocks.NewDataFrameBuilder(t)
	perfGit := gitMocks.NewGit(t)

	api := NewTraceValuesApi(dfBuilder, perfGit, nil, nil)

	// Mock Git calls
	perfGit.On("CommitFromCommitNumber", mock.Anything, types.CommitNumber(100)).Return(provider.Commit{Timestamp: 1000}, nil)

	// Mock CommitNumberFromTime to return 150 as most recent
	perfGit.On("CommitNumberFromTime", mock.Anything, time.Time{}).Return(types.CommitNumber(150), nil)

	// Mock CommitFromCommitNumber for the capped value 150
	perfGit.On("CommitFromCommitNumber", mock.Anything, types.CommitNumber(150)).Return(provider.Commit{Timestamp: 1500}, nil)

	// Mock DataFrameBuilder calls
	fakeDf := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 100, Timestamp: 1000},
			{Offset: 150, Timestamp: 1500},
		},
		TraceSet: types.TraceSet{
			"trace1": []float32{1.0, 1.5},
		},
	}
	dfBuilder.On("NewFromKeysAndRange", mock.Anything, []string{"trace1"}, time.Unix(1000, 0), time.Unix(1500, 0), mock.Anything).Return(fakeDf, nil)

	api.traceValuesHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	var resp TraceValuesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	require.Len(t, resp.Results, 1)
	require.Len(t, resp.Results["trace1"], 2)
	require.Equal(t, int64(150), resp.Results["trace1"][1].CommitNumber)
}

func TestTraceValuesHandler_WithLegacyAnomalies(t *testing.T) {
	config.Config = &config.InstanceConfig{
		FetchAnomaliesFromSql:    false,
		FetchChromePerfAnomalies: true,
	}
	w := httptest.NewRecorder()
	req := TraceValuesRequest{
		Ids:   []string{"trace1"},
		Begin: 1000,
		End:   2000,
	}
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/_/trace_values", bytes.NewReader(body))

	dfBuilder := dataframeMocks.NewDataFrameBuilder(t)
	perfGit := gitMocks.NewGit(t)
	legacyAnomalyStore := &anomalyMocks.Store{}

	api := NewTraceValuesApi(dfBuilder, perfGit, nil, legacyAnomalyStore)

	fakeDf := &dataframe.DataFrame{
		Header: []*dataframe.ColumnHeader{
			{Offset: 100, Timestamp: 1000},
			{Offset: 200, Timestamp: 2000},
		},
		TraceSet: types.TraceSet{
			"trace1": []float32{1.0, 2.0},
		},
	}
	dfBuilder.On("NewFromKeysAndRange", mock.Anything, []string{"trace1"}, time.Unix(1000, 0), time.Unix(2000, 0), mock.Anything).Return(fakeDf, nil)

	fakeAnomalyMap := chromeperf.AnomalyMap{
		"trace1": {
			types.CommitNumber(100): {Id: "legacy_anomaly1", State: "untriaged"},
		},
	}
	legacyAnomalyStore.On("GetAnomaliesInTimeRange", mock.Anything, []string{"trace1"}, time.Unix(1000, 0), time.Unix(2000, 0)).Return(fakeAnomalyMap, nil)

	api.traceValuesHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	var resp TraceValuesResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	require.Len(t, resp.Results, 1)
	require.NotNil(t, resp.AnomalyMap)
	require.Equal(t, "legacy_anomaly1", resp.AnomalyMap["trace1"][100].Id)
}
