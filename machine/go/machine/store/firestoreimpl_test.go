package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/emulators/gcp_emulator"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machineserver/config"
)

func TestConvertDescription_NoDimensions(t *testing.T) {
	d := machine.NewDescription(now.TimeTravelingContext(fakeTime))
	m := convertDescription(d)
	assert.Equal(t, storeDescription{
		AttachedDevice: machine.AttachedDeviceNone,
		LastUpdated:    fakeTime,
		Dimensions:     machine.SwarmingDimensions{},
	}, m)
}

func TestConvertDescription_WithDimensions(t *testing.T) {
	d := machine.NewDescription(now.TimeTravelingContext(fakeTime))
	d.AttachedDevice = machine.AttachedDeviceAdb
	d.Dimensions = machine.SwarmingDimensions{
		machine.DimOS:          []string{"Android"},
		machine.DimDeviceType:  []string{"sailfish"},
		machine.DimQuarantined: []string{"Device sailfish too hot."},
	}
	expectedDims := d.Dimensions.Copy()

	m := convertDescription(d)
	assert.Equal(t, storeDescription{
		AttachedDevice: machine.AttachedDeviceAdb,
		Dimensions:     expectedDims,
		LastUpdated:    fakeTime,
	}, m)
}

func TestConvertDescription_WithPowerCycle(t *testing.T) {
	d := machine.NewDescription(now.TimeTravelingContext(fakeTime))
	d.AttachedDevice = machine.AttachedDeviceAdb
	d.Dimensions = machine.SwarmingDimensions{
		machine.DimOS: []string{"Android"},
	}
	d.PowerCycle = true

	expectedDims := d.Dimensions.Copy()

	m := convertDescription(d)
	assert.Equal(t, storeDescription{
		AttachedDevice: machine.AttachedDeviceAdb,
		Dimensions:     expectedDims,
		LastUpdated:    fakeTime,
		PowerCycle:     true,
	}, m)
}

func setupForTest(t *testing.T) (context.Context, config.InstanceConfig) {
	gcp_emulator.RequireFirestore(t)
	cfg := config.InstanceConfig{
		Store: config.Store{
			Project:  "test-project",
			Instance: fmt.Sprintf("test-%s", uuid.New()),
		},
	}
	return now.TimeTravelingContext(fakeTime), cfg
}

func TestNew(t *testing.T) {
	ctx, cfg := setupForTest(t)
	_, err := NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)
}

func TestGet_MachineDoesNotExist_ReturnsError(t *testing.T) {
	ctx, cfg := setupForTest(t)
	store, err := NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	store.getCounter.Reset()

	_, err = store.Get(ctx, "id for machine that does not exist")
	require.Error(t, err)
	require.Equal(t, int64(1), store.getCounter.Get())
}

func TestGet_MachineExists_ReturnsCurrentDescription(t *testing.T) {
	ctx, cfg := setupForTest(t)
	store, err := NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	store.getCounter.Reset()

	const machineID = "skia-rpi2-rack2-shelf2-002"
	ret := machine.NewDescription(ctx)
	err = store.Update(ctx, machineID, func(previous machine.Description) machine.Description {
		ret = previous.Copy()
		ret.PowerCycleState = machine.NotAvailable
		ret.MaintenanceMode = "jcgregorio 2022-11-08"
		return ret
	})
	require.NoError(t, err)

	getRet, err := store.Get(ctx, machineID)
	require.NoError(t, err)
	require.Equal(t, ret, getRet)
}

func TestUpdate_CanUpdateEvenIfDescriptionDoesntExist(t *testing.T) {
	ctx, cfg := setupForTest(t)
	store, err := NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	store.updateCounter.Reset()
	called := false
	err = store.Update(ctx, "skia-rpi2-rack2-shelf1-001", func(previous machine.Description) machine.Description {
		ret := previous.Copy()
		ret.MaintenanceMode = "jcgregorio 2022-11-08"
		called = true
		return ret
	})
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, int64(1), store.updateCounter.Get())
	assert.Equal(t, int64(0), store.updateDataToErrorCounter.Get())

	snap, err := store.machinesCollection.Doc("skia-rpi2-rack2-shelf1-001").Get(ctx)
	require.NoError(t, err)
	var storedDescription storeDescription
	err = snap.DataTo(&storedDescription)
	require.NoError(t, err)
	assert.NoError(t, store.firestoreClient.Close())
}

func TestUpdate_CanUpdateIfDescriptionExists(t *testing.T) {
	ctx, cfg := setupForTest(t)
	store, err := NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	// First write a Description.
	err = store.Update(ctx, "skia-rpi2-rack2-shelf1-001", func(previous machine.Description) machine.Description {
		ret := previous.Copy()
		ret.Recovering = "Low power."
		ret.Dimensions[machine.DimOS] = []string{"Android"}
		return ret
	})
	require.NoError(t, err)

	// Now confirm we get the Description we previously wrote on the next update.
	err = store.Update(ctx, "skia-rpi2-rack2-shelf1-001", func(previous machine.Description) machine.Description {
		assert.Equal(t, "Low power.", previous.Recovering)
		assert.Equal(t, []string{"Android"}, previous.Dimensions["os"])
		assert.Empty(t, previous.Dimensions[machine.DimDeviceType])
		ret := previous.Copy()
		return ret
	})
	require.NoError(t, err)
	assert.NoError(t, store.firestoreClient.Close())
}

