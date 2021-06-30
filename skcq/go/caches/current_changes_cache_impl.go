package caches

import (
	"context"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/skcq/go/db"
	"go.skia.org/infra/skcq/go/types"
)

type CurrentChangesCacheImpl struct {
	dbClient            *db.FirestoreDB
	currentChangesCache map[string]*types.CurrentlyProcessingChange
}

func GetCurrentChangesCache(ctx context.Context, dbClient *db.FirestoreDB) (CurrentChangesCache, error) {
	currentChangesCache, err := dbClient.GetCurrentChanges(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not get current changes")
	}
	return &CurrentChangesCacheImpl{
		dbClient:            dbClient,
		currentChangesCache: currentChangesCache,
	}, nil
}

func (c *CurrentChangesCacheImpl) Get() map[string]*types.CurrentlyProcessingChange {
	return c.currentChangesCache
}

// Add implements the CurrentChangesCache interface.
func (c *CurrentChangesCacheImpl) Add(ctx context.Context, changeEquivalentPatchset, changeSubject, changeOwner, repo, branch string, dryRun, internal bool, changeID, equivalentPatchsetID int64) (int64, bool, error) {
	newEntry := false
	cqStartTime := time.Now().Unix()
	// Add to the changes cache if it is already not there.
	cqRecord, ok := c.currentChangesCache[changeEquivalentPatchset]
	if !ok {
		cqRecord = &types.CurrentlyProcessingChange{
			ChangeID:             changeID,
			EquivalentPatchsetID: equivalentPatchsetID,
			Repo:                 repo,
			Branch:               branch,
			StartTime:            cqStartTime,
			DryRun:               dryRun,
			Internal:             internal,
			ChangeSubject:        changeSubject,
			ChangeOwner:          changeOwner,
		}
		c.currentChangesCache[changeEquivalentPatchset] = cqRecord
		if err := c.dbClient.PutCurrentChanges(ctx, c.currentChangesCache); err != nil {
			return -1, false, skerr.Wrapf(err, "Error persisting the current changes cache")
		}
		newEntry = true
	} else {
		cqStartTime = cqRecord.StartTime
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
