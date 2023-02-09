package cdb_test

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/machinetest"
	"go.skia.org/infra/machine/go/machine/pools"
	"go.skia.org/infra/machine/go/machine/pools/poolstest"
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

	pool = "Skia"
)

func setupForTest(t *testing.T) (context.Context, *cdb.Store) {
	ctx := context.Background()
	db := cdbtest.NewCockroachDBForTests(t, "desc")
	p, err := pools.New(poolstest.PoolConfigForTesting)
	require.NoError(t, err)
	s := cdb.New(db, p)
	err = s.Update(ctx, machineID1, func(in machine.Description) machine.Description {
		ret := machinetest.FullyFilledInDescription.Copy()
		ret.Dimensions[machine.DimID] = []string{machineID1}
		ret.Dimensions[machine.DimPool] = []string{pool}
		ret.Dimensions[machine.DimTaskType] = []string{string(machine.SkTask)}
		return ret
	})
	require.NoError(t, err)
	err = s.Update(ctx, machineID2, func(in machine.Description) machine.Description {
		ret := machinetest.FullyFilledInDescription.Copy()
		ret.Dimensions[machine.DimID] = []string{machineID2}
		ret.Dimensions[machine.DimPool] = []string{pool}
		ret.Dimensions[machine.DimTaskType] = []string{string(machine.SkTask)}
		ret.PowerCycle = false
		return ret
	})
	require.NoError(t, err)
	err = s.Update(ctx, machineID3, func(in machine.Description) machine.Description {
		ret := machinetest.FullyFilledInDescription.Copy()
		ret.Dimensions[machine.DimID] = []string{machineID3}
		ret.Dimensions[machine.DimPool] = []string{pool}
		ret.Dimensions[machine.DimTaskType] = []string{string(machine.SkTask)}
		ret.PowerCycle = true
		ret.IsQuarantined = true
		return ret
	})
	require.NoError(t, err)

	return ctx, s

}

func setupForTestWithEmptyStore(t *testing.T) (context.Context, *cdb.Store, machine.Description) {
	ctx := context.Background()
	db := cdbtest.NewCockroachDBForTests(t, "desc")
	p, err := pools.New(poolstest.PoolConfigForTesting)
	require.NoError(t, err)
	s := cdb.New(db, p)
	full := machinetest.FullyFilledInDescription.Copy()
	return ctx, s, full
}

