package sqlshortcutstore

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/shortcut"
)

// cacheShortcutStore provides an implementation of shortcut.Store
// which stores these shortcuts in a cache instead of the database.
type cacheShortcutStore struct {
	cacheClient cache.Cache
}

// NewCacheShortcutStore returns a new instance of cacheShortcutStore.
func NewCacheShortcutStore(cacheClient cache.Cache) *cacheShortcutStore {
	return &cacheShortcutStore{
		cacheClient: cacheClient,
	}
}

func (c *cacheShortcutStore) Insert(ctx context.Context, r io.Reader) (string, error) {
	var s shortcut.Shortcut
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return "", skerr.Wrapf(err, "Failed to decode shortcut")
	}
	return c.InsertShortcut(ctx, &s)
}

func (c *cacheShortcutStore) InsertShortcut(ctx context.Context, s *shortcut.Shortcut) (string, error) {
	id := shortcut.IDFromKeys(s)

	var buff bytes.Buffer
	err := json.NewEncoder(&buff).Encode(s)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to encode shortcut")
	}

	err = c.cacheClient.SetValue(ctx, id, buff.String())
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to set value in cache")
	}

	return id, nil
}

func (c *cacheShortcutStore) Get(ctx context.Context, id string) (*shortcut.Shortcut, error) {
	value, err := c.cacheClient.GetValue(ctx, id)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get value from cache")
	}

	var s shortcut.Shortcut
	if err := json.Unmarshal([]byte(value), &s); err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode shortcut")
	}

	return &s, nil
}

func (c *cacheShortcutStore) GetAll(ctx context.Context) (<-chan *shortcut.Shortcut, error) {
	return nil, skerr.Fmt("Unimplemented")
}

func (c *cacheShortcutStore) DeleteShortcut(ctx context.Context, id string, tx pgx.Tx) error {
	return skerr.Fmt("Unimplemented")
}
