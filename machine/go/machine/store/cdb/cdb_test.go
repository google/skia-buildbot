package cdb_test

import (
	"context"
	"sort"
	"testing"
	"time"

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
	maintenance_mode,is_quarantined,recovering,attached_device,annotation,note,version,powercycle,powercycle_state,last_updated,battery,temperatures,running_swarmingTask,launched_swarming,recovery_start,device_uptime,ssh_user_ip,supplied_dimensions,dimensions
FROM
	Description
WHERE
	dimensions @> CONCAT('{"id": ["', $1, '"]}')::JSONB
FOR UPDATE`, cdb.Statements[cdb.GetAndLockRow])

	require.Equal(t, `
UPSERT INTO
	Description (maintenance_mode,is_quarantined,recovering,attached_device,annotation,note,version,powercycle,powercycle_state,last_updated,battery,temperatures,running_swarmingTask,launched_swarming,recovery_start,device_uptime,ssh_user_ip,supplied_dimensions,dimensions)
VALUES
	($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
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

	machineID := full.Dimensions[machine.DimID][0]
	err := s.Update(ctx, machineID, func(in machine.Description) machine.Description {
		return full
	})
	require.NoError(t, err)

	d, err := s.Get(ctx, machineID)
	require.NoError(t, err)

	assertdeep.Copy(t, d, full)
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
