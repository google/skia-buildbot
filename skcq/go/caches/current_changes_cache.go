package caches

import (
	"context"

	"go.skia.org/infra/skcq/go/types"
)

type CurrentChangesCache interface {
	// Get returns the current cache.
	Get() map[string]*types.CurrentlyProcessingChange

	// Add creates an entry in the changes cache if it does not already
	// exist. It returns the cqStartTime of this attempt and a boolean
	// indicating whether this is a new CQ attempt.
	Add(ctx context.Context, changeEquivalentPatchset, changeSubject, changeOwner, repo, branch string, dryRun, internal bool, changeID, latestPatchsetID int64) (int64, bool, error)

	// Remove removes the specified change from the cache.
	Remove(ctx context.Context, changeEquivalentPatchset string) error
}
