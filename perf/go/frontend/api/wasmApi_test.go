package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
)

type mockTraceStore struct {
	tracestore.TraceStore
	mock.Mock
}

func (m *mockTraceStore) GetLatestTile(ctx context.Context) (types.TileNumber, error) {
	args := m.Called(ctx)
	return args.Get(0).(types.TileNumber), args.Error(1)
}

func (m *mockTraceStore) QueryTracesIDOnly(ctx context.Context, tileNumber types.TileNumber, q *query.Query) (<-chan paramtools.Params, error) {
	args := m.Called(ctx, tileNumber, q)
	return args.Get(0).(<-chan paramtools.Params), args.Error(1)
}

func (m *mockTraceStore) TileSize() int32 {
	args := m.Called()
	return args.Get(0).(int32)
}

type mockPsRefresher struct {
	mock.Mock
}

func (m *mockPsRefresher) GetAll() paramtools.ReadOnlyParamSet {
	args := m.Called()
	return args.Get(0).(paramtools.ReadOnlyParamSet)
}

func (m *mockPsRefresher) GetParamSetForQuery(ctx context.Context, q *query.Query, values url.Values) (int64, paramtools.ParamSet, error) {
	args := m.Called(ctx, q, values)
	return args.Get(0).(int64), args.Get(1).(paramtools.ParamSet), args.Error(2)
}

func (m *mockPsRefresher) Start(period time.Duration) error {
	args := m.Called(period)
	return args.Error(0)
}

func TestWasmApi_MetaHandler_Success(t *testing.T) {
	ts := &mockTraceStore{}
	ps := &mockPsRefresher{}

	cacheDir, err := os.MkdirTemp("", "wasm_cache_test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(cacheDir)
	}()

	api := NewWasmApi(ts, ps, cacheDir)

	ts.On("GetLatestTile", mock.Anything).Return(types.TileNumber(1), nil)
	ts.On("TileSize").Return(int32(256))

	p1 := paramtools.Params{"config": "8888", "arch": "arm"}

	pChan := make(chan paramtools.Params, 1)
	pChan <- p1
	close(pChan)
	ts.On("QueryTracesIDOnly", mock.Anything, types.TileNumber(1), mock.Anything).Return((<-chan paramtools.Params)(pChan), nil)

	paramSet := paramtools.ParamSet{}
	paramSet["config"] = []string{"8888"}
	paramSet["arch"] = []string{"arm"}
	ps.On("GetAll").Return(paramSet.Freeze())

	req := httptest.NewRequest("GET", "/_/wasm/meta.json", nil)
	w := httptest.NewRecorder()

	api.metaHandler(w, req)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	var meta struct {
		Stride       int               `json:"stride"`
		Count        int               `json:"count"`
		Version      string            `json:"version"`
		CommonParams map[string]string `json:"commonParams"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &meta)
	require.NoError(t, err)

	require.Equal(t, 1, meta.Count)
	// Since there is only 1 trace, both params should be common!
	require.Equal(t, "8888", meta.CommonParams["config"])
	require.Equal(t, "arm", meta.CommonParams["arch"])
}

func TestWasmApi_EmptyQueryWithStat(t *testing.T) {
	ts := &mockTraceStore{}
	ps := &mockPsRefresher{}

	cacheDir, err := os.MkdirTemp("", "wasm_cache_test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(cacheDir)
	}()

	api := NewWasmApi(ts, ps, cacheDir)

	ts.On("GetLatestTile", mock.Anything).Return(types.TileNumber(1), nil)
	ts.On("TileSize").Return(int32(256))

	p1 := paramtools.Params{"config": "8888", "stat": "median"}

	pChan := make(chan paramtools.Params, 1)
	pChan <- p1
	close(pChan)

	// We expect the wildcard name query to match all traces!
	ts.On("QueryTracesIDOnly", mock.Anything, types.TileNumber(1), mock.Anything).Return((<-chan paramtools.Params)(pChan), nil)

	paramSet := paramtools.ParamSet{}
	paramSet["config"] = []string{"8888"}
	paramSet["stat"] = []string{"median"}
	ps.On("GetAll").Return(paramSet.Freeze())

	req := httptest.NewRequest("GET", "/_/wasm/meta.json", nil)
	w := httptest.NewRecorder()
	api.metaHandler(w, req)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
}

func TestWasmApi_CommonParamsExtraction(t *testing.T) {
	ts := &mockTraceStore{}
	ps := &mockPsRefresher{}

	cacheDir, err := os.MkdirTemp("", "wasm_cache_test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(cacheDir)
	}()

	api := NewWasmApi(ts, ps, cacheDir)

	ts.On("GetLatestTile", mock.Anything).Return(types.TileNumber(1), nil)
	ts.On("TileSize").Return(int32(256))

	// Return two traces that share "master=master" but differ in "bot"
	p1 := paramtools.Params{"master": "master", "bot": "bot1"}
	p2 := paramtools.Params{"master": "master", "bot": "bot2"}

	pChan := make(chan paramtools.Params, 2)
	pChan <- p1
	pChan <- p2
	close(pChan)
	ts.On("QueryTracesIDOnly", mock.Anything, types.TileNumber(1), mock.Anything).Return((<-chan paramtools.Params)(pChan), nil)

	paramSet := paramtools.ParamSet{}
	paramSet["master"] = []string{"master"}
	paramSet["bot"] = []string{"bot1", "bot2"}
	ps.On("GetAll").Return(paramSet.Freeze())

	req := httptest.NewRequest("GET", "/_/wasm/meta.json", nil)
	w := httptest.NewRecorder()

	api.metaHandler(w, req)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	var meta struct {
		Stride       int               `json:"stride"`
		Count        int               `json:"count"`
		Version      string            `json:"version"`
		CommonParams map[string]string `json:"commonParams"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &meta)
	require.NoError(t, err)

	require.Equal(t, 2, meta.Count)
	require.Equal(t, "master", meta.CommonParams["master"])
	_, ok := meta.CommonParams["bot"]
	require.False(t, ok) // bot should not be common!
}
