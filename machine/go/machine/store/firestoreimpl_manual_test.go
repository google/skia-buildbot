package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/machine"
)

func TestWatch_StartWatchBeforeMachineExists(t *testing.T) {
	unittest.ManualTest(t)
	ctx, cfg := setupForFlakyTest(t)
	store, err := NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// First add the watch.
	ch := store.Watch(ctx, "skia-rpi2-rack2-shelf1-001")

	// Then create the document.
	err = store.Update(ctx, "skia-rpi2-rack2-shelf1-001", func(previous machine.Description) machine.Description {
		ret := previous.Copy()
		ret.Mode = machine.ModeMaintenance
		return ret
	})
	require.NoError(t, err)

	// Wait for first description.
	m := <-ch
	assert.Equal(t, machine.ModeMaintenance, m.Mode)
	assert.Equal(t, int64(1), store.watchReceiveSnapshotCounter.Get())
	assert.Equal(t, int64(0), store.watchDataToErrorCounter.Get())
	assert.NoError(t, store.firestoreClient.Close())
}

func TestWatchForPowerCycle_Success(t *testing.T) {
	unittest.ManualTest(t)
	ctx, cfg := setupForTest(t)
	store, err := NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer store.watchForPowerCycleDataToErrorCounter.Reset()
	defer store.watchForPowerCycleReceiveSnapshotCounter.Reset()

	// First add the watch.
	ch := store.WatchForPowerCycle(ctx)

	const machineName = "skia-rpi2-rack2-shelf1-001"

	// Then create the document.
	err = store.Update(ctx, machineName, func(previous machine.Description) machine.Description {
		ret := previous.Copy()
		ret.Mode = machine.ModeMaintenance
		ret.RunningSwarmingTask = false
		ret.PowerCycle = true
		return ret
	})
	require.NoError(t, err)

	// Wait for first pod name.
	m := <-ch
	assert.Equal(t, machineName, m)

	assert.Equal(t, int64(1), store.watchForPowerCycleReceiveSnapshotCounter.Get())
	assert.Equal(t, int64(0), store.watchForPowerCycleDataToErrorCounter.Get())

	// Confirm that PowerCycle has been set back to false.
	err = store.Update(ctx, machineName, func(previous machine.Description) machine.Description {
		require.False(t, previous.PowerCycle)
		return previous
	})
	require.NoError(t, err)

	assert.NoError(t, store.firestoreClient.Close())
}

func TestWatchForPowerCycle_OnlyMatchesTheRightMachines(t *testing.T) {
	unittest.ManualTest(t)
	ctx, cfg := setupForTest(t)
	store, err := NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer store.watchForPowerCycleDataToErrorCounter.Reset()
	defer store.watchForPowerCycleReceiveSnapshotCounter.Reset()

	// First add the watch.
	ch := store.WatchForPowerCycle(ctx)

	// Add some machines that don't match the query.
	err = store.Update(ctx, "skia-rpi2-rack4-shelf2-013", func(previous machine.Description) machine.Description {
		ret := previous.Copy()
		ret.Mode = machine.ModeMaintenance
		ret.RunningSwarmingTask = true
		ret.PowerCycle = true
		return ret
	})
	require.NoError(t, err)

	err = store.Update(ctx, "skia-rpi2-rack4-shelf1-021", func(previous machine.Description) machine.Description {
		ret := previous.Copy()
		ret.Mode = machine.ModeMaintenance
		ret.RunningSwarmingTask = false
		ret.PowerCycle = false
		return ret
	})
	require.NoError(t, err)

	// Now add a machine that will match the query.
	const machineName = "skia-rpi2-rack2-shelf1-001"

	// Then create the document.
	err = store.Update(ctx, machineName, func(previous machine.Description) machine.Description {
		ret := previous.Copy()
		ret.Mode = machine.ModeMaintenance
		ret.RunningSwarmingTask = false
		ret.PowerCycle = true
		return ret
	})
	require.NoError(t, err)

	// Wait for first pod name.
	m := <-ch
	assert.Equal(t, machineName, m)

	assert.Equal(t, int64(1), store.watchForPowerCycleReceiveSnapshotCounter.Get())
	assert.Equal(t, int64(0), store.watchForPowerCycleDataToErrorCounter.Get())
	assert.NoError(t, store.firestoreClient.Close())
}

func TestWatchForPowerCycle_IsCancellable(t *testing.T) {
	unittest.ManualTest(t)
	ctx, cfg := setupForTest(t)
	store, err := NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(ctx)

	// First add the watch.
	ch := store.WatchForPowerCycle(ctx)

	const machineName = "skia-rpi2-rack2-shelf1-001"

	// Then create the document.
	err = store.Update(ctx, machineName, func(previous machine.Description) machine.Description {
		ret := previous.Copy()
		ret.Mode = machine.ModeMaintenance
		ret.RunningSwarmingTask = false
		ret.PowerCycle = true
		return ret
	})
	require.NoError(t, err)

	cancel()

	// The test passes if we get past this loop since that means the channel was closed.
	for range ch {
	}
	assert.NoError(t, store.firestoreClient.Close())
}
