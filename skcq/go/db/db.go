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
)

const (
	// For accessing Firestore.
	defaultAttempts  = 3
	getSingleTimeout = 10 * time.Second
	putSingleTimeout = 10 * time.Second

	// Names of Collections
	// runIdsCol = "RunIds"
)

// FirestoreDB uses Cloud Firestore for storage.
type FirestoreDB struct {
	client *firestore.Client
	// mtx to control access to firestore
	mtx sync.RWMutex
}

// CurrentChangesSnapshot is the type that will be stored in FirestoreDB.
type CurrentChangesSnapshot struct {
	Data []byte `json:"data"`
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
func (f *FirestoreDB) GetCurrentChanges(ctx context.Context) (*CurrentChangesSnapshot, error) {
	currentChanges := &CurrentChangesSnapshot{}
	col := f.client.Collection("Snapshots")
	if col == nil {
		return nil, nil
	}
	docRef := col.Doc("CurrentChangesSnapshot")
	if docRef == nil {
		// If not found then return a nil currentChanges.
		return nil, nil
	}
	snapshot, err := docRef.Get(ctx)
	if err != nil {
		fmt.Println("HERE")
		fmt.Println(err)
		if status.Code(err) == codes.NotFound {
			// If not found then return an empty currentChanges.
			return nil, nil
		}
		return nil, err
	}
	if err := snapshot.DataTo(currentChanges); err != nil {
		return nil, err
	}

	return currentChanges, nil
}

// GetCurrentChangesSnapshot returns the changes that were processed by the last iteration.
func (f *FirestoreDB) PutCurrentChanges(ctx context.Context, data []byte) error {
	// TODO(rmistry): Use the mutex.
	currentChanges := &CurrentChangesSnapshot{
		Data: data,
	}
	fmt.Println("HERE HERE")
	fmt.Println(data)
	fmt.Println(currentChanges)
	fmt.Println(f.client.Collection("Snapshots"))
	// if _, setErr := f.client.Collection("CurrentChangesSnapshot").Set(ctx, currentChanges); setErr != nil {
	// 	return skerr.Fmt("Could not set CurrentChangesSnapshot: %s", setErr)
	// }
	col := f.client.Collection("Snapshots")
	if _, setErr := f.client.Set(ctx, col.Doc("CurrentChangesSnapshot"), currentChanges, defaultAttempts, putSingleTimeout); setErr != nil {
		return skerr.Fmt("Could not set CurrentChangesSnapshot: %s", setErr)
	}

	return nil
}

// // getAllLatestCounts returns the latest counts data for all clients.
// func (f *FirestoreDB) getAllLatestCounts(ctx context.Context) (*types.IssueCountsData, error) {
// 	countData := &types.IssueCountsData{}
// 	clients := f.client.Collections(ctx)
// 	for {
// 		c, err := clients.Next()
// 		if err == iterator.Done {
// 			break
// 		} else if err != nil {
// 			return nil, err
// 		} else if c.ID == "RunIds" {
// 			continue
// 		}
// 		qcd, err := f.getLatestCountsFromClient(ctx, c)
// 		if err != nil {
// 			return nil, skerr.Wrapf(err, "could not get all sources counts from db")
// 		}
// 		countData.Merge(qcd)
// 	}
// 	return countData, nil
// }
