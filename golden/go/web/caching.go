package web

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/web/frontend"
)

// webCacheManager provides a struct to handle caching for the web handlers.
type webCacheManager struct {
	cacheClient cache.Cache
}

func baselineCacheKey(crs, clID string) string {
	return fmt.Sprintf("baseline_gz_%s_%s", crs, clID)
}

func encodeBaseline(resp frontend.BaselineV2Response) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if err := json.NewEncoder(gz).Encode(resp); err != nil {
		_ = gz.Close()
		return nil, skerr.Wrap(err)
	}
	if err := gz.Close(); err != nil {
		return nil, skerr.Wrap(err)
	}
	return buf.Bytes(), nil
}

// NewCacheManager returns a new instance of the webCacheManager.
func NewCacheManager(cacheClient cache.Cache) *webCacheManager {
	if cacheClient == nil {
		sklog.Errorf("Nil cacheClient is provided to the NewCacheManager.")
		return nil
	}
	return &webCacheManager{
		cacheClient: cacheClient,
	}
}

// GetBaseline returns the gzipped JSON baseline bytes for the given crs and clID values.
func (w *webCacheManager) GetBaseline(ctx context.Context, crs, clID string) ([]byte, error) {
	cacheKey := baselineCacheKey(crs, clID)
	gzStr, err := w.cacheClient.GetValue(ctx, cacheKey)
	if err != nil {
		return nil, err
	}

	if gzStr == "" {
		return nil, nil
	}

	// Minimum valid gzip stream is 18 bytes (10-byte header starting with 0x1f 0x8b + 8-byte trailer).
	if len(gzStr) < 18 || gzStr[0] != 0x1f || gzStr[1] != 0x8b {
		return nil, skerr.Fmt("cached baseline for %q is not valid gzip", cacheKey)
	}

	return []byte(gzStr), nil
}

// SetBaseline stores the gzipped JSON baseline bytes for the given crs and clID values.
func (w *webCacheManager) SetBaseline(ctx context.Context, crs, clID string, gzBaseline []byte, expiry time.Duration) error {
	cacheKey := baselineCacheKey(crs, clID)
	return w.cacheClient.SetValueWithExpiry(ctx, cacheKey, string(gzBaseline), expiry)
}
