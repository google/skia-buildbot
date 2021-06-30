package db

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/skcq/go/types"
)

const (
	// For accessing Firestore.
	defaultAttempts  = 3
	getSingleTimeout = 10 * time.Second
	putSingleTimeout = 10 * time.Second

	// Names of Collections and Documents.
	snapshotsCol              = "Snapshots"
	currentChangesSnapshotDoc = "CurrentChangesSnapshot"

	publicChangesCol   = "PublicChanges"
	internalChangesCol = "InternalChanges"
)

// FirestoreDB uses Cloud Firestore for storage.
type FirestoreDB struct {
	client *firestore.Client
	// mtx to control access to firestore
	mtx sync.RWMutex
}

// New returns an instance of FirestoreDB.
func New(ctx context.Context, ts oauth2.TokenSource, fsNamespace, fsProjectId string) (*FirestoreDB, error) {
	// Instantiate firestore.
	fsClient, err := firestore.NewClient(ctx, fsProjectId, "skcq", fsNamespace, ts)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not init firestore")
	}
	return &FirestoreDB{
		client: fsClient,
	}, nil
}

// GetCurrentChangesSnapshot returns the changes that were processed by the last iteration.
// If none is found in DB then an empty map is returned.
func (f *FirestoreDB) GetCurrentChanges(ctx context.Context) (map[string]*types.CurrentlyProcessingChange, error) {
	currentChanges := map[string]*types.CurrentlyProcessingChange{}
	col := f.client.Collection(snapshotsCol)
	if col == nil {
		return currentChanges, nil
	}
	docRef := col.Doc(currentChangesSnapshotDoc)
	if docRef == nil {
		return currentChanges, nil
	}
	snapshot, err := docRef.Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return currentChanges, nil
		}
		return nil, err
	}
	if err := snapshot.DataTo(&currentChanges); err != nil {
		return nil, err
	}

	return currentChanges, nil
}

// GetCurrentChangesSnapshot returns the changes that were processed by the last iteration.
func (f *FirestoreDB) PutCurrentChanges(ctx context.Context, currentChangesCache interface{}) error {
	// TODO(rmistry): Use the mutex.
	col := f.client.Collection(snapshotsCol)
	if _, setErr := f.client.Set(ctx, col.Doc(currentChangesSnapshotDoc), currentChangesCache, defaultAttempts, putSingleTimeout); setErr != nil {
		return skerr.Fmt("Could not set CurrentChangesSnapshot: %s", setErr)
	}

	return nil
}

func (f *FirestoreDB) GetChangeAttempts(ctx context.Context, changeID, patchsetID int64, internal bool) (*types.ChangeAttempts, error) {
	col := f.client.Collection(getChangesColName(internal))
	if col == nil {
		// Collection does not exist yet, this will be the first entry.
		return nil, nil
	}
	docName := getChangeAttemptsDocName(changeID, patchsetID)
	docRef := col.Doc(docName)
	if docRef == nil {
		// Doc does not exist yet, this will be the first entry.
		return nil, nil
	}
	ch, err := docRef.Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			// Doc does not exist yet, this will be the first entry.
			return nil, nil
		}
		return nil, err
	}
	changeAttempts := types.ChangeAttempts{}
	if err := ch.DataTo(&changeAttempts); err != nil {
		return nil, err
	}
	return &changeAttempts, nil
}

// // GetChangeAttempt returns a ChangeAttempt that matches the specified PatchStart time.
// func (f *FirestoreDB) GetChangeAttempt(ctx context.Context, changeID string, internal bool, patchStart int64) (*types.ChangeAttempt, error) {
// 	changesColName := publicChangesCol
// 	if internal {
// 		changesColName = internalChangesCol
// 	}
// 	changeAttempts, err := f.GetChangeAttempts(ctx, changeID, changesColName)
// 	if err != nil {
// 		return nil, skerr.Fmt("Error getting change attempts of %s from DB: %s", changeID, err)
// 	}
// 	if changeAttempts == nil || len(changeAttempts.Attempts) == 0 {
// 		return nil, nil
// 	}
// 	for _, ca := range changeAttempts.Attempts {
// 		if ca.PatchStart == patchStart {
// 			return ca, nil
// 		}
// 	}
// 	return nil, nil
// }

