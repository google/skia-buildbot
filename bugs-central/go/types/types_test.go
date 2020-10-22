package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
)

func TestCalculateSLOViolations(t *testing.T) {
	unittest.SmallTest(t)

	now := time.Unix(1405544146, 0)
	after1Day := now.Add(Daily).Add(time.Minute)
	after1Week := now.Add(Weekly).Add(time.Minute)
	after1Month := now.Add(Monthly).Add(time.Minute)
	after6Months := now.Add(Biannualy).Add(time.Minute)
	after1Year := now.Add(Yearly).Add(time.Minute)
	after2Years := now.Add(Biennialy).Add(time.Minute)

	tests := []struct {
		now      time.Time
		created  time.Time
		modified time.Time
		priority StandardizedPriority

		expectedP0Violations int
		expectedP1Violations int
		expectedP2Violations int
		expectedP3Violations int
	}{
		// Test P0 SLOs:
		// * No violations with now used for created and modified.
		{now: now, created: now, modified: now, priority: PriorityP0},
		// * Created before a week.
		{now: after1Week, created: now, modified: now, priority: PriorityP0, expectedP0Violations: 1},
		// * Modified before 24 hours.
		{now: after1Day, created: now, modified: now, priority: PriorityP0, expectedP0Violations: 1},

		// Test P1 SLOs:
		// * No violations with now used for created and modified.
		{now: now, created: now, modified: now, priority: PriorityP1},
		// * Created before a month.
		{now: after1Month, created: now, modified: now, priority: PriorityP1, expectedP1Violations: 1},
		// * Modified before a week.
		{now: after1Week, created: now, modified: now, priority: PriorityP1, expectedP1Violations: 1},

		// Test P2 SLOs:
		// * No violations with now used for created and modified.
		{now: now, created: now, modified: now, priority: PriorityP2},
		// * Created before a year.
		{now: after1Year, created: now, modified: now, priority: PriorityP2, expectedP2Violations: 1},
		// * Modified before 6 months.
		{now: after6Months, created: now, modified: now, priority: PriorityP2, expectedP2Violations: 1},

		// Test P3 SLOs:
		// * No violations with now used for created and modified.
		{now: now, created: now, modified: now, priority: PriorityP3},
		// * Created before 2 years.
		{now: after2Years, created: now, modified: now, priority: PriorityP3, expectedP3Violations: 1},
		// * Modified before a year.
		{now: after1Year, created: now, modified: now, priority: PriorityP3, expectedP3Violations: 1},
	}
	for _, test := range tests {
		ics := IssueCountsData{}
		ics.CalculateSLOViolations(test.now, test.created, test.modified, test.priority)
		require.Equal(t, test.expectedP0Violations, ics.P0SLOViolationCount)
		require.Equal(t, test.expectedP1Violations, ics.P1SLOViolationCount)
		require.Equal(t, test.expectedP2Violations, ics.P2SLOViolationCount)
		require.Equal(t, test.expectedP3Violations, ics.P3SLOViolationCount)
	}
}

func TestMergeInfo(t *testing.T) {
	unittest.SmallTest(t)

	to := IssueCountsData{
		OpenCount:       120,
		UnassignedCount: 31,

		P0Count: 1,
		P1Count: 2,
		P2Count: 5,
		P3Count: 52,

		P0SLOViolationCount: 1,
		P2SLOViolationCount: 2,
	}
	from := IssueCountsData{
		OpenCount:       20,
		UnassignedCount: 13,

		P0Count: 1,
		P1Count: 2,
		P2Count: 5,
		P3Count: 4,

		P0SLOViolationCount: 0,
		P2SLOViolationCount: 2,
		P3SLOViolationCount: 4,
	}
	to.Merge(&from)
	require.Equal(t, 140, to.OpenCount)
	require.Equal(t, 44, to.UnassignedCount)
	require.Equal(t, 2, to.P0Count)
	require.Equal(t, 4, to.P1Count)
	require.Equal(t, 10, to.P2Count)
	require.Equal(t, 56, to.P3Count)
	require.Equal(t, 1, to.P0SLOViolationCount)
	require.Equal(t, 4, to.P2SLOViolationCount)
	require.Equal(t, 4, to.P3SLOViolationCount)
}

func TestIncPriority(t *testing.T) {
	unittest.SmallTest(t)

	ics := IssueCountsData{}
	assertPriorityCounts := func(p0, p1, p2, p3, p4, p5, p6 int) {
		require.Equal(t, p0, ics.P0Count)
		require.Equal(t, p1, ics.P1Count)
		require.Equal(t, p2, ics.P2Count)
		require.Equal(t, p3, ics.P3Count)
		require.Equal(t, p4, ics.P4Count)
		require.Equal(t, p5, ics.P5Count)
		require.Equal(t, p6, ics.P6Count)
	}

	assertPriorityCounts(0, 0, 0, 0, 0, 0, 0)
	ics.IncPriority(PriorityP0)
	assertPriorityCounts(1, 0, 0, 0, 0, 0, 0)
	ics.IncPriority(PriorityP1)
	ics.IncPriority(PriorityP1)
	ics.IncPriority(PriorityP4)
	assertPriorityCounts(1, 2, 0, 0, 1, 0, 0)
	ics.IncPriority(PriorityP2)
	ics.IncPriority(PriorityP4)
	ics.IncPriority(PriorityP5)
	ics.IncPriority(PriorityP6)
	assertPriorityCounts(1, 2, 1, 0, 2, 1, 1)
}
