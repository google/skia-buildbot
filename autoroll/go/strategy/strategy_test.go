package strategy

import (
	"io/ioutil"
	"path"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

// TestStrategyHistory verifies that we correctly track strategy history.
func TestStrategyHistory(t *testing.T) {
	testutils.MediumTest(t)

	// Create the StrategyHistory.
	tmpDir, err := ioutil.TempDir("", "test_autoroll_strategy_")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)
	mh, err := NewStrategyHistory(path.Join(tmpDir, "test.db"), ROLL_STRATEGY_BATCH, []string{ROLL_STRATEGY_BATCH, ROLL_STRATEGY_SINGLE})
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, mh.Close())
	}()

	// Use this function for checking expectations.
	check := func(expect, actual []*StrategyChange) {
		assert.Equal(t, len(expect), len(actual))
		for i, e := range expect {
			assert.Equal(t, e.Strategy, actual[i].Strategy)
			assert.Equal(t, e.Message, actual[i].Message)
			assert.Equal(t, e.User, actual[i].User)
		}

	}

	// Initial strategy, set automatically.
	mc0 := &StrategyChange{
		Message:  "Setting initial strategy.",
		Strategy: ROLL_STRATEGY_BATCH,
		User:     "AutoRoll Bot",
	}

	expect := []*StrategyChange{mc0}
	setStrategyAndCheck := func(mc *StrategyChange) {
		assert.NoError(t, mh.Add(mc.Strategy, mc.User, mc.Message))
		assert.Equal(t, mc.Strategy, mh.CurrentStrategy().Strategy)
		expect = append([]*StrategyChange{mc}, expect...)
		check(expect, mh.GetHistory())
	}

	// Ensure that we set our initial state properly.
	assert.Equal(t, mc0.Strategy, mh.CurrentStrategy().Strategy)
	check(expect, mh.GetHistory())

	// Change the strategy.
	setStrategyAndCheck(&StrategyChange{
		Message:  "Stop the presses!",
		Strategy: ROLL_STRATEGY_SINGLE,
		User:     "test@google.com",
	})

	// Change a few times.
	setStrategyAndCheck(&StrategyChange{
		Message:  "Resume!",
		Strategy: ROLL_STRATEGY_BATCH,
		User:     "test@google.com",
	})
}
