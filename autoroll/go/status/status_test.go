package status

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestCopyStatus(t *testing.T) {
	unittest.SmallTest(t)
	recent := []*autoroll.AutoRollIssue{
		{
			RollingTo: "abc123",
		},
		{
			RollingTo: "def456",
		},
	}
	v := &AutoRollStatus{
		AutoRollMiniStatus: AutoRollMiniStatus{
			CurrentRollRev:      recent[0].RollingTo,
			LastRollRev:         recent[1].RollingTo,
			NumFailedRolls:      3,
			NumNotRolledCommits: 6,
		},
		ChildHead:      "abc123",
		ChildName:      "child-repo",
		CurrentRoll:    recent[0],
		Error:          "some error!",
		FullHistoryUrl: "http://history",
		IssueUrlBase:   "http://issue.url/",
		LastRoll:       recent[1],
		NotRolledRevisions: []*revision.Revision{
			{
				Id: "a",
			},
			{
				Id: "b",
			},
		},
		ParentName:      "parent-repo",
		Recent:          recent,
		Status:          "some-status",
		ThrottledUntil:  time.Now().Unix(),
		ValidModes:      modes.ValidModes,
		ValidStrategies: []string{strategy.ROLL_STRATEGY_SINGLE, strategy.ROLL_STRATEGY_BATCH},
	}
	assertdeep.Copy(t, v, v.Copy())
}

func TestStatus(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_STATUS)

	// No data in the datastore, but we shouldn't return an error.
	rollerName := "test-roller"
	c, err := NewCache(ctx, rollerName)
	require.NoError(t, err)

	// We should return empty until there's actually some data.
	assertdeep.Equal(t, &AutoRollStatus{}, c.Get())
	assertdeep.Equal(t, &AutoRollMiniStatus{}, c.GetMini())

	// Insert a status.
	recent := []*autoroll.AutoRollIssue{
		{
			RollingTo: "abc123",
		},
		{
			RollingTo: "def456",
		},
	}
	s := &AutoRollStatus{
		AutoRollMiniStatus: AutoRollMiniStatus{
			CurrentRollRev:      recent[0].RollingTo,
			LastRollRev:         recent[1].RollingTo,
			NumFailedRolls:      3,
			NumNotRolledCommits: 6,
		},
		CurrentRoll:     recent[0],
		Error:           "some error!",
		FullHistoryUrl:  "http://history",
		IssueUrlBase:    "http://issue.url/",
		LastRoll:        recent[1],
		Recent:          recent,
		Status:          "some-status",
		ThrottledUntil:  time.Now().Unix(),
		ValidModes:      modes.ValidModes,
		ValidStrategies: []string{strategy.ROLL_STRATEGY_SINGLE, strategy.ROLL_STRATEGY_BATCH},
	}
	require.NoError(t, Set(ctx, rollerName, s))
	actual, err := Get(ctx, rollerName)
	require.NoError(t, err)
	assertdeep.Equal(t, s, actual)

	// Cache should return empty until we Update(), at which point we should
	// get back the same status.
	require.Equal(t, &AutoRollStatus{}, c.Get())
	require.NoError(t, c.Update(ctx))
	assertdeep.Equal(t, s, c.Get())

	// Ensure that we don't confuse multiple rollers.
	c2, err := NewCache(ctx, "roller2")
	require.NoError(t, err)
	require.Equal(t, &AutoRollStatus{}, c2.Get())
	require.Equal(t, &AutoRollMiniStatus{}, c2.GetMini())
}
