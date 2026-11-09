// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"sort"
)

// WasmParamEntry matches the Param JSON entry in /_/wasm/params.json.
type WasmParamEntry struct {
	ID    uint16 `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// WasmMeta matches the metadata payload in /_/wasm/meta.json.
type WasmMeta struct {
	Version      string            `json:"version"`
	Count        int               `json:"count"`
	Stride       int               `json:"stride"`
	CommonParams map[string]string `json:"commonParams"`
}

// WasmCache holds the pre-encoded /_/wasm/meta.json, /_/wasm/params.json, and
// /_/wasm/traces.bin buffers for the /e2 Web Worker.
type WasmCache struct {
	Meta       WasmMeta
	Params     []WasmParamEntry
	MetaJSON   []byte
	ParamsJSON []byte
	TracesBin  []byte
}

// BuildWasmCache encodes a slice of TraceDefinitions into the exact binary and
// JSON layout consumed by perf/modules/explore-multi-v2-sk/worker/filter.worker.ts.
func BuildWasmCache(traces []TraceDefinition) (*WasmCache, error) {
	numTraces := len(traces)
	paramSet := BuildParamSet(traces)

	// Identify parameters that share a single identical value across all traces.
	commonParams := make(map[string]string)
	if numTraces > 0 {
		for k, vals := range paramSet {
			if len(vals) != 1 {
				continue
			}
			val := vals[0]
			allHave := true
			for _, tr := range traces {
				if tr.Params[k] != val {
					allHave = false
					break
				}
			}
			if allHave {
				commonParams[k] = val
			}
		}
	}

	filteredKeys := make([]string, 0, len(paramSet))
	for k := range paramSet {
		if _, isCommon := commonParams[k]; !isCommon {
			filteredKeys = append(filteredKeys, k)
		}
	}
	sort.Strings(filteredKeys)

	var paramsList []WasmParamEntry
	paramLookup := make(map[string]map[string]uint16, len(filteredKeys))
	nextID := uint16(1)
	for _, k := range filteredKeys {
		vals := make([]string, len(paramSet[k]))
		copy(vals, paramSet[k])
		sort.Strings(vals)
		paramLookup[k] = make(map[string]uint16, len(vals))
		for _, v := range vals {
			paramsList = append(paramsList, WasmParamEntry{
				ID:    nextID,
				Key:   k,
				Value: v,
			})
			paramLookup[k][v] = nextID
			nextID++
		}
	}

	stride := len(filteredKeys)
	if stride == 0 || stride%8 != 0 {
		stride = (stride/8 + 1) * 8
	}

	tracesBin := make([]byte, numTraces*stride*2)
	for tIdx, tr := range traces {
		col := 0
		for _, k := range filteredKeys {
			if col >= stride {
				break
			}
			val, ok := tr.Params[k]
			if !ok {
				continue
			}
			pid := paramLookup[k][val]
			if pid != 0 {
				offset := (tIdx*stride + col) * 2
				binary.LittleEndian.PutUint16(tracesBin[offset:offset+2], pid)
				col++
			}
		}
	}

	meta := WasmMeta{
		Version:      "mock-v1",
		Count:        numTraces,
		Stride:       stride,
		CommonParams: commonParams,
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	paramsBytes, err := json.Marshal(paramsList)
	if err != nil {
		return nil, err
	}

	return &WasmCache{
		Meta:       meta,
		Params:     paramsList,
		MetaJSON:   metaBytes,
		ParamsJSON: paramsBytes,
		TracesBin:  tracesBin,
	}, nil
}

func (s *Server) WasmMetaHandler(w http.ResponseWriter, _ *http.Request) {
	cache, err := s.state.GetWasmCache()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(cache.MetaJSON)
}

func (s *Server) WasmParamsHandler(w http.ResponseWriter, _ *http.Request) {
	cache, err := s.state.GetWasmCache()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(cache.ParamsJSON)
}

func (s *Server) WasmTracesHandler(w http.ResponseWriter, _ *http.Request) {
	cache, err := s.state.GetWasmCache()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(cache.TracesBin)
}
