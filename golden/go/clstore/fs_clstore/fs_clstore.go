// Package fs_clstore implements the clstore.Store interface with
// a FireStore backend.
package fs_clstore

import (
	"context"
	"time"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// These are the collections in Firestore.
	changelistCollection = "clstore_changelist"
	patchsetCollection   = "clstore_patchset"

	// These are the fields we query by
	systemIDField = "systemid"
	systemField   = "system"
	clIDField     = "changelistid"

	maxAttempts = 10

	maxOperationTime = time.Minute
)

// StoreImpl is the firestore based implementation of clstore.
type StoreImpl struct {
	client  *ifirestore.Client
	crsName string
}

// New returns a new StoreImpl
func New(client *ifirestore.Client, crsName string) *StoreImpl {
	return &StoreImpl{
		client:  client,
		crsName: crsName,
	}
}

// changeListEntry represents how a ChangeList is stored in FireStore.
type changeListEntry struct {
	SystemID string               `firestore:"systemid"`
	System   string               `firestore:"system"`
	Owner    string               `firestore:"owner"`
	Status   code_review.CLStatus `firestore:"status"`
	Subject  string               `firestore:"subject"`
	Updated  time.Time            `firestore:"updated"`
}

// patchSetEntry represents how a PatchSet is stored in FireStore.
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
	id = s.changeListFirestoreID(id)
	doc, err := s.client.Collection(changelistCollection).Doc(id).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return code_review.ChangeList{}, clstore.ErrNotFound
		}
		return code_review.ChangeList{}, skerr.Wrapf(err, "retrieving CL %s from firestore", doc.Ref.ID)
	}
	if doc == nil {
		return code_review.ChangeList{}, clstore.ErrNotFound
	}

	cle := changeListEntry{}
	if err := doc.DataTo(&cle); err != nil {
		id := doc.Ref.ID
		return code_review.ChangeList{}, skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal %s changelist with id %s", s.crsName, id)
	}
	cl := code_review.ChangeList{
		SystemID: cle.SystemID,
		Owner:    cle.Owner,
		Status:   cle.Status,
		Subject:  cle.Subject,
		Updated:  cle.Updated,
	}

	return cl, nil
}

func (s *StoreImpl) changeListFirestoreID(clID string) string {
	return clID + "_" + s.crsName
}

// GetPatchSet implements the clstore.Store interface.
func (s *StoreImpl) GetPatchSet(ctx context.Context, clID, psID string) (code_review.PatchSet, error) {
	defer metrics2.FuncTimer().Stop()
	psID = s.patchSetFirestoreID(psID, clID)
	doc, err := s.client.Collection(patchsetCollection).Doc(psID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return code_review.PatchSet{}, clstore.ErrNotFound
		}
		return code_review.PatchSet{}, skerr.Wrapf(err, "retrieving PS %s from firestore", doc.Ref.ID)
	}
	if doc == nil {
		return code_review.PatchSet{}, clstore.ErrNotFound
	}

	pse := patchSetEntry{}
	if err := doc.DataTo(&pse); err != nil {
		id := doc.Ref.ID
		return code_review.PatchSet{}, skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal %s patchset with id %s", s.crsName, id)
	}
	ps := code_review.PatchSet{
		SystemID:     pse.SystemID,
		ChangeListID: pse.ChangeListID,
		Order:        pse.Order,
		GitHash:      pse.GitHash,
	}

	return ps, nil
}

func (s *StoreImpl) patchSetFirestoreID(psID, clID string) string {
	return psID + "_" + s.crsName + "_" + clID
}

// PutChangeList implements the clstore.Store interface.
func (s *StoreImpl) PutChangeList(ctx context.Context, cl code_review.ChangeList) error {
	defer metrics2.FuncTimer().Stop()
	cd := s.client.Collection(changelistCollection).Doc(s.changeListFirestoreID(cl.SystemID))
	record := changeListEntry{
		SystemID: cl.SystemID,
		System:   s.crsName,
		Owner:    cl.Owner,
		Status:   cl.Status,
		Subject:  cl.Subject,
		Updated:  cl.Updated,
	}
	_, err := s.client.Set(cd, record, maxAttempts, maxOperationTime)
	if err != nil {
		return skerr.Wrapf(err, "could not write CL %v to clstore", cl)
	}
	return nil
}

// PutPatchSet implements the clstore.Store interface.
func (s *StoreImpl) PutPatchSet(ctx context.Context, ps code_review.PatchSet) error {
	defer metrics2.FuncTimer().Stop()
	psID := s.patchSetFirestoreID(ps.SystemID, ps.ChangeListID)
	pd := s.client.Collection(patchsetCollection).Doc(psID)
	record := patchSetEntry{
		SystemID:     ps.SystemID,
		System:       s.crsName,
		ChangeListID: ps.ChangeListID,
		Order:        ps.Order,
		GitHash:      ps.GitHash,
	}
	_, err := s.client.Set(pd, record, maxAttempts, maxOperationTime)
	if err != nil {
		return skerr.Wrapf(err, "could not write PS %v to clstore", ps)
	}
	return nil
}

// Make sure StoreImpl fulfills the clstore.Store interface.
var _ clstore.Store = (*StoreImpl)(nil)
