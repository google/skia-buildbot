package graphsshortcutstore

import (
	"context"
	"encoding/json"

	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/graphsshortcut"
)

// cacheGraphsShortcutStore provides an implementation of graphsshortcut.Store
// which stores these shortcuts in a cache instead of the database.
// The primary use case is when we connect to prod database on a local instance,
// using multigraph needs write access to the database in order to write the
// graphsshortcuts. This store prevents the need to elevate to breakglass by
// using a local cache to store this data.
type cacheGraphsShortcutStore struct {
	cacheClient cache.Cache
}

// NewCacheGraphsShortcutStore returns a new instance of cacheGraphsShortcutStore.
func NewCacheGraphsShortcutStore(cacheClient cache.Cache) *cacheGraphsShortcutStore {
	return &cacheGraphsShortcutStore{
		cacheClient: cacheClient,
	}
}

// InsertShortcut inserts the given shortcut into the cache.
func (s *cacheGraphsShortcutStore) InsertShortcut(ctx context.Context, shortcut *graphsshortcut.GraphsShortcut) (string, error) {
	id := (*shortcut).GetID()
	b, err := json.Marshal(shortcut)
	if err != nil {
		return "", err
	}
	err = s.cacheClient.SetValue(ctx, id, string(b))
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetShortcut returns the shortcut matching the id from the cache.
func (s *cacheGraphsShortcutStore) GetShortcut(ctx context.Context, id string) (*graphsshortcut.GraphsShortcut, error) {
	jsonFromCache, err := s.cacheClient.GetValue(ctx, id)
	if err != nil {
		return nil, err
	}
	var sc graphsshortcut.GraphsShortcut
	if err := json.Unmarshal([]byte(jsonFromCache), &sc); err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode json from cache.")
	}

	return &sc, nil
}
