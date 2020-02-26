// Package sqlshortcutstore implements shortcut.Store using an SQL database.
package sqlshortcutstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/shortcut"
	perfsql "go.skia.org/infra/perf/go/sql"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	insertShortcut = iota
	getShortcut
)

// statements allows looking up raw SQL statements by their statement id.
type statements map[statement]string

// statementsByDialect holds all the raw SQL statemens used per Dialect of SQL.
var statementsByDialect = map[perfsql.Dialect]statements{
	perfsql.SQLiteDialect: {
		insertShortcut: `
		INSERT OR IGNORE INTO
			Shortcuts (id, trace_ids)
		VALUES
			(?, ?)`,
		getShortcut: `
		SELECT
			(trace_ids)
		FROM
			Shortcuts
		WHERE
			id=?
		`,
	},
	perfsql.CockroachDBDialect: {
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
	},
}

// SQLShortcutStore implements the shortcut.Store interface using an SQL
// database.
type SQLShortcutStore struct {
	// db is the database connection.
	db *sql.DB

	// preparedStatements are all the prepared SQL statements.
	preparedStatements map[statement]*sql.Stmt
}

// New returns a new *SQLShortcutStore.
//
// We presume all migrations have been run against db before this function is
// called.
func New(db *sql.DB, dialect perfsql.Dialect) (*SQLShortcutStore, error) {
	preparedStatements := map[statement]*sql.Stmt{}
	for key, statement := range statementsByDialect[dialect] {
		prepared, err := db.Prepare(statement)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to prepare statment %v %q", key, statement)
		}
		preparedStatements[key] = prepared
	}

	return &SQLShortcutStore{
		db:                 db,
		preparedStatements: preparedStatements,
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
	id := shortcut.IDFromKeys(sc)
	b, err := json.Marshal(sc)
	if err != nil {
		return "", err
	}
	if _, err := s.preparedStatements[insertShortcut].ExecContext(ctx, id, string(b)); err != nil {
		return "", skerr.Wrap(err)
	}
	return id, nil
}

// Get implements the shortcut.Store interface.
func (s *SQLShortcutStore) Get(ctx context.Context, id string) (*shortcut.Shortcut, error) {
	var encoded string
	if err := s.preparedStatements[getShortcut].QueryRowContext(ctx, id).Scan(&encoded); err != nil {
		return nil, skerr.Wrapf(err, "Failed to load shortcuts.")
	}
	var sc shortcut.Shortcut
	if err := json.Unmarshal([]byte(encoded), &sc); err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode keys.")
	}

	return &sc, nil
}
