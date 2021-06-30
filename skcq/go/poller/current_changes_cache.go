package poller

import (
	"context"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/db"
	"go.skia.org/infra/skcq/go/types"
)

var (
	// Cache of CLs + Patchsets and their CQDetails.
	// This is to keep track of which CLs leave the CQ. Their verifiers are cleaned up when this happens.
	// This is also used to keep track of the start time of when the CQ started processing this CL. This
	// is useful to determine which CQ try jobs should be considered.
	CurrentChangesCache = map[string]*types.CurrentlyProcessingChange{}
)

// AddToChangesCache creates an entry in the changes cache if it does not already exist.
// It returns the cqStartTime of this attempt and a boolean indicating whether this is a
// new CQ attempt.
func AddToChangesCache(ctx context.Context, changeEquivalentPatchset, changeSubject, changeOwner, repo, branch string, dbClient *db.FirestoreDB, dryRun, internal bool, changeID, equivalentPatchsetID int64) (int64, bool) {
	newEntry := false
	cqStartTime := time.Now().Unix()
	// Add to the changes cache if it is already not there.
	cqRecord, ok := CurrentChangesCache[changeEquivalentPatchset]
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
		CurrentChangesCache[changeEquivalentPatchset] = cqRecord
		updateDB(ctx, dbClient)
		newEntry = true
	} else {
		cqStartTime = cqRecord.StartTime
	}
	return cqStartTime, newEntry
}

func RemoveFromChangesCache(ctx context.Context, changeEquivalentPatchset string, dbClient *db.FirestoreDB) {
	if _, ok := CurrentChangesCache[changeEquivalentPatchset]; ok {
		delete(CurrentChangesCache, changeEquivalentPatchset)
		updateDB(ctx, dbClient)
	}
}

// uppdateDB updates the DB with the current changes cache. Errors (if any) are
// logged and not returned.
func updateDB(ctx context.Context, dbClient *db.FirestoreDB) {
	// buf := bytes.NewBuffer(nil)
	// if err := gob.NewEncoder(buf).Encode(CurrentChangesCache); err != nil {
	// 	sklog.Errorf("Error encoding the current changes cache: %s", err)
	// }
	// if err := dbClient.PutCurrentChanges(ctx, buf.Bytes()); err != nil {
	// 	sklog.Errorf("Error updating the current changes cache: %s", err)
	// }
	if err := dbClient.PutCurrentChanges(ctx, CurrentChangesCache); err != nil {
		sklog.Errorf("Error updating the current changes cache: %s", err)
	}
}
