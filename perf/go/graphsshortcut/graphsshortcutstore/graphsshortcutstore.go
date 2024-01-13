// Package graphsshortcutstore implements graphsshortcut.Store using an SQL database.

package graphsshortcutstore

import (
	"context"
	"encoding/json"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/graphsshortcut"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	insertShortcut statement = iota
	getShortcut
)

// statements holds all the raw SQL statemens.
var statements = map[statement]string{
	insertShortcut: `
		INSERT INTO
			GraphsShortcuts (id, graphs)
		VALUES
			($1, $2)
		ON CONFLICT
		DO NOTHING`,
	getShortcut: `
		SELECT
			(graphs)
		FROM
			GraphsShortcuts
		WHERE
			id=$1
		`,
}

// GraphsShortcutStore implements the graphsshortcut.Store interface using an SQL
// database.
type GraphsShortcutStore struct {
	db pool.Pool
}

// New returns a new *GraphsShortcutStore.
func New(db pool.Pool) (*GraphsShortcutStore, error) {
	return &GraphsShortcutStore{
		db: db,
	}, nil
}

// InsertShortcut implements the graphsshortcut.Store interface.
func (s *GraphsShortcutStore) InsertShortcut(ctx context.Context, sc *graphsshortcut.GraphsShortcut) (string, error) {
	id := (*sc).GetID()
	b, err := json.Marshal(sc)
	if err != nil {
		return "", err
	}
	if _, err := s.db.Exec(ctx, statements[insertShortcut], id, string(b)); err != nil {
		return "", skerr.Wrap(err)
	}
	return id, nil
}

// GetShortcut implements the graphsshortcut.Store interface.
func (s *GraphsShortcutStore) GetShortcut(ctx context.Context, id string) (*graphsshortcut.GraphsShortcut, error) {
	var encoded string
	if err := s.db.QueryRow(ctx, statements[getShortcut], id).Scan(&encoded); err != nil {
		return nil, skerr.Wrapf(err, "Failed to load shortcuts.")
	}
	var sc graphsshortcut.GraphsShortcut
	if err := json.Unmarshal([]byte(encoded), &sc); err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode keys.")
	}
	return &sc, nil
}
