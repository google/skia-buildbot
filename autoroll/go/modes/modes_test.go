package modes

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
)

// TestGetHistory verifies that we correctly track mode history.
func TestGetHistory(t *testing.T) {
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_MODE)

	// Create the ModeHistory.
	rollerName := "test-roller"
	mh, err := NewDatastoreModeHistory(ctx, rollerName)
	require.NoError(t, err)

	// Use this function for checking expectations.
	check := func(e, a *ModeChange) {
		require.Equal(t, e.Mode, a.Mode)
		require.Equal(t, e.Message, a.Message)
		require.Equal(t, e.Roller, a.Roller)
		require.Equal(t, e.User, a.User)
	}
	checkGetHistory := func(expect []*ModeChange, mh ModeHistory) {
		actual, _, err := mh.GetHistory(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, len(expect), len(actual))
		for i, e := range expect {
			check(e, actual[i])
		}
	}

	// Should be empty initially.
	require.Nil(t, mh.CurrentMode())

	// Set the initial mode.
	expect := map[string][]*ModeChange{}
	setModeAndCheck := func(mc *ModeChange) {
		require.NoError(t, mh.Add(ctx, mc.Mode, mc.User, mc.Message))
		require.Equal(t, mc.Mode, mh.CurrentMode().Mode)
		expect[mc.Roller] = append([]*ModeChange{mc}, expect[mc.Roller]...)
		checkGetHistory(expect[mc.Roller], mh)
	}

	// Set the initial mode.
	mc0 := &ModeChange{
		Message: "Setting initial mode.",
		Mode:    ModeRunning,
		Roller:  rollerName,
		User:    "AutoRoll Bot",
	}
	setModeAndCheck(mc0)

	// Change the mode.
	setModeAndCheck(&ModeChange{
		Message: "Stop the presses!",
		Mode:    ModeStopped,
		Roller:  rollerName,
		User:    "test@google.com",
	})

	// Change a few times.
	setModeAndCheck(&ModeChange{
		Message: "Resume!",
		Mode:    ModeRunning,
		Roller:  rollerName,
		User:    "test@google.com",
	})

	// Create a new ModeHistory for a different roller. Ensure that we don't
	// get the two mixed up.
	rollerName2 := "test-roller-2"
	mh2, err := NewDatastoreModeHistory(ctx, rollerName2)
	require.NoError(t, err)

	mc0_2 := &ModeChange{
		Message: "Setting initial mode.",
		Mode:    ModeRunning,
		Roller:  rollerName2,
		User:    "AutoRoll Bot",
	}
	require.Nil(t, mh2.CurrentMode())
	require.NoError(t, mh2.Add(ctx, mc0_2.Mode, mc0_2.User, mc0_2.Message))
	check(mc0_2, mh2.CurrentMode())
	expect[rollerName2] = []*ModeChange{mc0_2}
	checkGetHistory(expect[rollerName2], mh2)

	require.NoError(t, mh.Update(ctx))
	require.NoError(t, mh2.Update(ctx))

	checkGetHistory(expect[rollerName], mh)
	checkGetHistory(expect[rollerName2], mh2)

	// Add a bunch of mode changes and check pagination.
	for i := 0; i < ModeHistoryLength*2; i++ {
		require.NoError(t, mh.Add(ctx, ModeRunning, "test@google.com", fmt.Sprintf("Mode change %d", i)))
	}
	history, nextOffset, err := mh.GetHistory(ctx, 0)
	require.NoError(t, err)
	require.Len(t, history, ModeHistoryLength)
	require.Equal(t, ModeHistoryLength, nextOffset)
	history, nextOffset, err = mh.GetHistory(ctx, nextOffset)
	require.NoError(t, err)
	require.Len(t, history, ModeHistoryLength)
	require.Equal(t, ModeHistoryLength*2, nextOffset)
	history, nextOffset, err = mh.GetHistory(ctx, nextOffset)
	require.NoError(t, err)
	require.Len(t, history, 3)
	require.Equal(t, 0, nextOffset)
}
