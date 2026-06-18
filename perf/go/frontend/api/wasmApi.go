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

type Param struct {
	Id    uint16 `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type wasmApi struct {
	traceStore  tracestore.TraceStore
	psRefresher psrefresh.ParamSetRefresher
	cacheDir    string

	cacheMu sync.RWMutex
	cache   *wasmCache

	updateMu sync.Mutex

	// Configurable TTLs
	defaultCacheTTL time.Duration
	fileCacheTTL    time.Duration
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
		traceStore:      traceStore,
		psRefresher:     psRefresher,
		cacheDir:        cacheDir,
		defaultCacheTTL: 5 * time.Minute,
		fileCacheTTL:    14 * 24 * time.Hour,
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
				api.tryTriggerBackgroundUpdate()
			}
		}
	}()
}

func (api *wasmApi) RegisterHandlers(router *chi.Mux) {
	router.Get("/_/wasm/meta.json", api.metaHandler)
	router.Get("/_/wasm/params.json", api.paramsHandler)
	router.Get("/_/wasm/traces.bin", api.tracesHandler)
}

// getCache returns the current cache, and if it is stale, triggers an async update.
// It may return nil if the cache has never been populated.
func (api *wasmApi) getCache() *wasmCache {
	api.cacheMu.RLock()
	cache := api.cache
	api.cacheMu.RUnlock()

	if cache == nil {
		return nil
	}

	if time.Since(cache.createdAt) > api.defaultCacheTTL {
		api.tryTriggerBackgroundUpdate()
	}

	return cache
}

// tryTriggerBackgroundUpdate attempts to start a non-blocking background cache regeneration.
// It returns true if a new regeneration was started, false if one was already in progress.
func (api *wasmApi) tryTriggerBackgroundUpdate() bool {
	if api.updateMu.TryLock() {
		go func() {
			defer api.updateMu.Unlock()
			if err := api.regenerateCache(context.Background()); err != nil {
				sklog.Errorf("Failed to regenerate Wasm cache in background: %v", err)
			}
		}()
		return true
	}
	return false
}

// ensureCache is a blocking call to ensure the cache is populated.
// It is used for initial load or as a fallback if the cache is nil.
func (api *wasmApi) ensureCache(ctx context.Context) error {
	api.cacheMu.RLock()
	hasCache := api.cache != nil
	api.cacheMu.RUnlock()
	if hasCache {
		return nil
	}

	api.updateMu.Lock()
	defer api.updateMu.Unlock()

	// Double check after lock
	api.cacheMu.RLock()
	hasCache = api.cache != nil
	api.cacheMu.RUnlock()
	if hasCache {
		return nil
	}

	return api.regenerateCache(ctx)
}

// regenerateCache does the actual work of loading/generating the cache.
// It must be called under updateMu lock.
func (api *wasmApi) regenerateCache(ctx context.Context) error {
	tileCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tile, err := api.traceStore.GetLatestTile(tileCtx)
	if err != nil {
		return skerr.Wrap(err)
	}

	api.cacheMu.RLock()
	currentCache := api.cache
	api.cacheMu.RUnlock()

	if currentCache != nil && currentCache.tileNumber == tile {
		if time.Since(currentCache.createdAt) < api.defaultCacheTTL {
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

	var newCache *wasmCache

	stat, err := os.Stat(tracesFile)
	if err == nil && time.Since(stat.ModTime()) < api.fileCacheTTL {
		_, errMeta := os.Stat(metaFile)
		_, errParams := os.Stat(paramsFile)
		if errMeta == nil && errParams == nil {
			sklog.Infof("Loading Wasm cache from files for tile %d", tile)
			tracesBuf, err1 := os.ReadFile(tracesFile)
			metaBuf, err2 := os.ReadFile(metaFile)
			paramsBuf, err3 := os.ReadFile(paramsFile)
			if err1 == nil && err2 == nil && err3 == nil {
				var metaParsed struct {
					Version string `json:"version"`
				}
				errMetaParse := json.Unmarshal(metaBuf, &metaParsed)

				gr, err := gzip.NewReader(bytes.NewReader(tracesBuf))
				if err == nil && errMetaParse == nil {
					decompressedTraces, err := io.ReadAll(gr)
					_ = gr.Close()
					if err == nil {
						newCache = &wasmCache{
							tileNumber: tile,
							version:    metaParsed.Version,
							traces:     decompressedTraces,
							meta:       metaBuf,
							params:     paramsBuf,
							// Set createdAt to time.Now() to reflect that the in-memory cache is fresh as of now,
							// even if the file on disk is older. Using stat.ModTime() here could lead to
							// a loop where getCache considers the cache stale and triggers regenerateCache,
							// which reloads the same file and sets the old createdAt again.
							createdAt: time.Now(),
						}
					} else {
						sklog.Warningf("Failed to decompress traces cache file on disk: %v. Will regenerate cache.", err)
					}
				} else {
					sklog.Warningf("Failed to parse traces cache gzip header or meta JSON on disk: %v, %v. Will regenerate cache.", err, errMetaParse)
				}
			} else {
				sklog.Errorf("Failed to read cache files: %v, %v, %v", err1, err2, err3)
			}
		}
	}

	if newCache == nil {
		var err error
		newCache, err = api.generateCacheFromDB(ctx, tile)
		if err != nil {
			return skerr.Wrap(err)
		}
	}

	api.cacheMu.Lock()
	api.cache = newCache
	api.cacheMu.Unlock()

	return nil
}

// generateCacheFromDB fetches data from the database, compresses it, saves it to disk, and returns a new wasmCache.
// It must be called under updateMu lock.
func (api *wasmApi) generateCacheFromDB(ctx context.Context, tile types.TileNumber) (*wasmCache, error) {
	sklog.Infof("Generating Wasm memory cache for tile %d", tile)

	ps := api.psRefresher.GetAll()
	cacheData, err := api.traceStore.GetWasmCache(ctx, tile, ps)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var metaParsed struct {
		Version string `json:"version"`
		Count   int    `json:"count"`
		Stride  int    `json:"stride"`
	}
	if err := json.Unmarshal(cacheData.Meta, &metaParsed); err != nil {
		return nil, skerr.Wrap(err)
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(cacheData.Traces); err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := gw.Close(); err != nil {
		return nil, skerr.Wrap(err)
	}
	compressedTraces := buf.Bytes()

	tracesFile := filepath.Join(api.cacheDir, fmt.Sprintf("traces_%d.bin", tile))
	metaFile := filepath.Join(api.cacheDir, fmt.Sprintf("meta_%d.json", tile))
	paramsFile := filepath.Join(api.cacheDir, fmt.Sprintf("params_%d.json", tile))

	if err := os.WriteFile(tracesFile, compressedTraces, 0644); err != nil {
		sklog.Errorf("Failed to save traces cache: %v", err)
	}
	if err := os.WriteFile(metaFile, cacheData.Meta, 0644); err != nil {
		sklog.Errorf("Failed to save meta cache: %v", err)
	}
	if err := os.WriteFile(paramsFile, cacheData.Params, 0644); err != nil {
		sklog.Errorf("Failed to save params cache: %v", err)
	}

	sklog.Infof("Successfully saved Wasm cache files to disk for tile %d (traces: %d bytes compressed, %d bytes uncompressed)",
		tile, len(compressedTraces), len(cacheData.Traces))

	return &wasmCache{
		tileNumber: tile,
		version:    metaParsed.Version,
		meta:       cacheData.Meta,
		params:     cacheData.Params,
		traces:     cacheData.Traces,
		createdAt:  time.Now(),
	}, nil
}

// getCacheOrEnsure is a helper that attempts to get the cache from memory,
// falling back to a blocking ensureCache call if it is nil.
func (api *wasmApi) getCacheOrEnsure(ctx context.Context) (*wasmCache, error) {
	cache := api.getCache()
	if cache == nil {
		if err := api.ensureCache(ctx); err != nil {
			return nil, skerr.Wrap(err)
		}
		cache = api.getCache()
	}
	if cache == nil {
		return nil, skerr.Fmt("cache is nil after ensure")
	}
	return cache, nil
}

func (api *wasmApi) metaHandler(w http.ResponseWriter, r *http.Request) {
	cache, err := api.getCacheOrEnsure(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get cache", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(cache.meta); err != nil {
		sklog.Errorf("Failed to write meta response: %v", err)
	}
}

func (api *wasmApi) paramsHandler(w http.ResponseWriter, r *http.Request) {
	cache, err := api.getCacheOrEnsure(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get cache", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(cache.params); err != nil {
		sklog.Errorf("Failed to write params response: %v", err)
	}
}

func (api *wasmApi) tracesHandler(w http.ResponseWriter, r *http.Request) {
	cache, err := api.getCacheOrEnsure(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get cache", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	if _, err := w.Write(cache.traces); err != nil {
		sklog.Errorf("Failed to write traces response: %v", err)
	}
}
