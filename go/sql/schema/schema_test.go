package schema

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/emulators/cockroachdb_instance"
)

const databaseNamePrefix = "schema"

type Machines struct{}
type Tasks struct{}

type Tables struct {
	Machines []Machines
	Tasks    []Tasks
}

func TestTableNames_NonEmpytTables_ReturnsTableNames(t *testing.T) {
	require.Equal(t, []string{"machines", "tasks"}, TableNames(Tables{}))
}

func TestTableNames_EmpytTables_ReturnsEmptySlice(t *testing.T) {
	require.Empty(t, TableNames(struct{}{}))
}

const schema = `CREATE TABLE IF NOT EXISTS Machines (
	dimensions JSONB NOT NULL,
	task_request JSONB,
	task_started TIMESTAMPTZ NOT NULL DEFAULT (0)::TIMESTAMPTZ,
	machine_id STRING PRIMARY KEY AS (dimensions->'id'->>0) STORED,
	running_task bool AS (task_request IS NOT NULL) STORED,
	INVERTED INDEX dimensions_gin (dimensions),
	INDEX by_running_task (running_task)
  );

  CREATE TABLE IF NOT EXISTS Tasks (
	result JSONB NOT NULL,
	id STRING NOT NULL PRIMARY KEY,
	machine_id STRING NOT NULL,
	finished TIMESTAMPTZ NOT NULL,
	status STRING NOT NULL DEFAULT '',
	INDEX by_machine_id (machine_id),
	INDEX by_status (status)
  );
`

func setupForTest(t *testing.T) *pgxpool.Pool {
	cockroachdb_instance.Require(t)

	rand.Seed(time.Now().UnixNano())
	databaseName := fmt.Sprintf("%s_%d", databaseNamePrefix, rand.Uint64())
	host := emulators.GetEmulatorHostEnvVar(emulators.CockroachDB)
	connectionString := fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", host, databaseName)

	ctx := context.Background()
	db, err := pgxpool.Connect(ctx, connectionString)
	require.NoError(t, err)

	// Create a database in cockroachdb just for this test.
	_, err = db.Exec(ctx, fmt.Sprintf(`
		CREATE DATABASE %s;
		SET DATABASE = %s;`, databaseName, databaseName))
	require.NoError(t, err)

	_, err = db.Exec(ctx, schema)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})
	return db

}

func TestGetDescription(t *testing.T) {
	db := setupForTest(t)
	desc, err := GetDescription(db, Tables{})
	require.NoError(t, err)

	expected := Description{
		ColumnNameAndType: map[string]string{
			"machines.dimensions":   "jsonb def: nullable:NO",
			"machines.machine_id":   "text def: nullable:NO",
			"machines.running_task": "boolean def: nullable:YES",
			"machines.task_request": "jsonb def: nullable:YES",
			"machines.task_started": "timestamp with time zone def:0:::INT8::TIMESTAMPTZ nullable:NO",
			"tasks.finished":        "timestamp with time zone def: nullable:NO",
			"tasks.id":              "text def: nullable:NO",
			"tasks.machine_id":      "text def: nullable:NO",
			"tasks.result":          "jsonb def: nullable:NO",
			"tasks.status":          "text def:'':::STRING nullable:NO",
		},
		IndexNames: []string{
			"machines.dimensions_gin",
			"machines.by_running_task",
			"tasks.by_status",
			"tasks.by_machine_id",
		},
	}

	require.Equal(t, expected, *desc)
}
