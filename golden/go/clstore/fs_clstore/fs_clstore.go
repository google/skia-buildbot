// Package fs_clstore implements the clstore.Store interface with
// a FireStore backend.
package fs_clstore

import (
	"context"
	"errors"
	"time"

	"cloud.google.com/go/firestore"
	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
)

const (
	// These are the collections in Firestore.
	changelistCollection = "clstore_changelist"
	patchsetCollection   = "clstore_patchset"

	// These are the fields we query by
	systemIDField = "systemid"
	systemField   = "system"
	clIDField     = "changelistid"
	updatedField  = "updated"

	maxAttempts = 10

	maxOperationTime = time.Minute
)

type StoreImpl struct {
	client  *ifirestore.Client
	crsName string
}

func New(client *ifirestore.Client, crsName string) *StoreImpl {
	return &StoreImpl{
		client:  client,
		crsName: crsName,
	}
}

type changeListEntry struct {
	SystemID string               `firestore:"systemid"`
	System   string               `firestore:"system"`
	Owner    string               `firestore:"owner"`
	Status   code_review.CLStatus `firestore:"status"`
	Subject  string               `firestore:"subject"`
	Updated  time.Time            `firestore:"updated"`
}

type patchSetEntry struct {
	SystemID     string `firestore:"systemid"`
	System       string `firestore:"system"`
	ChangeListID string `firestore:"changelistid"`
	Order        int    `firestore:"order"`
	GitHash      string `firestore:"githash"`
}

// GetChangeList implements the clstore.Store interface.
func (s *StoreImpl) GetChangeList(ctx context.Context, id string) (code_review.ChangeList, error) {
	defer metrics2.FuncTimer().Stop()
	q := s.client.Collection(changelistCollection).Where(systemIDField, "==", id).Where(systemField, "==", s.crsName)

	var cl code_review.ChangeList
	found := false
	err := s.client.IterDocs("get_cl", id, q, 3, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		cle := changeListEntry{}
		if err := doc.DataTo(&cle); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal %s changelist with id %s", s.crsName, id)
		}
		found = true
		cl.SystemID = cle.SystemID
		cl.Owner = cle.Owner
		cl.Status = cle.Status
		cl.Subject = cle.Subject
		cl.Updated = cle.Updated
		return nil
	})

	if err != nil {
		return cl, skerr.Wrapf(err, "could not execute GetChangeList query for %s", id)
	}
	if !found {
		return cl, code_review.ErrNotFound
	}

	return cl, nil
}

// GetPatchSet implements the clstore.Store interface.
func (s *StoreImpl) GetPatchSet(ctx context.Context, clId, psID string) (code_review.PatchSet, error) {
	return code_review.PatchSet{}, errors.New("not impl")
}

// PutChangeList implements the clstore.Store interface.
func (s *StoreImpl) PutChangeList(ctx context.Context, cl code_review.ChangeList) error {
	ir := s.client.Collection(changelistCollection).NewDoc()
	record := changeListEntry{
		SystemID: cl.SystemID,
		System:   s.crsName,
		Owner:    cl.Owner,
		Status:   cl.Status,
		Subject:  cl.Subject,
		Updated:  cl.Updated,
	}
	_, err := s.client.Set(ir, record, maxAttempts, maxOperationTime)
	if err != nil {
		return skerr.Wrapf(err, "could not write CL %v to clstore", cl)
	}
	return nil
}

// PutChangeList implements the clstore.Store interface.
func (s *StoreImpl) PutPatchSet(ctx context.Context, clID string, ps code_review.PatchSet) error {
	return errors.New("not impl")
}

// Make sure StoreImpl fulfills the clstore.Store interface.
var _ clstore.Store = (*StoreImpl)(nil)
