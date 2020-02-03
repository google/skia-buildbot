package machine_manager

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/niagara/go/fs_entries"
	"go.skia.org/infra/niagara/go/machine"
)

func TestProcess_FirstTimeSeeingMachine_MarkReady(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	ctx := context.Background()
	m := New(c)
	err := m.process(ctx, machine.State{
		ID: "machine-0001",
	}, machine.Booted)
	require.NoError(t, err)
	expectToFindMachineWithState(t, c, "machine-0001", machine.Ready)
}

func expectToFindMachineWithState(t *testing.T, client *firestore.Client, id string, state machine.Status) {
	doc := client.Collection("machines").Doc(id)
	ds, err := doc.Get(context.Background())
	require.NoError(t, err)
	assert.True(t, ds.Exists())
	var m fs_entries.Machine
	err = ds.DataTo(&m)
	require.NoError(t, err)
	assert.Equal(t, state, m.State)
}
