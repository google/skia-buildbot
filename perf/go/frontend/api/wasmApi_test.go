package api

import (
	"context"
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

func (m *mockTraceStore) GetWasmCache(ctx context.Context, tileNumber types.TileNumber, ps paramtools.ReadOnlyParamSet) (*tracestore.WasmCacheData, error) {
	args := m.Called(ctx, tileNumber, ps)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tracestore.WasmCacheData), args.Error(1)
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

func TestWasmApi_Handlers(t *testing.T) {
	ts := &mockTraceStore{}
	ps := &mockPsRefresher{}

	cacheDir, err := os.MkdirTemp("", "wasm_cache_test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(cacheDir)
	}()

	api := NewWasmApi(ts, ps, cacheDir)

	ts.On("GetLatestTile", mock.Anything).Return(types.TileNumber(1), nil)

	paramSet := paramtools.ParamSet{"config": []string{"8888"}}
	ps.On("GetAll").Return(paramSet.Freeze())

	cacheData := &tracestore.WasmCacheData{
		Meta:   []byte(`{"version":"123","stride":8,"count":1}`),
		Params: []byte(`[{"id":1,"key":"config","value":"8888"}]`),
		Traces: []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // 8 uint16s
	}
	ts.On("GetWasmCache", mock.Anything, types.TileNumber(1), mock.Anything).Return(cacheData, nil)

	// Test meta.json
	{
		req := httptest.NewRequest("GET", "/_/wasm/meta.json", nil)
		w := httptest.NewRecorder()
		api.metaHandler(w, req)
		require.Equal(t, http.StatusOK, w.Result().StatusCode)
		require.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
		require.Equal(t, cacheData.Meta, w.Body.Bytes())
	}

	// Test params.json
	{
		req := httptest.NewRequest("GET", "/_/wasm/params.json", nil)
		w := httptest.NewRecorder()
		api.paramsHandler(w, req)
		require.Equal(t, http.StatusOK, w.Result().StatusCode)
		require.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
		require.Equal(t, cacheData.Params, w.Body.Bytes())
	}

	// Test traces.bin
	{
		req := httptest.NewRequest("GET", "/_/wasm/traces.bin", nil)
		w := httptest.NewRecorder()
		api.tracesHandler(w, req)
		require.Equal(t, http.StatusOK, w.Result().StatusCode)
		require.Equal(t, "application/octet-stream", w.Result().Header.Get("Content-Type"))
		require.Equal(t, cacheData.Traces, w.Body.Bytes())
	}
}
