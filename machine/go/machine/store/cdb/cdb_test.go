package cdb_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/machinetest"
	"go.skia.org/infra/machine/go/machine/store/cdb"
	"go.skia.org/infra/machine/go/machine/store/cdb/cdbtest"
)

func Test_Statements_SprintfReturnsCorrectResults(t *testing.T) {
	require.Equal(t, `
SELECT
	maintenance_mode,is_quarantined,recovering,attached_device,annotation,note,version,powercycle,powercycle_state,last_updated,battery,temperatures,running_swarmingTask,launched_swarming,recovery_start,device_uptime,ssh_user_ip,supplied_dimensions,dimensions,task_request,task_started
FROM
	Description
WHERE
	dimensions @> CONCAT('{"id": ["', $1, '"]}')::JSONB
FOR UPDATE`, cdb.Statements[cdb.GetAndLockRow])

	require.Equal(t, `
UPSERT INTO
	Description (maintenance_mode,is_quarantined,recovering,attached_device,annotation,note,version,powercycle,powercycle_state,last_updated,battery,temperatures,running_swarmingTask,launched_swarming,recovery_start,device_uptime,ssh_user_ip,supplied_dimensions,dimensions,task_request,task_started)
VALUES
	($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
`, cdb.Statements[cdb.Update])
}

func Test_LengthsOfColumnHeadersAndCaolumnValuesAreTheSame(t *testing.T) {
	d := machine.NewDescription(context.Background())
	require.Equal(t, len(machine.DestFromDescription(&d)), len(cdb.Description))
}

const (
	machineID1 = "skia-linux-101"
	machineID2 = "skia-linux-102"
	machineID3 = "skia-linux-103"
)

func setupForTest(t *testing.T) (context.Context, *cdb.Store) {
	ctx := context.Background()
	db := cdbtest.NewCockroachDBForTests(t, "desc")
	s := cdb.New(db)
	err := s.Update(ctx, machineID1, func(in machine.Description) machine.Description {
		ret := machinetest.FullyFilledInDescription.Copy()
		ret.Dimensions[machine.DimID] = []string{machineID1}
		return ret
	})
	require.NoError(t, err)
	err = s.Update(ctx, machineID2, func(in machine.Description) machine.Description {
		ret := machinetest.FullyFilledInDescription.Copy()
		ret.Dimensions[machine.DimID] = []string{machineID2}
		ret.PowerCycle = false
		return ret
	})
	require.NoError(t, err)
	err = s.Update(ctx, machineID3, func(in machine.Description) machine.Description {
		ret := machinetest.FullyFilledInDescription.Copy()
		ret.Dimensions[machine.DimID] = []string{machineID3}
		ret.PowerCycle = true
		ret.IsQuarantined = true
		return ret
	})
	require.NoError(t, err)

	return ctx, s

}

func TestStore_UpdateAndGetFullyRoundTripTheDescription_Success(t *testing.T) {
	ctx := context.Background()
	db := cdbtest.NewCockroachDBForTests(t, "desc")
	s := cdb.New(db)
	full := machinetest.FullyFilledInDescription.Copy()

	// CockroachDB can only store times down to the millisecond, so truncate
	// what we store, so we are equal once we've round-tripped through the
	// database.
	full.LastUpdated = full.LastUpdated.Truncate(time.Millisecond)
	full.RecoveryStart = full.RecoveryStart.Truncate(time.Millisecond)
	full.TaskStarted = full.TaskStarted.Truncate(time.Millisecond)

	machineID := full.Dimensions[machine.DimID][0]
	err := s.Update(ctx, machineID, func(in machine.Description) machine.Description {
		return full
	})
	require.NoError(t, err)

	d, err := s.Get(ctx, machineID)
	require.NoError(t, err)

	assertdeep.Copy(t, d, full)
}

func TestStore_NilTaskDescriptorFullyRoundTrips_Success(t *testing.T) {
	ctx := context.Background()
	db := cdbtest.NewCockroachDBForTests(t, "desc")
	s := cdb.New(db)
	full := machinetest.FullyFilledInDescription.Copy()
	full.TaskRequest = nil

	machineID := full.Dimensions[machine.DimID][0]
	err := s.Update(ctx, machineID, func(in machine.Description) machine.Description {
		return full
	})
	require.NoError(t, err)

	d, err := s.Get(ctx, machineID)
	require.NoError(t, err)

	require.Nil(t, d.TaskRequest)
}

func TestStore_ZeroLengthDimensionsAreDiscarded_Success(t *testing.T) {
	ctx := context.Background()
	db := cdbtest.NewCockroachDBForTests(t, "desc")
	s := cdb.New(db)
	d := machine.NewDescription(ctx)
	d.Dimensions = machine.SwarmingDimensions{
		"keep":        {"a", "b"},
		"removed":     {},
		"alsoremoved": nil,
		machine.DimID: {machineID1},
	}
	d.SuppliedDimensions = machine.SwarmingDimensions{
		"keep":        {"a", "b"},
		"removed":     {},
		"alsoremoved": nil,
	}

	err := s.Update(ctx, machineID1, func(in machine.Description) machine.Description {
		return d
	})
	require.NoError(t, err)

	stored, err := s.Get(ctx, machineID1)
	require.NoError(t, err)

	expected := machine.SwarmingDimensions{
		"keep":          {"a", "b"},
		machine.DimID:   {machineID1},
		"task_type":     []string{"swarming"},
		machine.DimPool: []string{machine.PoolSkia},
	}
	require.Equal(t, expected, stored.Dimensions)
	expected = machine.SwarmingDimensions{
		"keep": {"a", "b"},
	}
	require.Equal(t, expected, stored.SuppliedDimensions)
}

