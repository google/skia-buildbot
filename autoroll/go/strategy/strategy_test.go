package strategy

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
)

// TestStrategyHistory verifies that we correctly track strategy history.
func TestStrategyHistory(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_STRATEGY)

	// Create the StrategyHistory.
	rollerName := "test-roller"
	sh, err := NewStrategyHistory(ctx, rollerName, ROLL_STRATEGY_BATCH, []string{ROLL_STRATEGY_BATCH, ROLL_STRATEGY_SINGLE})
	assert.NoError(t, err)

	// Use this function for checking expectations.
	check := func(e, a *StrategyChange) {
		assert.Equal(t, e.Strategy, a.Strategy)
		assert.Equal(t, e.Message, a.Message)
		assert.Equal(t, e.Roller, a.Roller)
		assert.Equal(t, e.User, a.User)
	}
	checkSlice := func(expect, actual []*StrategyChange) {
		assert.Equal(t, len(expect), len(actual))
		for i, e := range expect {
			check(e, actual[i])
		}

	}

	// Initial strategy, set automatically.
	sc0 := &StrategyChange{
		Message:  "Setting initial strategy.",
		Strategy: ROLL_STRATEGY_BATCH,
		Roller:   rollerName,
		User:     "AutoRoll Bot",
	}

	expect := map[string][]*StrategyChange{
		rollerName: {sc0},
	}
	setStrategyAndCheck := func(sc *StrategyChange) {
		assert.NoError(t, sh.Add(ctx, sc.Strategy, sc.User, sc.Message))
		assert.Equal(t, sc.Strategy, sh.CurrentStrategy().Strategy)
		expect[sc.Roller] = append([]*StrategyChange{sc}, expect[sc.Roller]...)
		checkSlice(expect[sc.Roller], sh.GetHistory())
	}

	// Ensure that we set our initial state properly.
	check(sc0, sh.CurrentStrategy())
	checkSlice(expect[sc0.Roller], sh.GetHistory())

	// Change the strategy.
	setStrategyAndCheck(&StrategyChange{
		Message:  "Stop the presses!",
		Roller:   rollerName,
		Strategy: ROLL_STRATEGY_SINGLE,
		User:     "test@google.com",
	})

	// Change a few times.
	setStrategyAndCheck(&StrategyChange{
		Message:  "Resume!",
		Roller:   rollerName,
		Strategy: ROLL_STRATEGY_BATCH,
		User:     "test@google.com",
	})

	// Create a new StrategyHistory for a different roller. Ensure that we
	// don't get the two mixed up.
	rollerName2 := "test-roller-2"
	sh2, err := NewStrategyHistory(ctx, rollerName2, ROLL_STRATEGY_SINGLE, []string{ROLL_STRATEGY_BATCH, ROLL_STRATEGY_SINGLE})
	assert.NoError(t, err)

	sc0_2 := &StrategyChange{
		Message:  "Setting initial strategy.",
		Strategy: ROLL_STRATEGY_SINGLE,
		Roller:   rollerName2,
		User:     "AutoRoll Bot",
	}
	check(sc0_2, sh2.CurrentStrategy())
	expect[rollerName2] = []*StrategyChange{sc0_2}
	checkSlice(expect[rollerName2], sh2.GetHistory())

	assert.NoError(t, sh.Update(ctx))
	assert.NoError(t, sh2.Update(ctx))

	checkSlice(expect[rollerName], sh.GetHistory())
	checkSlice(expect[rollerName2], sh2.GetHistory())
}
