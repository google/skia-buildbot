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
