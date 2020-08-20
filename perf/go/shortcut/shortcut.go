// Package shortcut handles storing and retrieving shortcuts.
package shortcut

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"sort"
)

// Shortcut is a list of Trace ids, it is used in the Store interface.
type Shortcut struct {
	Keys []string `json:"keys"`
}

// Store is an interface for things that persists Shortcuts.
type Store interface {
	// Insert adds the shortcut content into the database. The id of the
	// shortcut is returned.
	Insert(ctx context.Context, r io.Reader) (string, error)

	// InsertShortcut adds the shortcut content into the database. The id of the
	// shortcut is returned.
	InsertShortcut(ctx context.Context, shortcut *Shortcut) (string, error)

	// Get retrieves a parsed shortcut for the given id.
	Get(ctx context.Context, id string) (*Shortcut, error)

	// GetAll returns a channel that provides all the Shortcuts stored. This is
	// used to migrate between backends.
	GetAll(ctx context.Context) (<-chan *Shortcut, error)
}

// IDFromKeys returns a unique ID for the set of keys found
// in the given Shortcut.
func IDFromKeys(s *Shortcut) string {
	if s == nil {
		return "X"
	}
	sort.Strings(s.Keys)
	h := md5.New()
	for _, s := range s.Keys {
		_, _ = io.WriteString(h, s)
	}

	// Prefix the hash with an X. This is a holdover from a previous storage
	// system that we keep alive so that all old shortcuts work and new ones
	// look the same.
	return fmt.Sprintf("X%x", h.Sum(nil))
}
