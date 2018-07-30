package modes

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
)

// TestModeHistory verifies that we correctly track mode history.
func TestModeHistory(t *testing.T) {
	testutils.MediumTest(t)
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_MODE)

	// Create the ModeHistory.
	rollerName := "test-roller"
	mh, err := NewModeHistory(ctx, rollerName)
	assert.NoError(t, err)

	// Use this function for checking expectations.
	check := func(e, a *ModeChange) {
		assert.Equal(t, e.Mode, a.Mode)
		assert.Equal(t, e.Message, a.Message)
		assert.Equal(t, e.Roller, a.Roller)
		assert.Equal(t, e.User, a.User)
	}
	checkSlice := func(expect, actual []*ModeChange) {
		assert.Equal(t, len(expect), len(actual))
		for i, e := range expect {
			check(e, actual[i])
		}
	}

	// Initial mode, set automatically.
	mc0 := &ModeChange{
		Message: "Setting initial mode.",
		Mode:    MODE_RUNNING,
		Roller:  rollerName,
		User:    "AutoRoll Bot",
	}

	expect := []*ModeChange{mc0}

	// Ensure that we set our initial state properly.
	check(mc0, mh.CurrentMode())
	checkSlice(expect, mh.GetHistory())

	setModeAndCheck := func(mc *ModeChange) {
		assert.NoError(t, mh.Add(ctx, mc.Mode, mc.User, mc.Message))
		assert.Equal(t, mc.Mode, mh.CurrentMode().Mode)
		expect = append([]*ModeChange{mc}, expect...)
		checkSlice(expect, mh.GetHistory())
	}

	// Change the mode.
	setModeAndCheck(&ModeChange{
		Message: "Stop the presses!",
		Mode:    MODE_STOPPED,
		Roller:  rollerName,
		User:    "test@google.com",
	})

	// Change a few times.
	setModeAndCheck(&ModeChange{
		Message: "Resume!",
		Mode:    MODE_RUNNING,
		Roller:  rollerName,
		User:    "test@google.com",
	})
}
