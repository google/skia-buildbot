package poller

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/db"
)

var (
	// This is an optimization we can consider one day later.
	/*
		// Cache in-memory the list of supported projects + branches.
		// We are caching in-memory and not on disk because calculating if a project+branch is supported
		// is not much work.
		// TODO(rmistry): If support is removed for a project+branch then they will not be removed till the pod
		// is restarted. Bring up a separate go routine to periodically verify that everything in here is still
		// supported? Refresh the config at every tick here.
		// TODO(rmistry): Move this to the config package and have it be maintained there!
		ProjectsBranchesConfigCache = map[string]*config.SkCQCfg{}
	*/

	// Cache of CLs + Patchsets and their CQDetails.
	// This is to keep track of which CLs leave the CQ. Their verifiers are cleaned up when this happens.
	// This is also used to keep track of the start time of when the CQ started processing this CL. This
	// is useful to determine which CQ try jobs should be considered.

	// One issue - if CQ is restarted then this in-memory thing will be lost. and any cq job failures will not be considered
	// prior to this point?
	// is it better to persist startTime?
	// GOB THIS BEFORE LAUNCH.
	CurrentChangesCache = map[string]*CQRecord{}
)

// TODO(rmistry): Put this *cache* in a separate file!
type CQRecord struct {
	// The time the CQ first looked at this change.
	// Uses unix epoch time.
	StartTime int64
}

func AddToChangesCache(ctx context.Context, changeID string, dbClient *db.FirestoreDB, cqStartTime int64) {
	// Add to the changes cache if it is already not there.
	cqRecord, ok := CurrentChangesCache[changeID]
	if !ok {
		cqRecord = &CQRecord{
			StartTime: cqStartTime,
		}
	}
	CurrentChangesCache[changeID] = cqRecord
	updateDB(ctx, dbClient)
}

func RemoveFromChangesCache(ctx context.Context, changeID string, runCleanup bool, dbClient *db.FirestoreDB) {
	fmt.Printf("\nREMOVING %s from ChangesCache and running cleanup: %t", changeID, runCleanup)
	if cqRecord, ok := CurrentChangesCache[changeID]; ok {
		if runCleanup {
			fmt.Println(cqRecord)
			// TODO(rmistry): Find verifiers here and find gerrit.ChangeInfo.
			// for _, v := range cqRecord.changeVerifiers {
			// 	v.Cleanup(ctx, cqRecord.ci)
			// }
		}
		delete(CurrentChangesCache, changeID)
		updateDB(ctx, dbClient)
	}
}

// uppdateDB updates the DB with the current changes cache. Errors (if any) are
// logged and not returned.
func updateDB(ctx context.Context, dbClient *db.FirestoreDB) {
	buf := bytes.NewBuffer(nil)
	if err := gob.NewEncoder(buf).Encode(CurrentChangesCache); err != nil {
		sklog.Errorf("Error encoding the current changes cache: %s", err)
	}
	if err := dbClient.PutCurrentChanges(ctx, buf.Bytes()); err != nil {
		sklog.Errorf("Error updating the current changes cache: %s", err)
	}
}
