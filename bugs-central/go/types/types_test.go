package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
)

func TestCalculateSLOViolations(t *testing.T) {
	unittest.SmallTest(t)

	ics := IssueCountsData{}

	// Test P0 SLOs:
	// * No violations with now used for created and modified.
	now := time.Unix(1405544146, 0)
	ics.CalculateSLOViolations(now, now, now, PriorityP0)
	require.Equal(t, 0, ics.P0SLOViolationCount)
	// * Created before a week.
	created := time.Unix(1405544146, 0)
	now = created.Add(Weekly).Add(time.Minute)
	modified := now
	ics.CalculateSLOViolations(now, created, modified, PriorityP0)
	require.Equal(t, 1, ics.P0SLOViolationCount)
	// * Modified before 24 hours.
	modified = time.Unix(1405544146, 0)
	now = modified.Add(Daily).Add(time.Minute)
	created = now
	ics.CalculateSLOViolations(now, created, modified, PriorityP0)
	require.Equal(t, 2, ics.P0SLOViolationCount)

	// Test P1 SLOs:
	// * No violations with now used for created and modified.
	now = time.Unix(1405544146, 0)
	ics.CalculateSLOViolations(now, now, now, PriorityP1)
	require.Equal(t, 0, ics.P1SLOViolationCount)
	// * Created before a month.
	created = time.Unix(1405544146, 0)
	now = created.Add(Monthly).Add(time.Minute)
	modified = now
	ics.CalculateSLOViolations(now, created, modified, PriorityP1)
	require.Equal(t, 1, ics.P1SLOViolationCount)
	// * Modified before a week.
	modified = time.Unix(1405544146, 0)
	now = modified.Add(Weekly).Add(time.Minute)
	created = now
	ics.CalculateSLOViolations(now, created, modified, PriorityP1)
	require.Equal(t, 2, ics.P1SLOViolationCount)

	// Test P2 SLOs:
	// * No violations with now used for created and modified.
	now = time.Unix(1405544146, 0)
	ics.CalculateSLOViolations(now, now, now, PriorityP2)
	require.Equal(t, 0, ics.P2SLOViolationCount)
	// * Created before a year.
	created = time.Unix(1405544146, 0)
	now = created.Add(Yearly).Add(time.Minute)
	modified = now
	ics.CalculateSLOViolations(now, created, modified, PriorityP2)
	require.Equal(t, 1, ics.P2SLOViolationCount)
	// * Modified before 6 months.
	modified = time.Unix(1405544146, 0)
	now = modified.Add(Biannualy).Add(time.Minute)
	created = now
	ics.CalculateSLOViolations(now, created, modified, PriorityP2)
	require.Equal(t, 2, ics.P2SLOViolationCount)

	// Test P3 SLOs:
	// * No violations with now used for created and modified.
	now = time.Unix(1405544146, 0)
	ics.CalculateSLOViolations(now, now, now, PriorityP3)
	require.Equal(t, 0, ics.P3SLOViolationCount)
	// * Created before 2 years.
	created = time.Unix(1405544146, 0)
	now = created.Add(Biennialy).Add(time.Minute)
	modified = now
	ics.CalculateSLOViolations(now, created, modified, PriorityP3)
	require.Equal(t, 1, ics.P3SLOViolationCount)
	// * Modified before a year.
	modified = time.Unix(1405544146, 0)
	now = modified.Add(Yearly).Add(time.Minute)
	created = now
	ics.CalculateSLOViolations(now, created, modified, PriorityP3)
	require.Equal(t, 2, ics.P3SLOViolationCount)
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
