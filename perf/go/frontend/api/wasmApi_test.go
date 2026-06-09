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

func TestEncodeTraces(t *testing.T) {
	// Case 1: All params in lookup
	keys := []string{
		",arch=arm,config=8888,status=failed,",
		",arch=x86,config=8888,status=passed,",
	}

	lookup := map[string]map[string]uint16{
		"arch": {
			"arm": 1,
			"x86": 2,
		},
		"config": {
			"8888": 3,
		},
		"status": {
			"failed": 4,
			"passed": 5,
		},
	}

	stride := 8 // typical stride is multiple of 8

	binary, count := encodeTraces(keys, lookup, stride)
	require.Equal(t, 2, count)
	require.Equal(t, 2*stride*2, len(binary))

	// Expected row 1: [1, 3, 4, 0, 0, 0, 0, 0]
	// Expected row 2: [2, 3, 5, 0, 0, 0, 0, 0]
	expected := make([]byte, 2*stride*2)
	// Row 1
	expected[0], expected[1] = 1, 0
	expected[2], expected[3] = 3, 0
	expected[4], expected[5] = 4, 0
	// Row 2
	offset := stride * 2
	expected[offset+0], expected[offset+1] = 2, 0
	expected[offset+2], expected[offset+3] = 3, 0
	expected[offset+4], expected[offset+5] = 5, 0

	require.Equal(t, expected, binary)
}

func TestEncodeTraces_WithMissingParamsInLookup(t *testing.T) {
	// Case 2: Some params missing in lookup (e.g. common params)
	keys := []string{
		",arch=arm,config=8888,status=failed,",
		",arch=x86,config=8888,status=passed,",
	}

	// config is missing from lookup
	lookup := map[string]map[string]uint16{
		"arch": {
			"arm": 1,
			"x86": 2,
		},
		"status": {
			"failed": 3,
			"passed": 4,
		},
	}

	stride := 8

	binary, count := encodeTraces(keys, lookup, stride)
	require.Equal(t, 2, count)
	require.Equal(t, 2*stride*2, len(binary))

	// Expected row 1: [1, 3, 0, 0, 0, 0, 0, 0]  (config skipped, status becomes index 1 in row)
	// Expected row 2: [2, 4, 0, 0, 0, 0, 0, 0]
	expected := make([]byte, 2*stride*2)
	// Row 1
	expected[0], expected[1] = 1, 0
	expected[2], expected[3] = 3, 0
	// Row 2
	offset := stride * 2
	expected[offset+0], expected[offset+1] = 2, 0
	expected[offset+2], expected[offset+3] = 4, 0

	require.Equal(t, expected, binary)
}
