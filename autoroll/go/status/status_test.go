package status

import (
	"context"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
)

func TestCopyStatus(t *testing.T) {
	testutils.SmallTest(t)
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
		LastRollRev:    recent[1].RollingTo,
		NotRolledRevisions: []*repo_manager.Revision{
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
		ValidModes:      modes.VALID_MODES,
		ValidStrategies: []string{strategy.ROLL_STRATEGY_SINGLE, strategy.ROLL_STRATEGY_BATCH},
	}
	deepequal.AssertCopy(t, v, v.Copy())
}

func TestStatus(t *testing.T) {
	testutils.LargeTest(t)
	ctx := context.Background()
	testutil.InitDatastore(t, ds.KIND_AUTOROLL_STATUS)

	// No data in the datastore, but we shouldn't return an error.
	rollerName := "test-roller"
	c, err := NewCache(ctx, rollerName)
	assert.NoError(t, err)

	// We should return empty until there's actually some data.
	deepequal.AssertDeepEqual(t, &AutoRollStatus{}, c.Get())
	deepequal.AssertDeepEqual(t, &AutoRollMiniStatus{}, c.GetMini())

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
		LastRollRev:     recent[1].RollingTo,
		Recent:          recent,
		Status:          "some-status",
		ThrottledUntil:  time.Now().Unix(),
		ValidModes:      modes.VALID_MODES,
		ValidStrategies: []string{strategy.ROLL_STRATEGY_SINGLE, strategy.ROLL_STRATEGY_BATCH},
	}
	assert.NoError(t, Set(ctx, rollerName, s))
	actual, err := Get(ctx, rollerName)
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, s, actual)

	// Cache should return empty until we Update(), at which point we should
	// get back the same status.
	assert.Equal(t, &AutoRollStatus{}, c.Get())
	assert.NoError(t, c.Update(ctx))
	deepequal.AssertDeepEqual(t, s, c.Get())

	// Ensure that we don't confuse multiple rollers.
	c2, err := NewCache(ctx, "roller2")
	assert.NoError(t, err)
	assert.Equal(t, &AutoRollStatus{}, c2.Get())
	assert.Equal(t, &AutoRollMiniStatus{}, c2.GetMini())
}
