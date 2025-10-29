// Package sqlshortcutstore implements shortcut.Store using an SQL database.
package sqlshortcutstore

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/shortcut"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	insertShortcut statement = iota
	getShortcut
	getAllShortcuts
	deleteShortcut
)

// statements holds all the raw SQL statemens.
var statements = map[statement]string{
	insertShortcut: `
		INSERT INTO
			Shortcuts (id, trace_ids)
		VALUES
			($1, $2)
		ON CONFLICT (id)
		DO NOTHING`,
	getShortcut: `
		SELECT
			(trace_ids)
		FROM
			Shortcuts
		WHERE
			id=$1
		`,
	getAllShortcuts: `
		SELECT
			(trace_ids)
		FROM
			Shortcuts
		`,
	deleteShortcut: `
		DELETE
		FROM
			Shortcuts
		WHERE
			id=$1
	`,
}

// SQLShortcutStore implements the shortcut.Store interface using an SQL
// database.
type SQLShortcutStore struct {
	db pool.Pool
}

// New returns a new *SQLShortcutStore.
func New(db pool.Pool) (*SQLShortcutStore, error) {
	return &SQLShortcutStore{
		db: db,
	}, nil
}

// Insert implements the shortcut.Store interface.
func (s *SQLShortcutStore) Insert(ctx context.Context, r io.Reader) (string, error) {
	shortcut := &shortcut.Shortcut{}
	if err := json.NewDecoder(r).Decode(shortcut); err != nil {
		return "", skerr.Wrapf(err, "Unable to read shortcut body")
	}
	return s.InsertShortcut(ctx, shortcut)
}

// InsertShortcut implements the shortcut.Store interface.
func (s *SQLShortcutStore) InsertShortcut(ctx context.Context, sc *shortcut.Shortcut) (string, error) {
	for _, key := range sc.Keys {
		if !query.IsValid(key) {
			return "", skerr.Fmt("Tried to store an invalid trace key: %q", key)
		}
	}
	id := shortcut.IDFromKeys(sc)
	var buff bytes.Buffer
	err := json.NewEncoder(&buff).Encode(sc)
	if err != nil {
		return "", err
	}
	if _, err := s.db.Exec(ctx, statements[insertShortcut], id, buff.String()); err != nil {
		return "", skerr.Wrap(err)
	}
	return id, nil
}

// Get implements the shortcut.Store interface.
func (s *SQLShortcutStore) Get(ctx context.Context, id string) (*shortcut.Shortcut, error) {
	var encoded string
	if err := s.db.QueryRow(ctx, statements[getShortcut], id).Scan(&encoded); err != nil {
		return nil, skerr.Wrapf(err, "Failed to load shortcuts.")
	}
	var sc shortcut.Shortcut
	if err := json.Unmarshal([]byte(encoded), &sc); err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode keys.")
	}

	return &sc, nil
}

// GetAll implements the shortcut.Store interface.
func (s *SQLShortcutStore) GetAll(ctx context.Context) (<-chan *shortcut.Shortcut, error) {
	ret := make(chan *shortcut.Shortcut)

	rows, err := s.db.Query(ctx, statements[getAllShortcuts])
	if err != nil {
		return ret, skerr.Wrapf(err, "Failed to query for all shortcuts.")
	}

	go func() {
		defer close(ret)

		var encoded string
		for rows.Next() {
			if err := rows.Scan(&encoded); err != nil {
				sklog.Warningf("Failed to load all shortcuts: %s", err)
				continue
			}
			var sc shortcut.Shortcut
			if err := json.Unmarshal([]byte(encoded), &sc); err != nil {
				sklog.Warningf("Failed to decode all shortcuts: %s", err)
				continue
			}
			ret <- &sc
		}
	}()

	return ret, nil
}

// DeleteShortcut implements the shortcut.Store interface.
func (s *SQLShortcutStore) DeleteShortcut(ctx context.Context, id string, tx pgx.Tx) error {
	var err error
	if tx == nil {
		_, err = s.db.Exec(ctx, statements[deleteShortcut], id)
	} else {
		_, err = tx.Exec(ctx, statements[deleteShortcut], id)
	}

	return skerr.Wrap(err)
}
