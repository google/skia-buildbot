package caches

import (
	"context"

	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/skcq/go/db"
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

type CurrentChangesCacheImpl struct {
	dbClient            db.DB
	currentChangesCache map[string]*types.CurrentlyProcessingChange
}

func GetCurrentChangesCache(ctx context.Context, dbClient db.DB) (CurrentChangesCache, error) {
	currentChangesCache, err := dbClient.GetCurrentChanges(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not get current changes")
	}
	return &CurrentChangesCacheImpl{
		dbClient:            dbClient,
		currentChangesCache: currentChangesCache,
	}, nil
}

// Get implements the CurrentChangesCache interface.
func (c *CurrentChangesCacheImpl) Get() map[string]*types.CurrentlyProcessingChange {
	return c.currentChangesCache
}

// Add implements the CurrentChangesCache interface.
func (c *CurrentChangesCacheImpl) Add(ctx context.Context, changeEquivalentPatchset, changeSubject, changeOwner, repo, branch string, dryRun, internal bool, changeID, latestPatchsetID int64) (int64, bool, error) {
	newEntry := false
	cqStartTime := now.Now(ctx).Unix()
	// Add to the changes cache if it is already not there.
	cqRecord, ok := c.currentChangesCache[changeEquivalentPatchset]
	if ok && cqRecord.DryRun != dryRun {
		// Abandon the previous attempt before we put the new one in.
		if err := c.dbClient.UpdateChangeAttemptAsAbandoned(ctx, cqRecord.ChangeID, cqRecord.LatestPatchsetID, db.GetChangesCol(internal), cqRecord.StartTs); err != nil {
			return -1, false, skerr.Wrapf(err, "Error abandoning change attempt")
		}
		ok = false
	}
	if !ok {
		cqRecord = &types.CurrentlyProcessingChange{
			ChangeID:         changeID,
			LatestPatchsetID: latestPatchsetID,
			Repo:             repo,
			Branch:           branch,
			StartTs:          cqStartTime,
			DryRun:           dryRun,
			Internal:         internal,
			ChangeSubject:    changeSubject,
			ChangeOwner:      changeOwner,
		}
		c.currentChangesCache[changeEquivalentPatchset] = cqRecord
		if err := c.dbClient.PutCurrentChanges(ctx, c.currentChangesCache); err != nil {
			return -1, false, skerr.Wrapf(err, "Error persisting the current changes cache")
		}
		newEntry = true
	} else {
		cqStartTime = cqRecord.StartTs
	}
	return cqStartTime, newEntry, nil
}

// Remove implements the CurrentChangesCache interface.
func (c *CurrentChangesCacheImpl) Remove(ctx context.Context, changeEquivalentPatchset string) error {
	if _, ok := c.currentChangesCache[changeEquivalentPatchset]; ok {
		delete(c.currentChangesCache, changeEquivalentPatchset)
		if err := c.dbClient.PutCurrentChanges(ctx, c.currentChangesCache); err != nil {
			return skerr.Wrapf(err, "Error persisting the current changes cache")
		}
	}
	return nil
}
