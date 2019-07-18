package strategy

import (
	"context"
	"fmt"

	"go.skia.org/infra/autoroll/go/revision"
)

const (
	ROLL_STRATEGY_BATCH = "batch"
	// TODO(rmistry): Rename to "batch of " + N_COMMITS ?
	ROLL_STRATEGY_N_BATCH = "n_batch"
	ROLL_STRATEGY_SINGLE  = "single"

	// The number of commits to use in ROLL_STRATEGY_N_BATCH.
	N_COMMITS = 20
)

// NextRollStrategy is an interface for modules which determine what the next roll
// revision should be.
type NextRollStrategy interface {
	// Return the next roll revision, given the list of not-yet-rolled
	// commits in reverse chronological order. Returning the empty string
	// implies that we are up-to-date.
	GetNextRollRev(context.Context, []*revision.Revision) (*revision.Revision, error)
}

// Return the NextRollStrategy indicated by the given string.
func GetNextRollStrategy(strategy string) (NextRollStrategy, error) {
	switch strategy {
	case ROLL_STRATEGY_BATCH:
		return StrategyHead(), nil
	case ROLL_STRATEGY_N_BATCH:
		return StrategyNCommits(), nil
	case ROLL_STRATEGY_SINGLE:
		return StrategySingle(), nil
	default:
		return nil, fmt.Errorf("Unknown roll strategy %q", strategy)
	}
}

// headStrategy is a NextRollStrategy which always rolls to HEAD of a given branch.
type headStrategy struct{}

// See documentation for NextRollStrategy interface.
func (s *headStrategy) GetNextRollRev(ctx context.Context, notRolled []*revision.Revision) (*revision.Revision, error) {
	if len(notRolled) > 0 {
		// Commits are listed in reverse chronological order.
		return notRolled[0], nil
	}
	return nil, nil
}

// StrategyHead returns a NextRollStrategy which always rolls to HEAD of a given branch.
func StrategyHead() NextRollStrategy {
	return &headStrategy{}
}

// nCommitsStrategy is a NextRollStrategy which always rolls to maximum N commits of a
// given branch.
type nCommitsStrategy struct{}

// See documentation for NextRollStrategy interface.
func (s *nCommitsStrategy) GetNextRollRev(ctx context.Context, notRolled []*revision.Revision) (*revision.Revision, error) {
	if len(notRolled) > N_COMMITS {
		return notRolled[len(notRolled)-N_COMMITS], nil
	} else if len(notRolled) > 0 {
		return notRolled[0], nil
	} else {
		return nil, nil
	}
}

// StrategyNCommits returns a NextRollStrategy which always rolls to maximum N commits of a
// given branch.
func StrategyNCommits() NextRollStrategy {
	return &nCommitsStrategy{}
}

// singleStrategy is a NextRollStrategy which rolls toward HEAD of a given branch, one
// commit at a time.
type singleStrategy struct{}

// See documentation for NextRollStrategy interface.
func (s *singleStrategy) GetNextRollRev(ctx context.Context, notRolled []*revision.Revision) (*revision.Revision, error) {
	if len(notRolled) > 0 {
		return notRolled[len(notRolled)-1], nil
	}
	return nil, nil
}

// StrategySingle returns a NextRollStrategy which rolls toward HEAD of a given branch,
// one commit at a time.
func StrategySingle() NextRollStrategy {
	return &singleStrategy{}
}
