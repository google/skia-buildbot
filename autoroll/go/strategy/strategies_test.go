package strategy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/revision"
)

func TestStrategyBatch(t *testing.T) {

	s := StrategyBatch()

	// No revisions to roll.
	require.Nil(t, s.GetNextRollRev(nil))
	require.Nil(t, s.GetNextRollRev([]*revision.Revision{}))

	// More than one revision available. We should choose the most recent.
	// Revisions are passed in reverse chronological order.
	testRevs := []*revision.Revision{
		{
			Id: "D",
		},
		{
			Id: "C",
		},
		{
			Id: "B",
		},
		{
			Id: "A",
		},
	}
	require.Equal(t, testRevs[0], s.GetNextRollRev(testRevs))

	// The most recent is not valid; we should choose the most recent valid
	// revision.
	testRevs[0].InvalidReason = "flu"
	require.Equal(t, testRevs[1], s.GetNextRollRev(testRevs))

	// No revisions are valid. We can't roll.
	for _, rev := range testRevs {
		rev.InvalidReason = "flu"
	}
	require.Nil(t, s.GetNextRollRev(testRevs))
}

func TestStrategyNBatch(t *testing.T) {

	s := StrategyNBatch()

	// No revisions to roll.
	require.Nil(t, s.GetNextRollRev(nil))
	require.Nil(t, s.GetNextRollRev([]*revision.Revision{}))

	// More than one revision available. We should choose the Nth (from the
	// end of the slice, since Revisions are passed in reverse chronological
	// order).
	testRevs := make([]*revision.Revision, 0, N_REVISIONS+2)
	for i := 0; i < N_REVISIONS+2; i++ {
		testRevs = append(testRevs, &revision.Revision{
			Id: fmt.Sprintf("%d", N_REVISIONS+2-i),
		})
	}
	nthIdx := len(testRevs) - N_REVISIONS
	require.Equal(t, testRevs[nthIdx], s.GetNextRollRev(testRevs))

	// The above Revision is not valid; we should choose the most recent
	// valid Revision *before* it.
	testRevs[nthIdx].InvalidReason = "flu"
	require.Equal(t, testRevs[nthIdx+1], s.GetNextRollRev(testRevs))

	// No revisions are valid. We can't roll.
	for _, rev := range testRevs {
		rev.InvalidReason = "flu"
	}
	require.Nil(t, s.GetNextRollRev(testRevs))
}

func TestStrategySingle(t *testing.T) {

	s := StrategySingle()

	// No revisions to roll.
	require.Nil(t, s.GetNextRollRev(nil))
	require.Nil(t, s.GetNextRollRev([]*revision.Revision{}))

	// More than one revision available. We should choose the oldest.
	// Revisions are passed in reverse chronological order.
	testRevs := []*revision.Revision{
		{
			Id: "D",
		},
		{
			Id: "C",
		},
		{
			Id: "B",
		},
		{
			Id: "A",
		},
	}
	require.Equal(t, testRevs[len(testRevs)-1], s.GetNextRollRev(testRevs))

	// The most recent is not valid; we should choose the most recent valid
	// revision.
	testRevs[len(testRevs)-1].InvalidReason = "flu"
	require.Equal(t, testRevs[len(testRevs)-2], s.GetNextRollRev(testRevs))

	// No revisions are valid. We can't roll.
	for _, rev := range testRevs {
		rev.InvalidReason = "flu"
	}
	require.Nil(t, s.GetNextRollRev(testRevs))
}
