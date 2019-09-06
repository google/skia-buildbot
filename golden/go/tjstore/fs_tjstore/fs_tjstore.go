// Package fs_tjstore implements the tjstore.Store interface with
// a FireStore backend.
package fs_tjstore

import (
	"context"
	"errors"
	"time"

	"cloud.google.com/go/firestore"
	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/clstore"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/tjstore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// These are the collections in Firestore.
	tryJobCollection   = "tjstore_tryjob"
	tjResultCollection = "tjstore_result"
	paramsCollection   = "tjstore_params"

	// These are the fields we query by
	changeListIDField = "clid"
	crsField          = "crs"
	patchSetIDField   = "psid"

	maxAttempts      = 10
	maxOperationTime = time.Minute
)

// StoreImpl is the firestore based implementation of tjstore.
type StoreImpl struct {
	client  *ifirestore.Client
	cisName string
}

// New returns a new StoreImpl
func New(client *ifirestore.Client, cisName string) *StoreImpl {
	return &StoreImpl{
		client:  client,
		cisName: cisName,
	}
}

// tryJobEntry represents how a TryJob is stored in FireStore.
type tryJobEntry struct {
	SystemID string `firestore:"systemid"`
	CISystem string `firestore:"cis"`

	CRSystem     string `firestore:"crs"`
	ChangeListID string `firestore:"clid"`
	PatchSetID   string `firestore:"psid"`

	DisplayName string    `firestore:"displayname"`
	Updated     time.Time `firestore:"updated"`
}

// GetTryJob implements the tjstore.Store interface.
func (s *StoreImpl) GetTryJob(ctx context.Context, id string) (ci.TryJob, error) {
	defer metrics2.FuncTimer().Stop()
	fID := s.tryJobFirestoreID(id)
	doc, err := s.client.Collection(tryJobCollection).Doc(fID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return ci.TryJob{}, clstore.ErrNotFound
		}
		return ci.TryJob{}, skerr.Wrapf(err, "retrieving TryJob %s from firestore", doc.Ref.ID)
	}
	if doc == nil {
		return ci.TryJob{}, clstore.ErrNotFound
	}

	tje := tryJobEntry{}
	if err := doc.DataTo(&tje); err != nil {
		id := doc.Ref.ID
		return ci.TryJob{}, skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal %s tryjob with id %s", s.cisName, id)
	}
	tj := ci.TryJob{
		SystemID:    tje.SystemID,
		DisplayName: tje.DisplayName,
		Updated:     tje.Updated,
	}

	return tj, nil
}

// tryJobFirestoreID returns the id for a given TryJob in a given CIS - this allows us to
// look up a document by id w/o having to perform a query.
func (s *StoreImpl) tryJobFirestoreID(tjID string) string {
	return tjID + "_" + s.cisName
}

// GetTryJobs implements the tjstore.Store interface.
func (s *StoreImpl) GetTryJobs(ctx context.Context, psID tjstore.CombinedPSID) ([]ci.TryJob, error) {
	defer metrics2.FuncTimer().Stop()
	q := s.client.Collection(tryJobCollection).Where(crsField, "==", psID.CRS).
		Where(changeListIDField, "==", psID.CL).Where(patchSetIDField, "==", psID.PS)

	var xtj []ci.TryJob

	err := s.client.IterDocs("GetTryJobs", psID.Key(), q, maxAttempts, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		entry := tryJobEntry{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal entry with id %s", id)
		}
		xtj = append(xtj, ci.TryJob{
			SystemID:    entry.SystemID,
			DisplayName: entry.DisplayName,
			Updated:     entry.Updated,
		})
		return nil
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching tryjobs for cl/ps %s", psID.Key())
	}

	// Sort after the fact to save us a composite index and due to the fact that the amount of
	// TryJobs per PatchSet should be small (< 100).
	ci.SortTryJobsByName(xtj)

	return xtj, nil
}

// GetResults implements the tjstore.Store interface.
func (s *StoreImpl) GetResults(ctx context.Context, psID tjstore.CombinedPSID) ([]tjstore.TryJobResult, error) {
	return []tjstore.TryJobResult{}, errors.New("not impl")
}

// PutTryJob implements the tjstore.Store interface.
func (s *StoreImpl) PutTryJob(ctx context.Context, psID tjstore.CombinedPSID, tj ci.TryJob) error {
	defer metrics2.FuncTimer().Stop()
	fID := s.tryJobFirestoreID(tj.SystemID)
	cd := s.client.Collection(tryJobCollection).Doc(fID)
	record := tryJobEntry{
		SystemID:     tj.SystemID,
		CISystem:     s.cisName,
		CRSystem:     psID.CRS,
		ChangeListID: psID.CL,
		PatchSetID:   psID.PS,
		DisplayName:  tj.DisplayName,
		Updated:      tj.Updated,
	}
	_, err := s.client.Set(cd, record, maxAttempts, maxOperationTime)
	if err != nil {
		return skerr.Wrapf(err, "could not write TryJob %v to tjstore", tj)
	}
	return nil
}

// PutResults implements the tjstore.Store interface.
func (s *StoreImpl) PutResults(ctx context.Context, psID tjstore.CombinedPSID, r []tjstore.TryJobResult) error {
	return errors.New("not impl")
}

// System implements the tjstore.Store interface.
func (s *StoreImpl) System() string {
	return s.cisName
}

// Make sure StoreImpl fulfills the tjstore.Store interface.
var _ tjstore.Store = (*StoreImpl)(nil)