func TestStore_UpdateAndGetFullyRoundTripTheDescription_Success(t *testing.T) {
	ctx, s, full := setupForTestWithEmptyStore(t)

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
	ctx, s, full := setupForTestWithEmptyStore(t)
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
	ctx, s, _ := setupForTestWithEmptyStore(t)
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

func TestGetFreeMachines_AllMachinesRunningTasks_ReturnsZeroMatches(t *testing.T) {
	// The added machines in setupForTest are running tasks, so this should return 0 machines.
	ctx, s := setupForTest(t)
	descriptions, err := s.GetFreeMachines(ctx, "Skia")
	require.NoError(t, err)
	require.Len(t, descriptions, 0)
}

func clearRunningTest(t *testing.T, ctx context.Context, s *cdb.Store, machineID string) machine.Description {
	var ret machine.Description
	err := s.Update(ctx, machineID, func(in machine.Description) machine.Description {
		ret = in.Copy()
		ret.TaskRequest = nil
		return ret
	})
	require.NoError(t, err)
	return ret
}

func setTaskTypeAndPool(t *testing.T, ctx context.Context, s *cdb.Store, machineID, pool string, taskType machine.TaskRequestor) machine.Description {
	var ret machine.Description
	err := s.Update(ctx, machineID, func(in machine.Description) machine.Description {
		ret = in.Copy()
		ret.Dimensions[machine.DimTaskType] = []string{string(taskType)}
		ret.Dimensions[machine.DimPool] = []string{pool}
		return ret
	})
	require.NoError(t, err)
	return ret
}

func TestGetFreeMachines_OneMachineNotRunningTasks_ReturnsMatchingMachine(t *testing.T) {
	ctx, s := setupForTest(t)

	expected := clearRunningTest(t, ctx, s, machineID2)
	descriptions, err := s.GetFreeMachines(ctx, "Skia")
	require.NoError(t, err)
	require.Len(t, descriptions, 1)
	deepequal.DeepEqual(expected, descriptions[0])
}

func TestGetFreeMachines_TwoMachinesNotRunningTasksOnlyOneInTheRightPool_ReturnsMatchingMachine(t *testing.T) {
	ctx, s := setupForTest(t)

	// machineID2 should match.
	expected := clearRunningTest(t, ctx, s, machineID2)

	// We also clear the running test from machineID1, but changing the pool
	// with cause it to not match.
	_ = clearRunningTest(t, ctx, s, machineID1)
	_ = setTaskTypeAndPool(t, ctx, s, machineID1, "SkiaInternal", machine.SkTask)
	descriptions, err := s.GetFreeMachines(ctx, "Skia")
	require.NoError(t, err)
	require.Len(t, descriptions, 1)
	deepequal.DeepEqual(expected, descriptions[0])
}

func TestGetFreeMachines_TwoMachinesNotRunningTasksOnlyOneWithTheRightTaskType_ReturnsMatchingMachine(t *testing.T) {
	ctx, s := setupForTest(t)

	// machineID3 should match.
	expected := clearRunningTest(t, ctx, s, machineID3)

	// Also clear machineID2, but change the task_type, which should cause it to
	// no longer match.
	_ = clearRunningTest(t, ctx, s, machineID2)
	_ = setTaskTypeAndPool(t, ctx, s, machineID2, pool, machine.Swarming)

	descriptions, err := s.GetFreeMachines(ctx, "Skia")
	require.NoError(t, err)
	require.Len(t, descriptions, 1)
	deepequal.DeepEqual(expected, descriptions[0])
}

const LiveSchema = `CREATE TABLE IF NOT EXISTS Description (
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
	task_request JSONB,
	task_started TIMESTAMPTZ NOT NULL DEFAULT (0)::TIMESTAMPTZ,
	machine_id STRING PRIMARY KEY AS (dimensions->'id'->>0) STORED,
	INVERTED INDEX dimensions_gin (dimensions),
	INDEX by_powercycle (powercycle)
  );`

const FromLiveToNext = `
ALTER TABLE Description
	ADD COLUMN IF NOT EXISTS running_task bool AS (task_request IS NOT NULL) STORED;

CREATE INDEX by_running_task ON Description (running_task);

CREATE TABLE IF NOT EXISTS TaskResult (
	result JSONB NOT NULL,
	id STRING PRIMARY KEY NOT NULL,
	machine_id STRING NOT NULL,
	finished TIMESTAMPTZ NOT NULL,
	status STRING NOT NULL DEFAULT '',
	INDEX by_machine_id (machine_id),
	INDEX by_status (status)
  );
`

// Returns all expected table names in a string with table names in single
// quotes and separated by commas, appropriate for using in an SQL query.
//
// For example:
//     "'description', 'taskresult'"
func tableNames() string {
	tableNames := []string{}
	for _, structField := range reflect.VisibleFields(reflect.TypeOf(cdb.Tables{})) {
		tableNames = append(tableNames, fmt.Sprintf("'%s'", strings.ToLower(structField.Name)))
	}
	return strings.Join(tableNames, ",")
}

// Query to return the indexes for all tables.
var indexes = fmt.Sprintf(`
SELECT
  indexname,
  indexdef
FROM
  pg_indexes
WHERE
  tablename IN (%s);
`, tableNames())

// Query to return the types for each column in all tables.
var types = fmt.Sprintf(`
SELECT
    column_name,
    data_type
FROM
    information_schema.columns
WHERE
    table_name IN (%s);
`, tableNames())

// schema describes the schema for all tables.
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

func Test_LiveToNextSchemaMigration(t *testing.T) {
	ctx := context.Background()
	db := cdbtest.NewCockroachDBForTests(t, "desc")

	v2Schema := getSchema(t, db)

	_, err := db.Exec(ctx, "DROP TABLE IF EXISTS Description")
	require.NoError(t, err)
	_, err = db.Exec(ctx, "DROP TABLE IF EXISTS TaskResult")
	require.NoError(t, err)

	_, err = db.Exec(ctx, LiveSchema)
	require.NoError(t, err)

	_, err = db.Exec(ctx, FromLiveToNext)
	require.NoError(t, err)

	v1toV2Schema := getSchema(t, db)

	assertdeep.Equal(t, v2Schema, v1toV2Schema)

	// Test the test, make sure at least one known column is present.
	require.Equal(t, "text", v1toV2Schema.ColumnNameAndType["machine_id"])
}

func TestSetQuarantineMetrics_Success(t *testing.T) {

	tests := []struct {
		name                string
		desc                machine.Description
		expectedMaintenance int64
		expectedRecovering  int64
		expectedQuarantined int64
	}{
		{
			name: "Machine is available",
			desc: machine.Description{
				MaintenanceMode: "",
				Recovering:      "",
				IsQuarantined:   false,
			},
			expectedMaintenance: 0,
			expectedRecovering:  0,
			expectedQuarantined: 0,
		},
		{
			name: "Manually put into maintenance mode.",
			desc: machine.Description{
				MaintenanceMode: "alice@example.com",
				Recovering:      "",
				IsQuarantined:   false,
			},
			expectedMaintenance: 1,
			expectedRecovering:  0,
			expectedQuarantined: 0,
		},
		{
			name: "Machine is recovering",
			desc: machine.Description{
				MaintenanceMode: "",
				Recovering:      "Too hot.",
				IsQuarantined:   false,
			},
			expectedMaintenance: 0,
			expectedRecovering:  1,
			expectedQuarantined: 0,
		},
		{
			name: "Machine was quarantined by failing an infra step",
			desc: machine.Description{
				MaintenanceMode: "",
				Recovering:      "",
				IsQuarantined:   true,
			},
			expectedMaintenance: 0,
			expectedRecovering:  0,
			expectedQuarantined: 1,
		},
		{
			name: "Machine has multiple reasons for being quarantined",
			desc: machine.Description{
				MaintenanceMode: "bob@example.com",
				Recovering:      "Low charge.",
				IsQuarantined:   true,
			},
			expectedMaintenance: 1,
			expectedRecovering:  1,
			expectedQuarantined: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cdb.SetQuarantineMetrics(tt.desc)
			require.Equal(t, tt.expectedMaintenance, metrics2.GetInt64Metric("machine_processor_device_quarantine_state", tt.desc.Dimensions.AsMetricsTags(), cdb.MaintenanceTag).Get())
			require.Equal(t, tt.expectedRecovering, metrics2.GetInt64Metric("machine_processor_device_quarantine_state", tt.desc.Dimensions.AsMetricsTags(), cdb.RecoveringTag).Get())
			require.Equal(t, tt.expectedQuarantined, metrics2.GetInt64Metric("machine_processor_device_quarantine_state", tt.desc.Dimensions.AsMetricsTags(), cdb.QuarantineTag).Get())
		})
	}
}
