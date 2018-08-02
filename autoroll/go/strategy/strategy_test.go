package strategy

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

// TestStrategyHistory verifies that we correctly track strategy history.
func TestStrategyHistory(t *testing.T) {
	testutils.LargeTest(t)
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_STRATEGY)

	// TODO(borenet): Remove after all rollers have been upgraded.
	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	// Create the StrategyHistory.
	rollerName := "test-roller"
	sh, err := NewStrategyHistory(ctx, rollerName, ROLL_STRATEGY_BATCH, []string{ROLL_STRATEGY_BATCH, ROLL_STRATEGY_SINGLE}, path.Join(wd, "fake1.db"))
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
		rollerName: []*StrategyChange{sc0},
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
	sh2, err := NewStrategyHistory(ctx, rollerName2, ROLL_STRATEGY_SINGLE, []string{ROLL_STRATEGY_BATCH, ROLL_STRATEGY_SINGLE}, path.Join(wd, "fake2.db"))
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

	assert.NoError(t, sh.refreshHistory(ctx))
	assert.NoError(t, sh2.refreshHistory(ctx))

	checkSlice(expect[rollerName], sh.GetHistory())
	checkSlice(expect[rollerName2], sh2.GetHistory())
}

// TODO(borenet): Remove after all rollers have been upgraded.
func TestStrategyHistoryUpgrade(t *testing.T) {
	testutils.LargeTest(t)
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_STRATEGY)

	rollerName := "test-roller"
	wd, cleanup := testutils.TempDir(t)
	defer cleanup()

	dbFile := path.Join(wd, "bolt.db")
	d, err := openDB(dbFile)
	assert.NoError(t, err)

	now := time.Now().Round(time.Millisecond)
	oldData := []*StrategyChange{
		&StrategyChange{
			Message:  "msg1",
			Strategy: ROLL_STRATEGY_BATCH,
			Time:     now,
			User:     "me",
		},
		&StrategyChange{
			Message:  "msg2",
			Strategy: ROLL_STRATEGY_SINGLE,
			Time:     now.Add(-time.Hour),
			User:     "you",
		},
		&StrategyChange{
			Message:  "msg3",
			Strategy: ROLL_STRATEGY_BATCH,
			Time:     now.Add(-2 * time.Hour),
			User:     "them",
		},
	}
	for _, sc := range oldData {
		assert.NoError(t, d.SetStrategy(sc))
	}
	assert.NoError(t, d.Close())

	// Verify that we port the old data over to the new DB when creating the
	// StrategyHistory.
	sh, err := NewStrategyHistory(ctx, rollerName, ROLL_STRATEGY_SINGLE, []string{ROLL_STRATEGY_BATCH, ROLL_STRATEGY_SINGLE}, dbFile)
	assert.NoError(t, err)
	newData := sh.GetHistory()
	assert.Equal(t, len(oldData), len(newData))
	for idx, actual := range newData {
		expect := oldData[idx]
		// Roller is intentionally not set above, to verify that the
		// migration sets it.
		expect.Roller = rollerName
		deepequal.AssertDeepEqual(t, expect, actual)
	}

	// Verify that we don't try to port the data again.
	sh, err = NewStrategyHistory(ctx, rollerName, ROLL_STRATEGY_SINGLE, []string{ROLL_STRATEGY_BATCH, ROLL_STRATEGY_SINGLE}, dbFile)
	assert.NoError(t, err)
	assert.Equal(t, len(oldData), len(sh.GetHistory()))
}
