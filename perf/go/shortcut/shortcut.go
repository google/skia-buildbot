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
	Insert(ctx context.Context, r io.Reader) (string, error)
	InsertShortcut(ctx context.Context, shortcut *Shortcut) (string, error)
	Get(ctx context.Context, id string) (*Shortcut, error)
}