func TestStore_Get_Success(t *testing.T) {
	ctx, s := setupForTest(t)
	d, err := s.Get(ctx, machineID1)
	require.NoError(t, err)
	require.Equal(t, machineID1, d.Dimensions[machine.DimID][0])
	require.True(t, d.IsQuarantined)
}

func TestStore_GetForMachineThatDoesNotExist_ReturnsError(t *testing.T) {
	ctx, s := setupForTest(t)
	_, err := s.Get(ctx, "some-machine-id-that-does-not-exist")
	require.Error(t, err)
}

func TestStore_List_Success(t *testing.T) {
	ctx, s := setupForTest(t)
	descriptions, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, descriptions, 3)
}

func TestStore_ListPowerCycle_Success(t *testing.T) {
	ctx, s := setupForTest(t)
	machines, err := s.ListPowerCycle(ctx)
	require.NoError(t, err)
	sort.Strings(machines)
	require.Equal(t, []string{machineID1, machineID3}, machines)
}

func TestStore_ListPowerCycle_NoMachinesNeedPowerCycle_ReturnsEmptyList(t *testing.T) {
	ctx, s := setupForTest(t)

	// Turn off powercycle for all machines.
	for _, name := range []string{machineID1, machineID2, machineID3} {
		err := s.Update(ctx, name, func(in machine.Description) machine.Description {
			ret := in.Copy()
			ret.PowerCycle = false
			return ret
		})
		require.NoError(t, err)
	}

	machines, err := s.ListPowerCycle(ctx)
	require.NoError(t, err)
	require.Empty(t, machines)
}

func TestStore_Delete_Success(t *testing.T) {
	ctx, s := setupForTest(t)
	err := s.Delete(ctx, machineID2)
	require.NoError(t, err)
	_, err = s.Get(ctx, machineID2)
	require.Error(t, err)
}

func TestStore_Delete_MachineDoesNotExist_Success(t *testing.T) {
	ctx, s := setupForTest(t)
	err := s.Delete(ctx, "this-machine-does-not-exist")
	require.NoError(t, err)
}

const V1Schema = `CREATE TABLE IF NOT EXISTS Description (
	maintenance_mode STRING NOT NULL DEFAULT '',
	is_quarantined BOOL NOT NULL DEFAULT FALSE,
	recovering STRING NOT NULL DEFAULT '',
	attached_device STRING NOT NULL DEFAULT 'nodevice',
	annotation JSONB NOT NULL,
	note JSONB NOT NULL,
	version STRING NOT NULL DEFAULT '',
	powercycle BOOL NOT NULL DEFAULT FALSE,
	powercycle_state STRING NOT NULL DEFAULT 'not_available',
	last_updated TIMESTAMPTZ NOT NULL,
	battery INT NOT NULL DEFAULT 0,
	temperatures JSONB NOT NULL,
	running_swarmingTask BOOL NOT NULL DEFAULT FALSE,
	launched_swarming BOOL NOT NULL DEFAULT FALSE,
	recovery_start TIMESTAMPTZ NOT NULL,
	device_uptime INT4 DEFAULT 0,
	ssh_user_ip STRING NOT NULL DEFAULT '',
	supplied_dimensions JSONB NOT NULL,
	dimensions JSONB NOT NULL,
	machine_id STRING PRIMARY KEY AS (dimensions->'id'->>0) STORED,
	INVERTED INDEX dimensions_gin (dimensions),
	INDEX by_powercycle (powercycle)
  );`

const V1ToV2 = `
ALTER TABLE Description
	ADD COLUMN IF NOT EXISTS task_request jsonb;

ALTER TABLE Description
	ADD COLUMN IF NOT EXISTS task_started timestamptz NOT NULL;
`

const indexes = `
SELECT
  indexname,
  indexdef
FROM
  pg_indexes
WHERE
  tablename = 'description'
`

const types = `
SELECT
    column_name,
    data_type
FROM
    information_schema.columns
WHERE
    table_name = 'description'
`

type schema struct {
	ColumnNameAndType map[string]string
	IndexNameAndDef   map[string]string
}

func getSchema(t *testing.T, db *pgxpool.Pool) schema {
	ctx := context.Background()
	rows, err := db.Query(ctx, types)
	require.NoError(t, err)

	colNameAndType := map[string]string{}
	for rows.Next() {
		var colName string
		var colType string
		err := rows.Scan(&colName, &colType)
		require.NoError(t, err)
		colNameAndType[colName] = colType
	}

	rows, err = db.Query(ctx, indexes)
	require.NoError(t, err)

	indexNameAndDef := map[string]string{}
	for rows.Next() {
		var indexName string
		var indexDef string
		err := rows.Scan(&indexName, &indexDef)
		require.NoError(t, err)
		indexNameAndDef[indexName] = indexDef
	}

	require.NotEmpty(t, indexNameAndDef)
	require.NotEmpty(t, colNameAndType)
	return schema{
		ColumnNameAndType: colNameAndType,
		IndexNameAndDef:   indexNameAndDef,
	}

}

func Test_V1ToV2SchemaMigration(t *testing.T) {
	ctx := context.Background()
	db := cdbtest.NewCockroachDBForTests(t, "desc")

	v2Schema := getSchema(t, db)

	_, err := db.Exec(ctx, "DROP TABLE IF EXISTS Description")
	require.NoError(t, err)

	_, err = db.Exec(ctx, V1Schema)
	require.NoError(t, err)

	_, err = db.Exec(ctx, V1ToV2)
	require.NoError(t, err)

	v1toV2Schema := getSchema(t, db)

	assertdeep.Equal(t, v2Schema, v1toV2Schema)

	// Test the test, make sure at least one known column is present.
	require.Equal(t, "text", v1toV2Schema.ColumnNameAndType["machine_id"])
}
