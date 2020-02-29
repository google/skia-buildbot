// Package dsshortcutstore implements shortcut.Shortcut using Google Cloud Datastore.
package dsshortcutstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
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
func (s *ShortcutStoreDS) InsertShortcut(ctx context.Context, sc *shortcut.Shortcut) (string, error) {
	key := ds.NewKey(ds.SHORTCUT)
	// Prefix the hash with an X. This is a holdover from a previous storage
	// system that we keep alive so that all old shortcuts work and new ones
	// look the same.
	key.Name = shortcut.IDFromKeys(sc)
	var err error
	key, err = ds.DS.Put(ctx, key, sc)
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

// getAllBatchSize is the batch size we for retrieving Shortcuts from the datastore.
const getAllBatchSize = 10

// GetAll implements the shortcut.Store interface.
func (s *ShortcutStoreDS) GetAll(ctx context.Context) (<-chan *shortcut.Shortcut, error) {
	ret := make(chan *shortcut.Shortcut)

	go func() {
		defer close(ret)
		var offset int
		var err error
		// Cloud Datatstore doesn't offer a streaming interface for query
		// responses, so we will request small batches using GetAll and then
		// move through the entire datastore via the offset.
		for err == nil {
			queryResults := []*shortcut.Shortcut{}
			q := ds.NewQuery(ds.SHORTCUT).Offset(offset).Limit(getAllBatchSize)
			keys, err := ds.DS.GetAll(ctx, q, &queryResults)
			if err != nil {
				sklog.Warningf("Error retrieving all shortcuts: %s", err)
				return
			}
			for i := range keys {
				ret <- queryResults[i]
			}
			if len(keys) < getAllBatchSize {
				return
			}
			offset += getAllBatchSize
		}
	}()
	return ret, nil
}

// Confirm that ShortcutStoreDS implements shortcut.Store.
var _ shortcut.Store = (*ShortcutStoreDS)(nil)
