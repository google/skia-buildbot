package strategy

import (
	"fmt"

	"go.skia.org/infra/autoroll/go/revision"
)

const (
	ROLL_STRATEGY_BATCH = "batch"
	// TODO(rmistry): Rename to "batch of " + N_REVISIONS ?
	ROLL_STRATEGY_N_BATCH = "n_batch"
	ROLL_STRATEGY_SINGLE  = "single"

	// The number of Revisions to use in ROLL_STRATEGY_N_BATCH.
	N_REVISIONS = 20
)

// NextRollStrategy is an interface for modules which determine what the next
// roll Revision should be.
type NextRollStrategy interface {
	// Return the next roll revision, given the list of not-yet-rolled
	// Revisions in reverse chronological order. Returning nil implies that
	// we cannot create a roll; we are up to date, or there are no valid
	// revisions to roll.
	GetNextRollRev([]*revision.Revision) *revision.Revision
}

// Return the NextRollStrategy indicated by the given string.
func GetNextRollStrategy(strategy string) (NextRollStrategy, error) {
	switch strategy {
	case ROLL_STRATEGY_BATCH:
		return StrategyBatch(), nil
	case ROLL_STRATEGY_N_BATCH:
		return StrategyNBatch(), nil
	case ROLL_STRATEGY_SINGLE:
		return StrategySingle(), nil
	default:
		return nil, fmt.Errorf("Unknown roll strategy %q", strategy)
	}
}

// batchStrategy is a NextRollStrategy which always rolls to HEAD of a given branch.
type batchStrategy struct{}

// See documentation for NextRollStrategy interface.
func (s *batchStrategy) GetNextRollRev(notRolled []*revision.Revision) *revision.Revision {
	// Revisions are listed in reverse chronological order. Return the first
	// one which is not marked invalid.
	for _, rev := range notRolled {
		if rev.InvalidReason == "" {
			return rev
		}
	}
	return nil
}

// StrategyBatch returns a NextRollStrategy which always rolls to HEAD of a given branch.
func StrategyBatch() NextRollStrategy {
	return &batchStrategy{}
}

// nBatchStrategy is a NextRollStrategy which always rolls to maximum N Revisions of a
// given branch.
type nBatchStrategy struct{}

// See documentation for NextRollStrategy interface.
func (s *nBatchStrategy) GetNextRollRev(notRolled []*revision.Revision) *revision.Revision {
	idx := 0
	if len(notRolled) > N_REVISIONS {
		idx = len(notRolled) - N_REVISIONS
	}
	return StrategyBatch().GetNextRollRev(notRolled[idx:])
}

// StrategyNBatch returns a NextRollStrategy which always rolls to maximum N Revisions of a
// given branch.
func StrategyNBatch() NextRollStrategy {
	return &nBatchStrategy{}
}

// singleStrategy is a NextRollStrategy which rolls toward HEAD of a given branch, one
// Revision at a time.
type singleStrategy struct{}

// See documentation for NextRollStrategy interface.
func (s *singleStrategy) GetNextRollRev(notRolled []*revision.Revision) *revision.Revision {
	// TODO(borenet): This violates the assumption that we roll exactly
	// one Revision at a time; if the first N Revisions are invalid, we'll
	// roll N+1 Revisions. I think this is what we want, to prevent the
	// roller getting stuck forever on an invalid Revision, but I'm not
	// 100% sure.
	for i := len(notRolled) - 1; i >= 0; i-- {
		if notRolled[i].InvalidReason == "" {
			return notRolled[i]
		}
	}
	return nil
}

// StrategySingle returns a NextRollStrategy which rolls toward HEAD of a given branch,
// one Revision at a time.
func StrategySingle() NextRollStrategy {
	return &singleStrategy{}
}
