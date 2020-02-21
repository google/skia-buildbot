// Package dsshortcutstore implements shortcut.Shortcut using Google Cloud Datastore.
package dsshortcutstore

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/perf/go/shortcut"
)

// ShortcutStoreDS implements shortcut.Store.
type ShortcutStoreDS struct {
}

// New returns a new *ShortcutStoreDS.
func New() *ShortcutStoreDS {
	return &ShortcutStoreDS{}
}

// Insert implements the shortcut.Store interface.
func (s *ShortcutStoreDS) Insert(ctx context.Context, r io.Reader) (string, error) {
	shortcut := &shortcut.Shortcut{}
	if err := json.NewDecoder(r).Decode(shortcut); err != nil {
		return "", fmt.Errorf("Unable to read shortcut body: %s", err)
	}
	return s.InsertShortcut(ctx, shortcut)
}

// InsertShortcut implements the shortcut.Store interface.
func (s *ShortcutStoreDS) InsertShortcut(ctx context.Context, shortcut *shortcut.Shortcut) (string, error) {
	sort.Strings(shortcut.Keys)
	h := md5.New()
	for _, s := range shortcut.Keys {
		_, _ = io.WriteString(h, s)
	}

	key := ds.NewKey(ds.SHORTCUT)
	// Prefix the hash with an X. This is a holdover from a previous storage
	// system that we keep alive so that all old shortcuts work and new ones
	// look the same.
	key.Name = fmt.Sprintf("X%x", h.Sum(nil))
	var err error
	key, err = ds.DS.Put(ctx, key, shortcut)
	if err != nil {
		return "", fmt.Errorf("Failed to store shortcut: %s", err)
	}
	return key.Name, nil
}

// Get implements the shortcut.Store interface.
func (s *ShortcutStoreDS) Get(ctx context.Context, id string) (*shortcut.Shortcut, error) {
	ret := &shortcut.Shortcut{}

	key := ds.NewKey(ds.SHORTCUT)
	if strings.HasPrefix(id, "X") {
		key.Name = id
	} else {
		i, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Error invalid id: %s", id)
		}
		key.ID = i
	}
	if err := ds.DS.Get(ctx, key, ret); err != nil {
		return nil, fmt.Errorf("Error retrieving shortcut from db: %s", err)
	}
	return ret, nil
}

// Confirm that ShortcutStoreDS implements shortcut.Store.
var _ shortcut.Store = (*ShortcutStoreDS)(nil)
