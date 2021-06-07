package db

import (
	"context"
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
)

// FirestoreDB uses Cloud Firestore for storage.
type FirestoreDB struct {
	client *firestore.Client
	// mtx to control access to firestore
	mtx sync.RWMutex
}

// CurrentChangesSnapshot is the type that will be stored in FirestoreDB.
type ChangeData struct {
	PatchStart         time.Time         `json:"created"`
	PatchStop          time.Time         `json:"stop"`
	PatchCommitted     time.Time         `json:"committed"`
	SubmittableChanges []string          `json:"submittable_changes"`
	VerifiersStatus    []*VerifierStatus `json:"verifiers_status"`
}

type VerifierStatus struct {
	Name      string    `json:"name"`
	Start     time.Time `json:"start"`
	Waiting   time.Time `json:"waiting"`
	Stop      time.Time `json:"stop"`
	Reason    string    `json:"reason"`
	Succeeded bool      `json:"succeeded"`
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
func (f *FirestoreDB) GetCurrentChanges(ctx context.Context) (map[string]*types.CQRecord, error) {
	currentChanges := map[string]*types.CQRecord{}
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
