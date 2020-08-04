// Package sqlshortcutstore implements shortcut.Store using an SQL database.
//
// Please see perf/sql/migrations for the database schema used.
package sqlshortcutstore

import (
	"context"
	"encoding/json"
	"io"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/shortcut"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	insertShortcut statement = iota
	getShortcut
	getAllShortcuts
)

// statements holds all the raw SQL statemens.
var statements = map[statement]string{
	insertShortcut: `
		INSERT INTO
			Shortcuts (id, trace_ids)
		VALUES
			($1, $2)
		ON CONFLICT
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
}

// SQLShortcutStore implements the shortcut.Store interface using an SQL
// database.
type SQLShortcutStore struct {
	db *pgxpool.Pool
}

// New returns a new *SQLShortcutStore.
//
// We presume all migrations have been run against db before this function is
// called.
func New(db *pgxpool.Pool) (*SQLShortcutStore, error) {
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
		if !query.ValidateKey(key) {
			return "", skerr.Fmt("Tried to store an invalid trace key: %q", key)
		}
	}
	id := shortcut.IDFromKeys(sc)
	b, err := json.Marshal(sc)
	if err != nil {
		return "", err
	}
	if _, err := s.db.Exec(ctx, statements[insertShortcut], id, string(b)); err != nil {
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