// Utility function to return which changes col name to use based on if the
// change is internal or not.
func getChangesColName(internal bool) string {
	changesColName := publicChangesCol
	if internal {
		changesColName = internalChangesCol
	}
	return changesColName
}

// Utility function to blah blah
func getChangeAttemptsDocName(changeID, patchsetID int64) string {
	return fmt.Sprintf("%d_%d", changeID, patchsetID)
}

func (f *FirestoreDB) UpdateChangeAttemptAsAbandoned(ctx context.Context, changeID, patchsetID int64, internal bool, patchStart int64) error {
	changeAttempts, err := f.GetChangeAttempts(ctx, changeID, patchsetID, internal)
	if err != nil {
		return skerr.Fmt("Error getting change attempts of %d/%d from DB: %s", changeID, patchsetID, err)
	}
	if changeAttempts == nil || len(changeAttempts.Attempts) == 0 {
		return nil
	}
	for _, ca := range changeAttempts.Attempts {
		if ca.PatchStart == patchStart {
			ca.PatchStop = time.Now().Unix()
			ca.CQAbandoned = true
			col := f.client.Collection(getChangesColName(internal))
			docName := getChangeAttemptsDocName(changeID, patchsetID)
			if _, err := f.client.Set(ctx, col.Doc(docName), changeAttempts, defaultAttempts, putSingleTimeout); err != nil {
				return skerr.Fmt("Could not set ChangeAttempts: %s", err)
			}
			return nil
		}
	}
	return skerr.Fmt("Could not find ChangeAttempt with ID %d/%d and start time of %d", changeID, patchsetID, patchStart)
}

func (f *FirestoreDB) PutChangeAttempt(ctx context.Context, newChangeAttempt *types.ChangeAttempt, internal bool) error {
	changeAttempts, err := f.GetChangeAttempts(ctx, newChangeAttempt.ChangeID, newChangeAttempt.PatchsetID, internal)
	if err != nil {
		return skerr.Fmt("Error getting change attempts of %d/%d from DB: %s", newChangeAttempt.ChangeID, newChangeAttempt.PatchsetID, err)
	}

	if changeAttempts == nil || len(changeAttempts.Attempts) == 0 {
		changeAttempts = &types.ChangeAttempts{}
		changeAttempts.Attempts = []*types.ChangeAttempt{newChangeAttempt}
	} else {
		// Go through the existing changeAttempts and see if this changeAttempt exists.
		exists := false
		for index, existingChangeAttempt := range changeAttempts.Attempts {
			if existingChangeAttempt.PatchStart == newChangeAttempt.PatchStart {
				exists = true
				// If it exists then we have to replace the change Attempt.
				// But first loop through the verifiers and if they have the same state then
				// do not replace their end time.
				for _, newVerifier := range newChangeAttempt.VerifiersStatuses {
					for _, existingVerifier := range existingChangeAttempt.VerifiersStatuses {
						if existingVerifier.Name == newVerifier.Name {
							if existingVerifier.State == types.VerifierSuccessState && newVerifier.State == types.VerifierSuccessState {
								// Reuse the end time of the old verifier.
								newVerifier.Stop = existingVerifier.Stop
							}
						}
					}
				}
				changeAttempts.Attempts[index] = newChangeAttempt
				break
			}
		}
		if !exists {
			changeAttempts.Attempts = append(changeAttempts.Attempts, newChangeAttempt)
		}
	}

	// Commit to DB.
	col := f.client.Collection(getChangesColName(internal))
	docName := getChangeAttemptsDocName(newChangeAttempt.ChangeID, newChangeAttempt.PatchsetID)
	if _, setErr := f.client.Set(ctx, col.Doc(docName), changeAttempts, defaultAttempts, putSingleTimeout); setErr != nil {
		return skerr.Fmt("Could not set ChangeAttempts: %s", setErr)
	}

	return nil
}
