package web

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/web/frontend"
)

// webCacheManager provides a struct to handle caching for the web handlers.
type webCacheManager struct {
	cacheClient cache.Cache
}

func baselineCacheKey(crs, clID string) string {
	return fmt.Sprintf("baseline_%s_%s", crs, clID)
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

// GetBaseline returns the baseline data for the given crs and clID values.
func (w *webCacheManager) GetBaseline(ctx context.Context, crs, clID string) (*frontend.BaselineV2Response, error) {
	cacheKey := baselineCacheKey(crs, clID)
	respJson, err := w.cacheClient.GetValue(ctx, cacheKey)
	if err != nil {
		return nil, err
	}

	if respJson == "" {
		return nil, nil
	}

	var resp frontend.BaselineV2Response
	if err := json.Unmarshal([]byte(respJson), &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// SetBaseline sets the baseline data for the given crs and clID values.
func (w *webCacheManager) SetBaseline(ctx context.Context, crs, clID string, resp frontend.BaselineV2Response, expiry time.Duration) error {
	respJson, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	cacheKey := baselineCacheKey(crs, clID)
	return w.cacheClient.SetValueWithExpiry(ctx, cacheKey, string(respJson), expiry)
}
