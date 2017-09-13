package modes

import (
	"io/ioutil"
	"path"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

// TestModeHistory verifies that we correctly track mode history.
func TestModeHistory(t *testing.T) {
	testutils.MediumTest(t)

	// Create the ModeHistory.
	tmpDir, err := ioutil.TempDir("", "test_autoroll_mode_")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)
	mh, err := NewModeHistory(path.Join(tmpDir, "test.db"))
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, mh.Close())
	}()

	// Use this function for checking expectations.
	check := func(expect, actual []*ModeChange) {
		assert.Equal(t, len(expect), len(actual))
		for i, e := range expect {
			assert.Equal(t, e.Mode, actual[i].Mode)
			assert.Equal(t, e.Message, actual[i].Message)
			assert.Equal(t, e.User, actual[i].User)
		}

	}

	// Initial mode, set automatically.
	mc0 := &ModeChange{
		Message: "Setting initial mode.",
		Mode:    MODE_RUNNING,
		User:    "AutoRoll Bot",
	}

	expect := []*ModeChange{mc0}
	setModeAndCheck := func(mc *ModeChange) {
		assert.NoError(t, mh.Add(mc.Mode, mc.User, mc.Message))
		assert.Equal(t, mc.Mode, mh.CurrentMode().Mode)
		expect = append([]*ModeChange{mc}, expect...)
		check(expect, mh.GetHistory())
	}

	// Ensure that we set our initial state properly.
	assert.Equal(t, mc0.Mode, mh.CurrentMode().Mode)
	check(expect, mh.GetHistory())

	// Change the mode.
	setModeAndCheck(&ModeChange{
		Message: "Stop the presses!",
		Mode:    MODE_STOPPED,
		User:    "test@google.com",
	})

	// Change a few times.
	setModeAndCheck(&ModeChange{
		Message: "Resume!",
		Mode:    MODE_RUNNING,
		User:    "test@google.com",
	})
}
