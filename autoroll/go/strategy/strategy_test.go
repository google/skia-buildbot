package strategy

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
)

// TestGetHistory verifies that we correctly track strategy history.
func TestGetHistory(t *testing.T) {
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_STRATEGY)

	// Create the StrategyHistory.
	rollerName := "test-roller"
	sh, err := NewDatastoreStrategyHistory(ctx, rollerName, []string{ROLL_STRATEGY_BATCH, ROLL_STRATEGY_SINGLE})
	require.NoError(t, err)

	// Use this function for checking expectations.
	check := func(e, a *StrategyChange) {
		require.Equal(t, e.Strategy, a.Strategy)
		require.Equal(t, e.Message, a.Message)
		require.Equal(t, e.Roller, a.Roller)
		require.Equal(t, e.User, a.User)
	}
	checkGetHistory := func(expect []*StrategyChange, sh StrategyHistory) {
		actual, _, err := sh.GetHistory(ctx, 0)
		require.NoError(t, err)
		require.Equal(t, len(expect), len(actual))
		for i, e := range expect {
			check(e, actual[i])
		}
	}

	// Should be empty initially.
	require.Nil(t, sh.CurrentStrategy())

	// Set the initial strategy.
	expect := map[string][]*StrategyChange{}
	setStrategyAndCheck := func(sc *StrategyChange) {
		require.NoError(t, sh.Add(ctx, sc.Strategy, sc.User, sc.Message))
		require.Equal(t, sc.Strategy, sh.CurrentStrategy().Strategy)
		expect[sc.Roller] = append([]*StrategyChange{sc}, expect[sc.Roller]...)
		checkGetHistory(expect[sc.Roller], sh)
	}

	// Set the initial strategy.
	sc0 := &StrategyChange{
		Message:  "Setting initial strategy.",
		Strategy: ROLL_STRATEGY_BATCH,
		Roller:   rollerName,
		User:     "AutoRoll Bot",
	}
	setStrategyAndCheck(sc0)

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
	sh2, err := NewDatastoreStrategyHistory(ctx, rollerName2, []string{ROLL_STRATEGY_BATCH, ROLL_STRATEGY_SINGLE})
	require.NoError(t, err)

	sc0_2 := &StrategyChange{
		Message:  "Setting initial strategy.",
		Strategy: ROLL_STRATEGY_SINGLE,
		Roller:   rollerName2,
		User:     "AutoRoll Bot",
	}
	require.Nil(t, sh2.CurrentStrategy())
	require.NoError(t, sh2.Add(ctx, sc0_2.Strategy, sc0_2.User, sc0_2.Message))
	check(sc0_2, sh2.CurrentStrategy())
	expect[rollerName2] = []*StrategyChange{sc0_2}
	checkGetHistory(expect[rollerName2], sh2)

	require.NoError(t, sh.Update(ctx))
	require.NoError(t, sh2.Update(ctx))

	checkGetHistory(expect[rollerName], sh)
	checkGetHistory(expect[rollerName2], sh2)

	// Add a bunch of strategy changes and check pagination.
	for i := 0; i < StrategyHistoryLength*2; i++ {
		require.NoError(t, sh.Add(ctx, ROLL_STRATEGY_BATCH, "test@google.com", fmt.Sprintf("Strategy change %d", i)))
	}
	history, nextOffset, err := sh.GetHistory(ctx, 0)
	require.NoError(t, err)
	require.Len(t, history, StrategyHistoryLength)
	require.Equal(t, StrategyHistoryLength, nextOffset)
	history, nextOffset, err = sh.GetHistory(ctx, nextOffset)
	require.NoError(t, err)
	require.Len(t, history, StrategyHistoryLength)
	require.Equal(t, StrategyHistoryLength*2, nextOffset)
	history, nextOffset, err = sh.GetHistory(ctx, nextOffset)
	require.NoError(t, err)
	require.Len(t, history, 3)
	require.Equal(t, 0, nextOffset)
}
