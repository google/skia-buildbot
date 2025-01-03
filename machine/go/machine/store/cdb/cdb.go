// Package cdb contains an implementation of ../store.Store that uses
// CockroachDB.
package cdb

//go:generate bazelisk run --config=mayberemote //:go -- run ./tosql
//go:generate bazelisk run --config=mayberemote //:go -- run ./tosql --schemaTarget spanner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/schema"
	"go.skia.org/infra/go/sql/sqlutil"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/pools"
	"go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machine/store/cdb/expectedschema"
)

const (
	// DatabaseName is the name of the database.
	DatabaseName = "machineserver"
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
	GetFreeMachines
)

var (
	descriptionAllNonComputedColumns = strings.Join(Description, ",")
)

// Statements are all the SQL statements used in Store.
//
// In the below statements you might see something like:
//
//	CONCAT('{"id": ["', $1, '"]}')::JSONB
//
// This allows building up a JSONB expression while still allowing the use of
// placeholders, like '$1'. The following will not work, since the $1 is inside
// a string and thus not substituted.
//
//	'{"id": ["$1"]}'
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
	GetFreeMachines: fmt.Sprintf(`
SELECT
	%s
FROM
	Description@by_running_task
WHERE
	running_task = FALSE
AND
	dimensions @> CONCAT('{"task_type": ["sktask"], "pool":["', $1, '"]}')::JSONB
`, descriptionAllNonComputedColumns),
}

// Tables represents all SQL tables used by machineserver.
type Tables struct {
	Description []machine.Description
	TaskResult  []machine.TaskResult
}

// Store implements ../store.Store.
type Store struct {
	db    pool.Pool
	pools *pools.Pools
}

// New returns a new *Store that uses the give Pool.
func New(ctx context.Context, db pool.Pool, pools *pools.Pools) (*Store, error) {
	// Confirm the database has the right schema.
	expectedSchema, err := expectedschema.Load()
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	actual, err := schema.GetDescription(ctx, db, Tables{}, schema.CockroachDBType)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if diff := assertdeep.Diff(expectedSchema, *actual); diff != "" {
		return nil, skerr.Fmt("Schema needs to be updated: %s.", diff)
	}

	return &Store{
		db:    db,
		pools: pools,
	}, nil
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

// wrappedError unwraps and re-wraps a pgconn.PgError to give more details on
// the failure and includes the machineID in the error message.
func wrappedErrorForID(err error, machineID string) error {
	return skerr.Wrapf(wrappedError(err), "Machine: %q", machineID)
}

// Remove dimensions that have 0 length slices for a value.
func sanitizeDimensions(in machine.SwarmingDimensions) machine.SwarmingDimensions {
	ret := machine.SwarmingDimensions{}
	for key, slice := range in {
		if len(slice) == 0 {
			continue
		}
		ret[key] = in[key]
	}
	return ret
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
			return wrappedErrorForID(err, machineID)
		}

		newD := updateCallback(d)

		newD.Dimensions = sanitizeDimensions(newD.Dimensions)
		newD.SuppliedDimensions = sanitizeDimensions(newD.SuppliedDimensions)

		// Default to a DimTaskType of Swarming if not set.
		if len(newD.Dimensions[machine.DimTaskType]) == 0 {
			newD.Dimensions[machine.DimTaskType] = []string{string(machine.Swarming)}
		}

		if !s.pools.HasValidPool(newD) {
			s.pools.SetSwarmingPool(&newD)
		}

		_ = machine.SetSwarmingQuarantinedMessage(&newD)
		SetQuarantineMetrics(newD)

		// Normalize times so they appear consistent in the database.
		newD.RecoveryStart = newD.RecoveryStart.UTC().Truncate(time.Millisecond)
		newD.LastUpdated = newD.LastUpdated.UTC().Truncate(time.Millisecond)
		newD.TaskStarted = newD.TaskStarted.UTC().Truncate(time.Millisecond)

		// Write the updated value.
		_, err = tx.Exec(ctx, Statements[Update], machine.DestFromDescription(&newD)...)
		if err != nil {
			return wrappedErrorForID(err, machineID)
		}
		return nil
	})
}

var (
	MaintenanceTag = map[string]string{"state": "Maintenance"}
	QuarantineTag  = map[string]string{"state": "Quarantined"}
	RecoveringTag  = map[string]string{"state": "Recovering"}
)

// Reflects MaintenanceMode, Quarantined, and Recovering into metrics.
func SetQuarantineMetrics(d machine.Description) {
	m := metrics2.GetBoolMetric("machine_processor_device_quarantine_state", d.Dimensions.AsMetricsTags(), MaintenanceTag)
	m.Update(d.InMaintenanceMode())

	m = metrics2.GetBoolMetric("machine_processor_device_quarantine_state", d.Dimensions.AsMetricsTags(), QuarantineTag)
	m.Update(d.IsQuarantined)

	m = metrics2.GetBoolMetric("machine_processor_device_quarantine_state", d.Dimensions.AsMetricsTags(), RecoveringTag)
	m.Update(d.IsRecovering())
}

// Get implements ../store.Store.
func (s *Store) Get(ctx context.Context, machineID string) (machine.Description, error) {
	ret := machine.NewDescription(ctx)
	ret.Dimensions[machine.DimID] = []string{machineID}
	err := s.db.QueryRow(ctx, Statements[Get], machineID).Scan(machine.DestFromDescription(&ret)...)
	if err != nil {
		return ret, wrappedErrorForID(err, machineID)
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
		return wrappedErrorForID(err, machineID)
	}
	return nil
}

// GetFreeMachines implements ../store.Store.
func (s *Store) GetFreeMachines(ctx context.Context, pool string) ([]machine.Description, error) {
	var ret []machine.Description

	rows, err := s.db.Query(ctx, Statements[GetFreeMachines], pool)
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
