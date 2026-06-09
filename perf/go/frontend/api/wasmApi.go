package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/psrefresh"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/tracestore/sqltracestore"
	"go.skia.org/infra/perf/go/types"
)

const (
	defaultCacheTTL = 5 * time.Minute
	fileCacheTTL    = 14 * 24 * time.Hour
)

type Param struct {
	Id    uint16 `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type wasmApi struct {
	traceStore  tracestore.TraceStore
	psRefresher psrefresh.ParamSetRefresher
	cacheDir    string

	mutex sync.Mutex
	cache *wasmCache
}

type wasmCache struct {
	tileNumber types.TileNumber
	version    string
	meta       []byte
	params     []byte
	traces     []byte
	createdAt  time.Time
}

func NewWasmApi(traceStore tracestore.TraceStore, psRefresher psrefresh.ParamSetRefresher, cacheDir string) *wasmApi {
	return &wasmApi{
		traceStore:  traceStore,
		psRefresher: psRefresher,
		cacheDir:    cacheDir,
	}
}

func (api *wasmApi) Start(ctx context.Context) {
	if api.traceStore == nil {
		sklog.Warningf("TraceStore is nil, not starting background Wasm cache generator")
		return
	}
	sklog.Infof("Starting background Wasm cache generator")
	go func() {
		if err := api.ensureCache(ctx); err != nil {
			sklog.Errorf("Failed to generate initial Wasm cache: %v", err)
		}

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := api.ensureCache(ctx); err != nil {
					sklog.Errorf("Failed to refresh Wasm cache: %v", err)
				}
			}
		}
	}()
}

func (api *wasmApi) RegisterHandlers(router *chi.Mux) {
	router.Get("/_/wasm/meta.json", api.metaHandler)
	router.Get("/_/wasm/params.json", api.paramsHandler)
	router.Get("/_/wasm/traces.bin", api.tracesHandler)
}

func (api *wasmApi) ensureCache(ctx context.Context) error {
	api.mutex.Lock()
	defer api.mutex.Unlock()

	tileCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tile, err := api.traceStore.GetLatestTile(tileCtx)
	if err != nil {
		return skerr.Wrap(err)
	}

	if api.cache != nil && api.cache.tileNumber == tile {
		// The latest tile is actively updated, so we enforce a TTL.
		if time.Since(api.cache.createdAt) < defaultCacheTTL {
			return nil
		}
		sklog.Infof("Refreshing Wasm cache for tile %d (TTL expired)", tile)
	}

	if err := os.MkdirAll(api.cacheDir, 0755); err != nil {
		return skerr.Wrapf(err, "Failed to create cache dir %q", api.cacheDir)
	}

	tracesFile := filepath.Join(api.cacheDir, fmt.Sprintf("traces_%d.bin", tile))
	metaFile := filepath.Join(api.cacheDir, fmt.Sprintf("meta_%d.json", tile))
	paramsFile := filepath.Join(api.cacheDir, fmt.Sprintf("params_%d.json", tile))

	stat, err := os.Stat(tracesFile)
	if err == nil && time.Since(stat.ModTime()) < fileCacheTTL {

		_, errMeta := os.Stat(metaFile)
		_, errParams := os.Stat(paramsFile)
		if errMeta == nil && errParams == nil {
			sklog.Infof("Loading Wasm cache from files for tile %d", tile)
			tracesBuf, err1 := os.ReadFile(tracesFile)
			metaBuf, err2 := os.ReadFile(metaFile)
			paramsBuf, err3 := os.ReadFile(paramsFile)
			if err1 == nil && err2 == nil && err3 == nil {
				gr, err := gzip.NewReader(bytes.NewReader(tracesBuf))
				if err == nil {
					decompressedTraces, err := io.ReadAll(gr)
					_ = gr.Close()
					if err == nil {
						api.cache = &wasmCache{
							tileNumber: tile,
							traces:     decompressedTraces,
							meta:       metaBuf,
							params:     paramsBuf,
							createdAt:  stat.ModTime(),
						}
						return nil
					} else {
						sklog.Warningf("Failed to decompress traces cache file on disk: %v. Will regenerate cache.", err)
					}
				} else {
					sklog.Warningf("Failed to parse traces cache gzip header on disk: %v. Will regenerate cache.", err)
				}
			} else {
				sklog.Errorf("Failed to read cache files: %v, %v, %v", err1, err2, err3)
			}
		}
	}

	sklog.Infof("Generating Wasm memory cache for tile %d", tile)

	// Fetch traces to build traces.bin.
	q, err := query.New(nil)
	if err != nil {
		return skerr.Wrap(err)
	}
	queryCtx, cancelQuery := context.WithTimeout(ctx, 60*time.Second)
	defer cancelQuery()
	queryCtx = context.WithValue(queryCtx, sqltracestore.UseInvertedIndex, true)
	queryCtx = context.WithValue(queryCtx, sqltracestore.AllowEmptyQuery, true)
	outParams, err := api.traceStore.QueryTracesIDOnly(queryCtx, tile, q)
	if err != nil {
		return skerr.Wrap(err)
	}

	var keys []string
	commonParams := map[string]string{}
	isFirst := true

	for p := range outParams {
		key, err := query.MakeKeyFast(p)
		if err != nil {
			continue
		}
		keys = append(keys, key)

		if isFirst {
			for k, v := range p {
				commonParams[k] = v
			}
			isFirst = false
		} else {
			for k, v := range commonParams {
				if pVal, ok := p[k]; !ok || pVal != v {
					delete(commonParams, k)
				}
			}
		}
	}

	sort.Strings(keys)

	// Filter ParamSet to remove common keys
	ps := api.psRefresher.GetAll()
	filteredPs := paramtools.ParamSet{}
	for k, v := range ps {
		if _, ok := commonParams[k]; !ok {
			filteredPs[k] = v
		}
	}

	lookup, stride, params := api.buildLookup(filteredPs)

	tracesBinary, traceCount := encodeTraces(keys, lookup, stride)

	version := fmt.Sprintf("%d", time.Now().Unix())

	meta := struct {
		Stride       int               `json:"stride"`
		Count        int               `json:"count"`
		Version      string            `json:"version"`
		CommonParams map[string]string `json:"commonParams"`
	}{
		Stride:       stride,
		Count:        traceCount,
		Version:      version,
		CommonParams: commonParams,
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return skerr.Wrap(err)
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return skerr.Wrap(err)
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(tracesBinary); err != nil {
		return skerr.Wrap(err)
	}
	if err := gw.Close(); err != nil {
		return skerr.Wrap(err)
	}
	compressedTraces := buf.Bytes()

	api.cache = &wasmCache{
		tileNumber: tile,
		version:    version,
		meta:       metaBytes,
		params:     paramsBytes,
		traces:     tracesBinary,
		createdAt:  time.Now(),
	}

	sklog.Infof("Generated Wasm cache: traces=%d stride=%d", traceCount, stride)

	// Save to cache files
	if err := os.WriteFile(tracesFile, compressedTraces, 0644); err != nil {
		sklog.Errorf("Failed to save traces cache: %v", err)
	}
	if err := os.WriteFile(metaFile, metaBytes, 0644); err != nil {
		sklog.Errorf("Failed to save meta cache: %v", err)
	}
	if err := os.WriteFile(paramsFile, paramsBytes, 0644); err != nil {
		sklog.Errorf("Failed to save params cache: %v", err)
	}

	return nil
}

func (api *wasmApi) metaHandler(w http.ResponseWriter, r *http.Request) {
	if err := api.ensureCache(r.Context()); err != nil {
		httputils.ReportError(w, err, "Failed to ensure cache", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(api.cache.meta); err != nil {
		sklog.Errorf("Failed to write meta response: %v", err)
	}
}

func (api *wasmApi) paramsHandler(w http.ResponseWriter, r *http.Request) {
	if err := api.ensureCache(r.Context()); err != nil {
		httputils.ReportError(w, err, "Failed to ensure cache", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(api.cache.params); err != nil {
		sklog.Errorf("Failed to write params response: %v", err)
	}
}

func (api *wasmApi) tracesHandler(w http.ResponseWriter, r *http.Request) {
	if err := api.ensureCache(r.Context()); err != nil {
		httputils.ReportError(w, err, "Failed to ensure cache", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	if _, err := w.Write(api.cache.traces); err != nil {
		sklog.Errorf("Failed to write traces response: %v", err)
	}
}

func (api *wasmApi) buildLookup(ps paramtools.ParamSet) (map[string]map[string]uint16, int, []Param) {
	lookup := map[string]map[string]uint16{}
	var idCounter uint16 = 1
	var params []Param

	for key, values := range ps {
		lookup[key] = map[string]uint16{}
		for _, val := range values {
			id := idCounter
			idCounter++
			params = append(params, Param{Id: id, Key: key, Value: val})
			lookup[key][val] = id
		}
	}

	stride := len(ps)
	if stride%8 != 0 {
		stride = (stride/8 + 1) * 8
	}
	return lookup, stride, params
}

func encodeTraces(keys []string, lookup map[string]map[string]uint16, stride int) ([]byte, int) {
	tracesBinary := make([]byte, 0, len(keys)*stride*2)
	traceCount := 0
	row := make([]uint16, stride)
	for _, key := range keys {
		if len(key) < 3 {
			continue
		}
		parts := strings.Split(key[1:len(key)-1], ",")

		// Reset row
		for i := range row {
			row[i] = 0
		}

		i := 0
		for _, part := range parts {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}
			k, val := kv[0], kv[1]
			if l, ok := lookup[k]; ok {
				if id, ok := l[val]; ok {
					row[i] = id
					i++
				}
			}
		}

		for _, v := range row {
			tracesBinary = append(tracesBinary, byte(v), byte(v>>8))
		}
		traceCount++
	}
	return tracesBinary, traceCount
}
