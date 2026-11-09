// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*httptest.Server, *Server) {
	t.Helper()
	r := chi.NewRouter()
	srv := NewServer()
	RegisterHandlers(r, srv, nil)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts, srv
}

func TestResetRestoresStateAndClearsMutations(t *testing.T) {
	ts, _ := setupTestServer(t)

	fileBugPayload := []byte(`{"keys":[123],"title":"Regression in JetStream3","component":"Chromium>Perf"}`)
	resp, err := ts.Client().Post(ts.URL+"/_/triage/file_bug", "application/json", bytes.NewReader(fileBugPayload))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var fileBugResp struct {
		BugID int `json:"bug_id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&fileBugResp))
	assert.Equal(t, 12345, fileBugResp.BugID)

	// Verify mutation was recorded.
	mutResp, err := ts.Client().Get(ts.URL + "/_/mock/mutations")
	require.NoError(t, err)
	defer mutResp.Body.Close()
	var mutData struct {
		Mutations []RecordedMutation `json:"mutations"`
	}
	require.NoError(t, json.NewDecoder(mutResp.Body).Decode(&mutData))
	require.Len(t, mutData.Mutations, 1)
	assert.Equal(t, "/_/triage/file_bug", mutData.Mutations[0].Endpoint)
	assert.Equal(t, "POST", mutData.Mutations[0].Method)

	// Untriaged list should no longer include anomaly 123.
	anomResp, err := ts.Client().Get(ts.URL + "/_/anomalies/anomaly_list?sheriff=Chromium+Perf+Sheriff")
	require.NoError(t, err)
	defer anomResp.Body.Close()
	var anomData struct {
		AnomalyList []MockAnomaly `json:"anomaly_list"`
	}
	require.NoError(t, json.NewDecoder(anomResp.Body).Decode(&anomData))
	for _, a := range anomData.AnomalyList {
		assert.NotEqual(t, "123", a.ID)
	}

	// Reset the server state.
	resetResp, err := ts.Client().Post(ts.URL+"/_/mock/reset", "application/json", nil)
	require.NoError(t, err)
	defer resetResp.Body.Close()
	require.Equal(t, http.StatusOK, resetResp.StatusCode)

	// Mutations should be empty and anomaly 123 should be untriaged again.
	mutRespAfter, err := ts.Client().Get(ts.URL + "/_/mock/mutations")
	require.NoError(t, err)
	defer mutRespAfter.Body.Close()
	require.NoError(t, json.NewDecoder(mutRespAfter.Body).Decode(&mutData))
	assert.Empty(t, mutData.Mutations)

	anomRespAfter, err := ts.Client().Get(ts.URL + "/_/anomalies/anomaly_list?sheriff=Chromium+Perf+Sheriff")
	require.NoError(t, err)
	defer anomRespAfter.Body.Close()
	require.NoError(t, json.NewDecoder(anomRespAfter.Body).Decode(&anomData))
	found123 := false
	for _, a := range anomData.AnomalyList {
		if a.ID == "123" {
			found123 = true
			assert.Equal(t, 0, a.BugID)
		}
	}
	assert.True(t, found123, "expected anomaly 123 to be restored after reset")
}

func TestPersonaSwitching(t *testing.T) {
	ts, srv := setupTestServer(t)

	assert.Equal(t, PersonaChrome, srv.State().GetPersona())
	chromeCfg := ConfigForPersona(srv.State().GetPersona())
	assert.Equal(t, "https://chromium.googlesource.com/chromium/src", chromeCfg.GitRepoURL)
	assert.True(t, chromeCfg.ShowBisectBtn)

	resp, err := ts.Client().Post(ts.URL+"/_/mock/set_persona", "application/json", bytes.NewReader([]byte(`{"persona":"v8"}`)))
	require.NoError(t, err)
	defer resp.Body.Close()
	var personaResp struct {
		Persona Persona       `json:"persona"`
		Config  PersonaConfig `json:"config"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&personaResp))
	assert.Equal(t, PersonaV8, personaResp.Persona)
	assert.Equal(t, "https://chromium.googlesource.com/v8/v8.git", personaResp.Config.GitRepoURL)
	assert.False(t, personaResp.Config.ShowBisectBtn)

	// Reset restores Chrome persona.
	resetResp, err := ts.Client().Post(ts.URL+"/_/mock/reset", "application/json", nil)
	require.NoError(t, err)
	defer resetResp.Body.Close()
	assert.Equal(t, PersonaChrome, srv.State().GetPersona())
}