func TestList_Success(t *testing.T) {
	ctx, cfg := setupForTest(t)
	store, err := NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	// List on an empty collection is OK.
	descriptions, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, descriptions, 0)

	// Add a single description.
	err = store.Update(ctx, "skia-rpi2-rack2-shelf1-001", func(previous machine.Description) machine.Description {
		ret := previous.Copy()
		ret.Dimensions["foo"] = []string{"bar", "baz"}
		return ret
	})
	require.NoError(t, err)

	// Confirm it appears in the list.
	descriptions, err = store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, descriptions, 1)
	assert.Equal(t, machine.SwarmingDimensions{
		"foo":         {"bar", "baz"},
		machine.DimID: {"skia-rpi2-rack2-shelf1-001"},
	}, descriptions[0].Dimensions)

	// Add a second description.
	err = store.Update(ctx, "skia-rpi2-rack2-shelf1-002", func(previous machine.Description) machine.Description {
		ret := previous.Copy()
		ret.Dimensions["foo"] = []string{"quux"}
		return ret
	})
	require.NoError(t, err)

	// Confirm they both show up in the list.
	descriptions, err = store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, descriptions, 2)
}

func TestListPowerCycle_OneMachineNeedsPowerCycling_ReturnsMachineInList(t *testing.T) {
	ctx, cfg := setupForTest(t)
	store, err := NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)
	store.listPowerCycleCounter.Reset()
	store.listPowerCycleIterErrorCounter.Reset()

	// Add a single machine that needs powercycle.
	const machineID = "skia-rpi2-rack2-shelf1-001"
	err = store.Update(ctx, machineID, func(previous machine.Description) machine.Description {
		ret := previous.Copy()
		ret.PowerCycle = true
		ret.RunningSwarmingTask = true // Confirm we allow powercycling of machines running tasks.
		return ret
	})
	require.NoError(t, err)

	// Add another machine that doesn't need powercycling.
	err = store.Update(ctx, "id-of-machine-that-does-not-need-power-cycle", func(previous machine.Description) machine.Description {
		ret := previous.Copy()
		return ret
	})
	require.NoError(t, err)

	// One machine appears in ListPowerCycle.
	toPowerCycle, err := store.ListPowerCycle(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{machineID}, toPowerCycle)
	require.Equal(t, int64(1), store.listPowerCycleCounter.Get())
	require.Equal(t, int64(0), store.listPowerCycleIterErrorCounter.Get())
}

func TestListPowerCycle_NoMachinesNeedPowerCycling_ReturnsEmptyList(t *testing.T) {
	ctx, cfg := setupForTest(t)
	store, err := NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)
	store.listPowerCycleCounter.Reset()
	store.listPowerCycleIterErrorCounter.Reset()

	// List on an empty collection is OK.
	toPowerCycle, err := store.ListPowerCycle(ctx)
	require.NoError(t, err)
	require.Len(t, toPowerCycle, 0)
	require.Equal(t, int64(1), store.listPowerCycleCounter.Get())
	require.Equal(t, int64(0), store.listPowerCycleIterErrorCounter.Get())

}

func TestDelete_Success(t *testing.T) {
	const machineName = "skia-rpi2-rack2-shelf1-001"
	ctx, cfg := setupForTest(t)
	store, err := NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)
	store.deleteCounter.Reset()

	err = store.Update(ctx, machineName, func(previous machine.Description) machine.Description {
		ret := previous.Copy()
		return ret
	})
	require.NoError(t, err)

	err = store.Delete(ctx, machineName)
	require.NoError(t, err)

	assert.Equal(t, int64(1), store.deleteCounter.Get())

	// Confirm it is really gone.
	_, err = store.machinesCollection.Doc(machineName).Get(ctx)
	require.Error(t, err)
}

func TestDelete_NoErrorIfMachineDoesntExist(t *testing.T) {
	const machineName = "skia-rpi2-rack2-shelf1-001"
	ctx, cfg := setupForTest(t)
	store, err := NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)
	store.deleteCounter.Reset()

	err = store.Delete(ctx, machineName)
	require.NoError(t, err)

	assert.Equal(t, int64(1), store.deleteCounter.Get())
}

var fakeTime = time.Date(2021, time.September, 1, 0, 0, 0, 0, time.UTC)

func TestForceToAttachedDevice(t *testing.T) {
	assert.Equal(t, machine.AttachedDeviceSSH, forceToAttachedDevice(machine.AttachedDeviceSSH))
	assert.Equal(t, machine.AttachedDeviceNone, forceToAttachedDevice(machine.AttachedDevice("this is not a valid attached device name")))
}

func TestForceToPowerCycleState_AllCurrentValuesConvertToThemSelves(t *testing.T) {
	for _, state := range machine.AllPowerCycleStates {
		assert.Equal(t, state, forceToPowerCycleState(state), state)
	}
}

func TestForceToPowerCycleState_UnknownValuesAreConvertedToNotAvailable(t *testing.T) {
	assert.Equal(t, machine.NotAvailable, forceToPowerCycleState(""), "empty string")
	assert.Equal(t, machine.NotAvailable, forceToPowerCycleState("foo-bar-baz"), "foo-bar-baz")
}
