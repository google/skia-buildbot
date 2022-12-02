package tests

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/emulators/gcp_emulator"
	"go.skia.org/infra/machine/go/machine/change/sink"
	"go.skia.org/infra/machine/go/machine/change/source"
	"go.skia.org/infra/machine/go/machineserver/config"
)

func TestSourceAndSink(t *testing.T) {
	gcp_emulator.RequirePubSub(t)

	const machineID1 = "skia-rpi2-rack4-shelf1-001"
	const machineID2 = "skia-rpi2-rack4-shelf1-002"

	ctx := context.Background()
	config := config.DescriptionChangeSource{
		Project: "test-project",
		Topic:   fmt.Sprintf("events-%d", rand.Int63()),
	}
	sink, err := sink.New(ctx, true, config)
	require.NoError(t, err)

	source1, err := source.New(ctx, true, config, machineID1)
	require.NoError(t, err)

	source2, err := source.New(ctx, true, config, machineID2)
	require.NoError(t, err)

	// Send two events to machine 1 and one event to machine 2.
	require.NoError(t, sink.Send(ctx, machineID1))
	require.NoError(t, sink.Send(ctx, machineID2))
	require.NoError(t, sink.Send(ctx, machineID1))

	// Confirm everyone gets the right number of events.
	ch1 := source1.Start(ctx)
	<-ch1
	<-ch1

	ch2 := source2.Start(ctx)
	<-ch2

	// The only way to reach here is if the three messages sent only went to
	// their designated listeners.
}
