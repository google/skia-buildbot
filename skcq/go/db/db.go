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

type ChangesCol string

const (
	// For accessing Firestore.
	defaultAttempts  = 3
	getSingleTimeout = 10 * time.Second
	putSingleTimeout = 10 * time.Second

	// Names of Collections and Documents.
	snapshotsCol              = "Snapshots"
	currentChangesSnapshotDoc = "CurrentChangesSnapshot"

	publicChangesCol   ChangesCol = "PublicChanges"
	internalChangesCol ChangesCol = "InternalChanges"
)

type DB interface {
	// GetCurrentChanges returns the changes that were processed by the last
	// iteration. If none is found in DB then an empty map is returned.
	GetCurrentChanges(ctx context.Context) (map[string]*types.CurrentlyProcessingChange, error)

	// PutCurrentChanges returns the changes that were processed by the last
	// iteration.
	PutCurrentChanges(ctx context.Context, currentChangesCache interface{}) error

	// GetChangeAttempts returns all ChangeAttempts in the DB for this
	// change+patchset.
	GetChangeAttempts(ctx context.Context, changeID, patchsetID int64, changesCol ChangesCol) (*types.ChangeAttempts, error)

	// UpdateChangeAttemptAsAbandoned marks the specified change attempt as
	// abandoned.
	UpdateChangeAttemptAsAbandoned(ctx context.Context, changeID, patchsetID int64, changesCol ChangesCol, patchStart int64) error

	// PutChangeAttempts adds the specified change attempt to the DB.
	PutChangeAttempt(ctx context.Context, newChangeAttempt *types.ChangeAttempt, changesCol ChangesCol) error
}

// FirestoreDB uses Cloud Firestore for storage and implements the DB
// interface.
type FirestoreDB struct {
	client *firestore.Client
	// mtx to control access to firestore
	mtx sync.RWMutex
}

// New returns an instance of FirestoreDB.
func New(ctx context.Context, ts oauth2.TokenSource, fsNamespace, fsProjectId string) (DB, error) {
	// Instantiate firestore.
	fsClient, err := firestore.NewClient(ctx, fsProjectId, "skcq", fsNamespace, ts)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not init firestore")
	}
	return &FirestoreDB{
		client: fsClient,
	}, nil
}

// GetCurrentChanges implements the DB interface.
func (f *FirestoreDB) GetCurrentChanges(ctx context.Context) (map[string]*types.CurrentlyProcessingChange, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

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

// PutCurrentChanges implements the DB interface.
func (f *FirestoreDB) PutCurrentChanges(ctx context.Context, currentChangesCache interface{}) error {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	col := f.client.Collection(snapshotsCol)
	if _, setErr := f.client.Set(ctx, col.Doc(currentChangesSnapshotDoc), currentChangesCache, defaultAttempts, putSingleTimeout); setErr != nil {
		return skerr.Fmt("Could not set CurrentChangesSnapshot: %s", setErr)
	}

	return nil
}

// GetChangeAttempts implements the DB interface.
func (f *FirestoreDB) GetChangeAttempts(ctx context.Context, changeID, patchsetID int64, changesCol ChangesCol) (*types.ChangeAttempts, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	col := f.client.Collection(string(changesCol))
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

// UpdateChangeAttemptAsAbandoned implements the DB interface.
func (f *FirestoreDB) UpdateChangeAttemptAsAbandoned(ctx context.Context, changeID, patchsetID int64, changesCol ChangesCol, patchStart int64) error {
	changeAttempts, err := f.GetChangeAttempts(ctx, changeID, patchsetID, changesCol)
	if err != nil {
		return skerr.Fmt("Error getting change attempts of %d/%d from DB: %s", changeID, patchsetID, err)
	}
	if changeAttempts == nil || len(changeAttempts.Attempts) == 0 {
		return nil
	}

	f.mtx.Lock()
	defer f.mtx.Unlock()

	for _, ca := range changeAttempts.Attempts {
		if ca.PatchStartTs == patchStart {
			ca.PatchStopTs = time.Now().Unix()
			ca.CQAbandoned = true
			col := f.client.Collection(string(changesCol))
			docName := getChangeAttemptsDocName(changeID, patchsetID)
			if _, err := f.client.Set(ctx, col.Doc(docName), changeAttempts, defaultAttempts, putSingleTimeout); err != nil {
				return skerr.Fmt("Could not set ChangeAttempts: %s", err)
			}
			return nil
		}
	}
	return skerr.Fmt("Could not find ChangeAttempt with ID %d/%d and start time of %d", changeID, patchsetID, patchStart)
}

// PutChangeAttempt implements the DB interface.
func (f *FirestoreDB) PutChangeAttempt(ctx context.Context, newChangeAttempt *types.ChangeAttempt, changesCol ChangesCol) error {
	changeAttempts, err := f.GetChangeAttempts(ctx, newChangeAttempt.ChangeID, newChangeAttempt.PatchsetID, changesCol)
	if err != nil {
		return skerr.Fmt("Error getting change attempts of %d/%d from DB: %s", newChangeAttempt.ChangeID, newChangeAttempt.PatchsetID, err)
	}

	f.mtx.Lock()
	defer f.mtx.Unlock()

	if changeAttempts == nil || len(changeAttempts.Attempts) == 0 {
		changeAttempts = &types.ChangeAttempts{}
		changeAttempts.Attempts = []*types.ChangeAttempt{newChangeAttempt}
	} else {
		// Go through the existing changeAttempts and see if this
		// changeAttempt exists.
		exists := false
		for index, existingChangeAttempt := range changeAttempts.Attempts {
			if existingChangeAttempt.PatchStartTs == newChangeAttempt.PatchStartTs {
				exists = true
				// If it exists then we have to replace the
				// change Attempt. But first loop through the
				// verifiers and if they have the same state
				// then do not replace their end time.
				for _, newVerifier := range newChangeAttempt.VerifiersStatuses {
					for _, existingVerifier := range existingChangeAttempt.VerifiersStatuses {
						if existingVerifier.Name == newVerifier.Name {
							if existingVerifier.State == types.VerifierSuccessState && newVerifier.State == types.VerifierSuccessState {
								// Reuse the end time of the old verifier.
								newVerifier.StopTs = existingVerifier.StopTs
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
	col := f.client.Collection(string(changesCol))
	docName := getChangeAttemptsDocName(newChangeAttempt.ChangeID, newChangeAttempt.PatchsetID)
	if _, setErr := f.client.Set(ctx, col.Doc(docName), changeAttempts, defaultAttempts, putSingleTimeout); setErr != nil {
		return skerr.Fmt("Could not set ChangeAttempts: %s", setErr)
	}

	return nil
}

// Utility function to return which changes col name to use based on if the
// change is internal or not.
func GetChangesCol(internal bool) ChangesCol {
	changesCol := publicChangesCol
	if internal {
		changesCol = internalChangesCol
	}
	return changesCol
}

// Utility function to return the name of the ChangeAttempts document.
func getChangeAttemptsDocName(changeID, patchsetID int64) string {
	return fmt.Sprintf("%d_%d", changeID, patchsetID)
}
