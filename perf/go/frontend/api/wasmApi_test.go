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

func TestWasmApi_StaleWhileRevalidate(t *testing.T) {
	ts := &mockTraceStore{}
	ps := &mockPsRefresher{}

	cacheDir, err := os.MkdirTemp("", "wasm_cache_test_swr")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(cacheDir)
	}()

	api := NewWasmApi(ts, ps, cacheDir)
	api.defaultCacheTTL = 10 * time.Millisecond

	// --- 1. Initial Synchronous Populate (Tile 1) ---
	ts.On("GetLatestTile", mock.Anything).Return(types.TileNumber(1), nil).Once()
	paramSet := paramtools.ParamSet{"config": []string{"8888"}}
	ps.On("GetAll").Return(paramSet.Freeze())

	cacheData1 := &tracestore.WasmCacheData{
		Meta:   []byte(`{"version":"123","stride":8,"count":1}`),
		Params: []byte(`[{"id":1,"key":"config","value":"8888"}]`),
		Traces: []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	ts.On("GetWasmCache", mock.Anything, types.TileNumber(1), mock.Anything).Return(cacheData1, nil).Once()

	// Initial request blocks and populates cache
	req := httptest.NewRequest("GET", "/_/wasm/meta.json", nil)
	w := httptest.NewRecorder()
	api.metaHandler(w, req)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.Equal(t, cacheData1.Meta, w.Body.Bytes())

	// --- 2. Fresh Cache Request (Tile 1) ---
	// Should return instantly from memory, no DB mocks should be called (Once mocks would fail if called)
	w = httptest.NewRecorder()
	api.metaHandler(w, req)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.Equal(t, cacheData1.Meta, w.Body.Bytes())

	// --- 3. Stale Cache Request (Tile 2 available) ---
	// Make it stale by sleeping
	time.Sleep(15 * time.Millisecond)

	// Setup mocks for the background update to Tile 2
	ts.On("GetLatestTile", mock.Anything).Return(types.TileNumber(2), nil).Once()
	cacheData2 := &tracestore.WasmCacheData{
		Meta:   []byte(`{"version":"456","stride":8,"count":1}`),
		Params: []byte(`[{"id":1,"key":"config","value":"8888"}]`),
		Traces: []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	ts.On("GetWasmCache", mock.Anything, types.TileNumber(2), mock.Anything).Return(cacheData2, nil).Once()

	// This request should return STALE data (cacheData1) instantly,
	// and trigger the background update to Tile 2.
	w = httptest.NewRecorder()
	api.metaHandler(w, req)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.Equal(t, cacheData1.Meta, w.Body.Bytes()) // Still got old data

	// Wait for background update to complete by polling
	success := false
	for start := time.Now(); time.Since(start) < 2*time.Second; {
		c := api.getCache()
		if c != nil && string(c.meta) == string(cacheData2.Meta) {
			success = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, success, "Cache was not updated in background")

	// --- 4. Request after Background Update ---
	// Should now return the new data (cacheData2)
	w = httptest.NewRecorder()
	api.metaHandler(w, req)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.Equal(t, cacheData2.Meta, w.Body.Bytes()) // Got new data!
}

func TestWasmApi_SingleFlightUpdates(t *testing.T) {
	ts := &mockTraceStore{}
	ps := &mockPsRefresher{}

	cacheDir, err := os.MkdirTemp("", "wasm_cache_test_sf")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(cacheDir)
	}()

	api := NewWasmApi(ts, ps, cacheDir)
	api.defaultCacheTTL = 10 * time.Millisecond

	// Populate initially
	ts.On("GetLatestTile", mock.Anything).Return(types.TileNumber(1), nil).Once()
	paramSet := paramtools.ParamSet{"config": []string{"8888"}}
	ps.On("GetAll").Return(paramSet.Freeze())
	cacheData1 := &tracestore.WasmCacheData{
		Meta:   []byte(`{"version":"123","stride":8,"count":1}`),
		Params: []byte(`[{"id":1,"key":"config","value":"8888"}]`),
		Traces: []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	ts.On("GetWasmCache", mock.Anything, types.TileNumber(1), mock.Anything).Return(cacheData1, nil).Once()

	req := httptest.NewRequest("GET", "/_/wasm/meta.json", nil)
	w := httptest.NewRecorder()
	api.metaHandler(w, req)
	require.Equal(t, cacheData1.Meta, w.Body.Bytes())

	// Make stale
	time.Sleep(15 * time.Millisecond)

	// Setup background update mock that blocks
	blockChan := make(chan struct{})
	ts.On("GetLatestTile", mock.Anything).Return(types.TileNumber(2), nil).Run(func(args mock.Arguments) {
		<-blockChan // Block here
	}).Once() // Crucial: Only Once! If a second update is triggered, it will panic because no mock matches.

	// Trigger first stale request (starts background update, which blocks)
	w1 := httptest.NewRecorder()
	api.metaHandler(w1, req)
	require.Equal(t, cacheData1.Meta, w1.Body.Bytes()) // Returns stale instantly

	// Trigger multiple concurrent requests.
	// They should all return stale data instantly and NOT trigger new updates.
	// If they tried to start a new update, they would call GetLatestTile again,
	// which would panic/fail the test because we only mocked it Once().
	for i := 0; i < 5; i++ {
		w2 := httptest.NewRecorder()
		api.metaHandler(w2, req)
		require.Equal(t, cacheData1.Meta, w2.Body.Bytes())
	}

	// Unblock the background update
	close(blockChan)

	// We need to mock GetWasmCache for Tile 2 because the blocked update will now proceed to it.
	cacheData2 := &tracestore.WasmCacheData{
		Meta:   []byte(`{"version":"456","stride":8,"count":1}`),
		Params: []byte(`[{"id":1,"key":"config","value":"8888"}]`),
		Traces: []byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
	ts.On("GetWasmCache", mock.Anything, types.TileNumber(2), mock.Anything).Return(cacheData2, nil).Once()

	// Wait for update to finish
	success := false
	for start := time.Now(); time.Since(start) < 2*time.Second; {
		c := api.getCache()
		if c != nil && string(c.meta) == string(cacheData2.Meta) {
			success = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, success, "Cache was not updated")
}
