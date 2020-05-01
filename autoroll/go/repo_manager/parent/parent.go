package parent

/*
   Package parent contains implementations of the Parent interface.
*/

import (
	"context"

	"go.skia.org/infra/autoroll/go/revision"
)

// Parent represents a git repo (or other destination) which depends on a Child
// and is capable of producing rolls.
type Parent interface {
	// Update returns the pinned version of the dependency at the most
	// recent revision of the Parent. For implementations which use local
	// checkouts, this implies a sync.
	Update(context.Context) (string, error)

	// CreateNewRoll uploads a CL which updates the pinned version of the
	// dependency to the given Revision.
	CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, dryRun bool, commitMsg string) (int64, error)
}
