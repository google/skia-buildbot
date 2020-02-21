// shortcut handles storing and retrieving shortcuts.
package shortcut

import (
	"context"
	"io"
)

// Shortcut is a list of Trace ids, it is used in the Store interface.
type Shortcut struct {
	Keys []string `json:"keys" datastore:",noindex"`
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
}