func TestDatasetAndQueryMatching(t *testing.T) {
	ts, srv := setupTestServer(t)

	assert.Len(t, srv.State().GetTraces(), 24)
	assert.Len(t, srv.State().GetCommitHeaders(), 20)

	initResp, err := ts.Client().Get(ts.URL + "/_/initpage/")
	require.NoError(t, err)
	defer initResp.Body.Close()
	var initData struct {
		Dataframe struct {
			Header   []CommitHeader      `json:"header"`
			ParamSet map[string][]string `json:"paramset"`
		} `json:"dataframe"`
	}
	require.NoError(t, json.NewDecoder(initResp.Body).Decode(&initData))
	assert.Len(t, initData.Dataframe.Header, 20)
	assert.Equal(t, 67120, initData.Dataframe.Header[0].Offset)
	assert.Equal(t, 67139, initData.Dataframe.Header[19].Offset)
	assert.Contains(t, initData.Dataframe.ParamSet["benchmark"], "jetstream3.crossbench")
	assert.Contains(t, initData.Dataframe.ParamSet["benchmark"], "speedometer3.crossbench")
	assert.Contains(t, initData.Dataframe.ParamSet["benchmark"], "v8")
	assert.Contains(t, initData.Dataframe.ParamSet["test"], " TodoMVC-React-Complex-DOM")

	// Querying benchmark=jetstream3.crossbench&test=Total should match 2 traces with anomalies 123 and 456.
	framePayload := []byte(`{"queries":["benchmark=jetstream3.crossbench&test=Total"]}`)
	frameResp, err := ts.Client().Post(ts.URL+"/_/frame/start", "application/json", bytes.NewReader(framePayload))
	require.NoError(t, err)
	defer frameResp.Body.Close()
	var frameData struct {
		Status  string `json:"status"`
		Results struct {
			Dataframe struct {
				TraceSet map[string][]float32 `json:"traceset"`
			} `json:"dataframe"`
			AnomalyMap map[string]map[string]MockAnomaly `json:"anomalymap"`
		} `json:"results"`
	}
	require.NoError(t, json.NewDecoder(frameResp.Body).Decode(&frameData))
	assert.Equal(t, "Finished", frameData.Status)
	assert.Len(t, frameData.Results.Dataframe.TraceSet, 2)
	assert.Len(t, frameData.Results.AnomalyMap, 2)

	// Shortcut update and retrieval round-trip.
	scPayload := []byte(`{"graphs":[{"queries":["benchmark=jetstream3.crossbench&test=Total"]}]}`)
	scResp, err := ts.Client().Post(ts.URL+"/_/shortcut/update", "application/json", bytes.NewReader(scPayload))
	require.NoError(t, err)
	defer scResp.Body.Close()
	var scUpdate struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(scResp.Body).Decode(&scUpdate))
	assert.Equal(t, "mock-shortcut-2", scUpdate.ID)

	getScResp, err := ts.Client().Post(ts.URL+"/_/shortcut/get", "application/json", bytes.NewReader([]byte(`{"ID":"`+scUpdate.ID+`"}`)))
	require.NoError(t, err)
	defer getScResp.Body.Close()
	var scFetched GraphsShortcut
	require.NoError(t, json.NewDecoder(getScResp.Body).Decode(&scFetched))
	require.Len(t, scFetched.Graphs, 1)
	assert.NotNil(t, scFetched.Graphs[0].Formulas)
	assert.Equal(t, []string{"benchmark=jetstream3.crossbench&test=Total"}, scFetched.Graphs[0].Queries)

	// Querying non-existent trace IDs with no queries should return empty rather than all traces.
	assert.Empty(t, srv.State().MatchTraces(nil, []string{",nonexistent=1,"}))
}

