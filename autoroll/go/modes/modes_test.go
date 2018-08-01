package modes

import (
	"context"
	"path"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
)

// TestModeHistory verifies that we correctly track mode history.
func TestModeHistory(t *testing.T) {
	testutils.LargeTest(t)
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_MODE)

	// TODO(borenet): Remove after all rollers have been upgraded.
	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	// Create the ModeHistory.
	rollerName := "test-roller"
	mh, err := NewModeHistory(ctx, rollerName, path.Join(wd, "fake1.db"))
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

	expect := map[string][]*ModeChange{
		mc0.Roller: []*ModeChange{mc0},
	}

	// Ensure that we set our initial state properly.
	check(mc0, mh.CurrentMode())
	checkSlice(expect[mc0.Roller], mh.GetHistory())

	setModeAndCheck := func(mc *ModeChange) {
		assert.NoError(t, mh.Add(ctx, mc.Mode, mc.User, mc.Message))
		assert.Equal(t, mc.Mode, mh.CurrentMode().Mode)
		expect[mc.Roller] = append([]*ModeChange{mc}, expect[mc.Roller]...)
		checkSlice(expect[mc.Roller], mh.GetHistory())
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

	// Create a new ModeHistory for a different roller. Ensure that we don't
	// get the two mixed up.
	rollerName2 := "test-roller-2"
	mh2, err := NewModeHistory(ctx, rollerName2, path.Join(wd, "fake2.db"))
	assert.NoError(t, err)

	mc0_2 := &ModeChange{
		Message: "Setting initial mode.",
		Mode:    MODE_RUNNING,
		Roller:  rollerName2,
		User:    "AutoRoll Bot",
	}
	check(mc0_2, mh2.CurrentMode())
	expect[rollerName2] = []*ModeChange{mc0_2}
	checkSlice(expect[rollerName2], mh2.GetHistory())

	assert.NoError(t, mh.refreshHistory(ctx))
	assert.NoError(t, mh2.refreshHistory(ctx))

	checkSlice(expect[rollerName], mh.GetHistory())
	checkSlice(expect[rollerName2], mh2.GetHistory())
}

// TODO(borenet): Remove after all rollers have been upgraded.
func TestModeHistoryUpgrade(t *testing.T) {
	testutils.LargeTest(t)
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_MODE)

	rollerName := "test-roller"
	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	dbFile := path.Join(wd, "bolt.db")
	d, err := openDB(dbFile)
	assert.NoError(t, err)

	now := time.Now().Round(time.Millisecond)
	oldData := []*ModeChange{
		&ModeChange{
			Message: "msg1",
			Mode:    MODE_RUNNING,
			Time:    now,
			User:    "me",
		},
		&ModeChange{
			Message: "msg2",
			Mode:    MODE_STOPPED,
			Time:    now.Add(-time.Hour),
			User:    "you",
		},
		&ModeChange{
			Message: "msg3",
			Mode:    MODE_DRY_RUN,
			Time:    now.Add(-2 * time.Hour),
			User:    "them",
		},
	}
	for _, mc := range oldData {
		assert.NoError(t, d.SetMode(mc))
	}
	assert.NoError(t, d.Close())

	// Verify that we port the old data over to the new DB when creating the
	// ModeHistory.
	mh, err := NewModeHistory(ctx, rollerName, dbFile)
	assert.NoError(t, err)
	newData := mh.GetHistory()
	assert.Equal(t, len(oldData), len(newData))
	for idx, actual := range newData {
		expect := oldData[idx]
		// Roller is intentionally not set above, to verify that the
		// migration sets it.
		expect.Roller = rollerName
		deepequal.AssertDeepEqual(t, expect, actual)
	}

	// Verify that we don't try to port the data again.
	mh, err = NewModeHistory(ctx, rollerName, dbFile)
	assert.NoError(t, err)
	assert.Equal(t, len(oldData), len(mh.GetHistory()))
}
