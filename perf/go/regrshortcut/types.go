package regrshortcut

import (
	"context"
	"database/sql"
)

// Store persists regression shortcuts
type Store interface {
	// Create creates and saves a new shortcut for a list of regressions.
	// Returns gracefully in case when an entry for the list already exists.
	Create(ctx context.Context, regressionIdList []string) (string, error)

	// Get returns a list of regression IDs associated with a shortcut.
	Get(ctx context.Context, shortcut string) (sql.NullBool, []string, error)
}