func TestWasmCacheAndTraceValues(t *testing.T) {
	ts, srv := setupTestServer(t)

	metaResp, err := ts.Client().Get(ts.URL + "/_/wasm/meta.json")
	require.NoError(t, err)
	defer metaResp.Body.Close()
	var meta WasmMeta
	require.NoError(t, json.NewDecoder(metaResp.Body).Decode(&meta))
	assert.Equal(t, 24, meta.Count)
	assert.Equal(t, 8, meta.Stride)
	assert.Equal(t, "value", meta.CommonParams["stat"])

	paramsResp, err := ts.Client().Get(ts.URL + "/_/wasm/params.json")
	require.NoError(t, err)
	defer paramsResp.Body.Close()
	var params []WasmParamEntry
	require.NoError(t, json.NewDecoder(paramsResp.Body).Decode(&params))
	require.NotEmpty(t, params)
	for i, p := range params {
		assert.Equal(t, uint16(i+1), p.ID)
	}

	tracesResp, err := ts.Client().Get(ts.URL + "/_/wasm/traces.bin")
	require.NoError(t, err)
	defer tracesResp.Body.Close()
	rawBin, err := io.ReadAll(tracesResp.Body)
	require.NoError(t, err)
	require.Len(t, rawBin, meta.Count*meta.Stride*2)

	// Reconstruct every trace key from traces.bin + params.json + commonParams,
	// matching the algorithm in filter.worker.ts.
	expectedTraces := srv.State().GetTraces()
	for tIdx := 0; tIdx < meta.Count; tIdx++ {
		reconstructed := make(map[string]string)
		for k, v := range meta.CommonParams {
			reconstructed[k] = v
		}
		for col := 0; col < meta.Stride; col++ {
			offset := (tIdx*meta.Stride + col) * 2
			pid := binary.LittleEndian.Uint16(rawBin[offset : offset+2])
			if pid == 0 {
				break
			}
			require.LessOrEqual(t, int(pid), len(params))
			entry := params[pid-1]
			reconstructed[entry.Key] = entry.Value
		}
		assert.Equal(t, expectedTraces[tIdx].Key, MakeStructuredKey(reconstructed))
	}

	// Verify /_/trace_values filters out missing data sentinels and returns anomalymap.
	primaryKey := expectedTraces[0].Key
	sentinelKey := expectedTraces[8].Key
	tvPayload, err := json.Marshal(map[string]interface{}{
		"ids":        []string{primaryKey, sentinelKey},
		"min_commit": 67120,
		"max_commit": 67139,
	})
	require.NoError(t, err)
	tvResp, err := ts.Client().Post(ts.URL+"/_/trace_values", "application/json", bytes.NewReader(tvPayload))
	require.NoError(t, err)
	defer tvResp.Body.Close()
	var tvData struct {
		Results    map[string][]TraceRow             `json:"results"`
		AnomalyMap map[string]map[string]MockAnomaly `json:"anomalymap"`
	}
	require.NoError(t, json.NewDecoder(tvResp.Body).Decode(&tvData))
	assert.Len(t, tvData.Results[primaryKey], 20)
	assert.Len(t, tvData.Results[sentinelKey], 18)
	assert.Contains(t, tvData.AnomalyMap, primaryKey)

	// Verify /_/trace_values filters rows to the requested commit sub-range.
	subRangePayload, err := json.Marshal(map[string]interface{}{
		"ids":        []string{primaryKey},
		"min_commit": 67125,
		"max_commit": 67130,
	})
	require.NoError(t, err)
	subRangeResp, err := ts.Client().Post(ts.URL+"/_/trace_values", "application/json", bytes.NewReader(subRangePayload))
	require.NoError(t, err)
	defer subRangeResp.Body.Close()
	var subRangeData struct {
		Results map[string][]TraceRow `json:"results"`
	}
	require.NoError(t, json.NewDecoder(subRangeResp.Body).Decode(&subRangeData))
	require.Len(t, subRangeData.Results[primaryKey], 6)
	assert.Equal(t, 67125, subRangeData.Results[primaryKey][0].CommitNumber)
	assert.Equal(t, 67130, subRangeData.Results[primaryKey][5].CommitNumber)

	// Verify WasmCache is cached across calls and invalidated on Reset.
	cache1, err := srv.State().GetWasmCache()
	require.NoError(t, err)
	cache2, err := srv.State().GetWasmCache()
	require.NoError(t, err)
	assert.Same(t, cache1, cache2)
	srv.State().Reset()
	cacheAfterReset, err := srv.State().GetWasmCache()
	require.NoError(t, err)
	assert.NotSame(t, cache1, cacheAfterReset)
}
