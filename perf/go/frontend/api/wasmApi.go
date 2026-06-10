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
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/psrefresh"
	"go.skia.org/infra/perf/go/tracestore"
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

	ps := api.psRefresher.GetAll()
	cacheData, err := api.traceStore.GetWasmCache(ctx, tile, ps)
	if err != nil {
		return skerr.Wrap(err)
	}

	var metaParsed struct {
		Version string `json:"version"`
		Count   int    `json:"count"`
		Stride  int    `json:"stride"`
	}
	if err := json.Unmarshal(cacheData.Meta, &metaParsed); err != nil {
		return skerr.Wrap(err)
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(cacheData.Traces); err != nil {
		return skerr.Wrap(err)
	}
	if err := gw.Close(); err != nil {
		return skerr.Wrap(err)
	}
	compressedTraces := buf.Bytes()

	api.cache = &wasmCache{
		tileNumber: tile,
		version:    metaParsed.Version,
		meta:       cacheData.Meta,
		params:     cacheData.Params,
		traces:     cacheData.Traces,
		createdAt:  time.Now(),
	}

	sklog.Infof("Generated Wasm cache: traces=%d stride=%d", metaParsed.Count, metaParsed.Stride)

	// Save to cache files
	if err := os.WriteFile(tracesFile, compressedTraces, 0644); err != nil {
		sklog.Errorf("Failed to save traces cache: %v", err)
	}
	if err := os.WriteFile(metaFile, cacheData.Meta, 0644); err != nil {
		sklog.Errorf("Failed to save meta cache: %v", err)
	}
	if err := os.WriteFile(paramsFile, cacheData.Params, 0644); err != nil {
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
