// Package cdb contains an implementation of ../store.Store that uses
// CockroachDB.
package cdb

//go:generate bazelisk run --config=mayberemote //:go -- run ./tosql --output_file sql.go --output_pkg cdb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/sqlutil"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/store"
)

// statement is an SQL statement or fragment of an SQL statement.
type statement int

// All the different statements we need. Each statement will appear in Statements.
const (
	GetAndLockRow statement = iota
	Update
	Get
	ListPowerCycle
	List
	Delete
)

var (
	descriptionAllNonComputedColumns = strings.Join(Description, ",")
)

// Statements are all the SQL statements used in Store.
//
// In the below statements you might see something like:
//
//    CONCAT('{"id": ["', $1, '"]}')::JSONB
//
// This allows building up a JSONB expression while still allowing the use of
// placeholders, like '$1'. The following will not work, since the $1 is inside
// a string and thus not substituted.
//
//    '{"id": ["$1"]}'
//
var Statements = map[statement]string{
	GetAndLockRow: fmt.Sprintf(`
SELECT
	%s
FROM
	Description
WHERE
	dimensions @> CONCAT('{"id": ["', $1, '"]}')::JSONB
FOR UPDATE`, descriptionAllNonComputedColumns),
	Update: fmt.Sprintf(`
UPSERT INTO
	Description (%s)
VALUES
	%s
`, descriptionAllNonComputedColumns, sqlutil.ValuesPlaceholders(len(Description), 1),
	),
	Get: fmt.Sprintf(`
SELECT
	%s
FROM
	Description
WHERE
	dimensions @> CONCAT('{"id": ["', $1, '"]}')::JSONB
`, descriptionAllNonComputedColumns),
	ListPowerCycle: `
SELECT
	machine_id
FROM
	Description@by_powercycle
WHERE
	powercycle = TRUE
`,
	List: fmt.Sprintf(`
SELECT
	%s
FROM
	Description
`, descriptionAllNonComputedColumns),
	Delete: `
DELETE FROM
	Description
WHERE
	dimensions @> CONCAT('{"id": ["', $1, '"]}')::JSONB
`,
}

// Tables represents all SQL tables used by machineserver.
type Tables struct {
	Description []machine.Description
}

// Store implements ../store.Store.
type Store struct {
	db *pgxpool.Pool
}

// New returns a new *Store that uses the give Pool.
func New(db *pgxpool.Pool) *Store {
	return &Store{
		db: db,
	}
}

// wrappedError unwraps and re-wraps a pgconn.PgError to give more details on
// the failure.
func wrappedError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return skerr.Wrapf(err, "Mgs: %s, Code: %s, Detail: %s, Hint: %s", pgErr.Message, pgErr.Code, pgErr.Detail, pgErr.Hint)
	}
	return skerr.Wrap(err)
}

// Update implements ../store.Store.
func (s *Store) Update(ctx context.Context, machineID string, updateCallback store.UpdateCallback) error {
	return s.db.BeginFunc(ctx, func(tx pgx.Tx) error {
		// Load the current machine description, if one already exists.
		d := machine.NewDescription(ctx)
		d.Dimensions[machine.DimID] = []string{machineID}
		err := tx.QueryRow(ctx, Statements[GetAndLockRow], machineID).Scan(machine.DestFromDescription(&d)...)
		// Not finding any rows is fine, but all other errors should return.
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return wrappedError(err)
		}

		newD := updateCallback(d)

		// Normalize times so they appear consistent in the database.
		newD.RecoveryStart = newD.RecoveryStart.UTC().Truncate(time.Millisecond)
		newD.LastUpdated = newD.LastUpdated.UTC().Truncate(time.Millisecond)

		// Write the updated value.
		_, err = tx.Exec(ctx, Statements[Update], machine.DestFromDescription(&newD)...)
		if err != nil {
			return wrappedError(err)
		}
		return nil
	})
}

// Get implements ../store.Store.
func (s *Store) Get(ctx context.Context, machineID string) (machine.Description, error) {
	ret := machine.NewDescription(ctx)
	ret.Dimensions[machine.DimID] = []string{machineID}
	err := s.db.QueryRow(ctx, Statements[Get], machineID).Scan(machine.DestFromDescription(&ret)...)
	if err != nil {
		return ret, wrappedError(err)
	}
	ret.RecoveryStart = ret.RecoveryStart.UTC().Truncate(time.Millisecond)
	ret.LastUpdated = ret.LastUpdated.UTC().Truncate(time.Millisecond)

	return ret, nil
}

// ListPowerCycle implements ../store.Store.
func (s *Store) ListPowerCycle(ctx context.Context) ([]string, error) {
	var ret []string

	rows, err := s.db.Query(ctx, Statements[ListPowerCycle])
	if err != nil {
		return nil, wrappedError(err)
	}

	var machineID string
	for rows.Next() {
		err := rows.Scan(&machineID)
		if err != nil {
			return nil, wrappedError(err)
		}
		ret = append(ret, machineID)
	}

	return ret, nil
}

// List implements ../store.Store.
func (s *Store) List(ctx context.Context) ([]machine.Description, error) {
	var ret []machine.Description

	rows, err := s.db.Query(ctx, Statements[List])
	if err != nil {
		return nil, wrappedError(err)
	}

	for rows.Next() {
		d := machine.NewDescription(ctx)
		err := rows.Scan(machine.DestFromDescription(&d)...)
		if err != nil {
			return nil, wrappedError(err)
		}
		ret = append(ret, d)
	}

	return ret, nil
}

// Delete implements ../store.Store.
func (s *Store) Delete(ctx context.Context, machineID string) error {
	if _, err := s.db.Exec(ctx, Statements[Delete], machineID); err != nil {
		return wrappedError(err)
	}
	return nil
}
