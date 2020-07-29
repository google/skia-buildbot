// Package fs_clstore implements the clstore.Store interface with
// a FireStore backend.
package fs_clstore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
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
	orderField   = "order"
	statusField  = "status"
	updatedField = "updated"

	maxReadAttempts  = 5
	maxWriteAttempts = 5
	maxOperationTime = time.Minute
)

// StoreImpl is the Firestore based implementation of clstore.
type StoreImpl struct {
	client *ifirestore.Client
	crsID  string
}

// New returns a new StoreImpl. The crsID should be distinct enough to separate an internal CRS
// from an external one, if needed (e.g. "gerrit" vs "gerrit-internal")
func New(client *ifirestore.Client, crsID string) *StoreImpl {
	return &StoreImpl{
		client: client,
		crsID:  crsID,
	}
}

// changeListEntry represents how a ChangeList is stored in Firestore.
type changeListEntry struct {
	SystemID string               `firestore:"systemid"`
	System   string               `firestore:"system"`
	Owner    string               `firestore:"owner"`
	Status   code_review.CLStatus `firestore:"status"`
	Subject  string               `firestore:"subject"`
	Updated  time.Time            `firestore:"updated"`
}

// patchSetEntry represents how a PatchSet is stored in Firestore.
type patchSetEntry struct {
	SystemID                      string    `firestore:"systemid"`
	System                        string    `firestore:"system"`
	ChangeListID                  string    `firestore:"changelistid"`
	Order                         int       `firestore:"order"`
	GitHash                       string    `firestore:"githash"`
	CommentedOnCL                 bool      `firestore:"did_comment"`
	LastCheckedIfCommentNecessary time.Time `firestore:"last_checked_about_comment"`
}

// toPatchSet converts the Firestore representation of a PatchSet to a code_review.PatchSet
func (p patchSetEntry) toPatchSet() code_review.PatchSet {
	return code_review.PatchSet{
		SystemID:                      p.SystemID,
		ChangeListID:                  p.ChangeListID,
		Order:                         p.Order,
		GitHash:                       p.GitHash,
		LastCheckedIfCommentNecessary: p.LastCheckedIfCommentNecessary,
		CommentedOnCL:                 p.CommentedOnCL,
	}
}

// GetChangeList implements the clstore.Store interface.
func (s *StoreImpl) GetChangeList(ctx context.Context, id string) (code_review.ChangeList, error) {
	defer metrics2.FuncTimer().Stop()
	fID := s.changeListFirestoreID(id)
	doc, err := s.client.Collection(changelistCollection).Doc(fID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return code_review.ChangeList{}, clstore.ErrNotFound
		}
		return code_review.ChangeList{}, skerr.Wrapf(err, "retrieving CL %s from Firestore", fID)
	}
	if doc == nil {
		return code_review.ChangeList{}, clstore.ErrNotFound
	}

	cle := changeListEntry{}
	if err := doc.DataTo(&cle); err != nil {
		id := doc.Ref.ID
		return code_review.ChangeList{}, skerr.Wrapf(err, "corrupt data in Firestore, could not unmarshal %s changelist with id %s", s.crsID, id)
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

// changeListFirestoreID returns the id for a given CL in a given CRS - this allows us to
// look up a document by id w/o having to perform a query.
func (s *StoreImpl) changeListFirestoreID(clID string) string {
	return clID + "_" + s.crsID
}

// GetChangeLists implements the clstore.Store interface.
func (s *StoreImpl) GetChangeLists(ctx context.Context, opts clstore.SearchOptions) ([]code_review.ChangeList, int, error) {
	defer metrics2.FuncTimer().Stop()
	if opts.Limit <= 0 {
		return nil, 0, skerr.Fmt("must supply a limit")
	}
	q := s.client.Collection(changelistCollection).OrderBy(updatedField, firestore.Desc)
	if !opts.After.IsZero() {
		q = q.Where(updatedField, ">=", opts.After)
	}
	if opts.OpenCLsOnly {
		q = q.Where(statusField, "==", code_review.Open)
	}
	q = q.Limit(opts.Limit).Offset(opts.StartIdx)

	var xcl []code_review.ChangeList

	r := fmt.Sprintf("[%d:%d]", opts.StartIdx, opts.StartIdx+opts.Limit)
	err := s.client.IterDocs(ctx, "GetChangeLists", r, q, maxReadAttempts, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		entry := changeListEntry{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in Firestore, could not unmarshal entry with id %s", id)
		}
		xcl = append(xcl, code_review.ChangeList{
			SystemID: entry.SystemID,
			Updated:  entry.Updated,
			Subject:  entry.Subject,
			Status:   entry.Status,
			Owner:    entry.Owner,
		})
		return nil
	})
	if err != nil {
		return nil, -1, skerr.Wrapf(err, "fetching cls in range %s", r)
	}
	n := len(xcl)
	if n == opts.Limit && n != 0 {
		// We don't know how many there are and it might be too slow to count, so just give
		// the "many" response.
		n = clstore.CountMany
	} else {
		// We know exactly either 1) how many there are (if n > 0) or 2) an upper bound on how many
		// there are (if n == 0)
		n += opts.StartIdx
	}

	return xcl, n, nil
}

// GetPatchSet implements the clstore.Store interface.
func (s *StoreImpl) GetPatchSet(ctx context.Context, clID, psID string) (code_review.PatchSet, error) {
	defer metrics2.FuncTimer().Stop()
	fID := s.changeListFirestoreID(clID)
	doc, err := s.client.Collection(changelistCollection).Doc(fID).
		Collection(patchsetCollection).Doc(psID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return code_review.PatchSet{}, clstore.ErrNotFound
		}
		return code_review.PatchSet{}, skerr.Wrapf(err, "retrieving PS %s from Firestore", fID)
	}
	if doc == nil {
		return code_review.PatchSet{}, clstore.ErrNotFound
	}

	pse := patchSetEntry{}
	if err := doc.DataTo(&pse); err != nil {
		id := doc.Ref.ID
		return code_review.PatchSet{}, skerr.Wrapf(err, "corrupt data in Firestore, could not unmarshal %s patchset with id %s", s.crsID, id)
	}
	return pse.toPatchSet(), nil
}

// GetPatchSetByOrder implements the clstore.Store interface.
func (s *StoreImpl) GetPatchSetByOrder(ctx context.Context, clID string, psOrder int) (code_review.PatchSet, error) {
	defer metrics2.FuncTimer().Stop()
	fID := s.changeListFirestoreID(clID)
	q := s.client.Collection(changelistCollection).Doc(fID).
		Collection(patchsetCollection).Where(orderField, "==", psOrder)

	ps := code_review.PatchSet{}
	found := false
	msg := fmt.Sprintf("%s:%d", clID, psOrder)
	err := s.client.IterDocs(ctx, "GetPatchSetByOrder", msg, q, maxReadAttempts, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil || found {
			return nil
		}
		entry := patchSetEntry{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in Firestore, could not unmarshal patchsetEntry with id %s", id)
		}
		ps = entry.toPatchSet()
		found = true
		return nil
	})
	if err != nil {
		return code_review.PatchSet{}, skerr.Wrapf(err, "fetching patchsets for cl %s", clID)
	}
	if !found {
		return code_review.PatchSet{}, clstore.ErrNotFound
	}

	return ps, nil
}

// GetPatchSets implements the clstore.Store interface.
func (s *StoreImpl) GetPatchSets(ctx context.Context, clID string) ([]code_review.PatchSet, error) {
	defer metrics2.FuncTimer().Stop()
	fID := s.changeListFirestoreID(clID)
	q := s.client.Collection(changelistCollection).Doc(fID).
		Collection(patchsetCollection).OrderBy(orderField, firestore.Asc)

	var xps []code_review.PatchSet

	err := s.client.IterDocs(ctx, "GetPatchSets", clID, q, maxReadAttempts, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		entry := patchSetEntry{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in Firestore, could not unmarshal entry with id %s", id)
		}
		xps = append(xps, entry.toPatchSet())
		return nil
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching patchsets for cl %s", clID)
	}

	return xps, nil
}

// PutChangeList implements the clstore.Store interface.
func (s *StoreImpl) PutChangeList(ctx context.Context, cl code_review.ChangeList) error {
	defer metrics2.FuncTimer().Stop()
	fID := s.changeListFirestoreID(cl.SystemID)
	cd := s.client.Collection(changelistCollection).Doc(fID)
	record := changeListEntry{
		SystemID: cl.SystemID,
		System:   s.crsID,
		Owner:    cl.Owner,
		Status:   cl.Status,
		Subject:  cl.Subject,
		Updated:  cl.Updated,
	}
	_, err := s.client.Set(ctx, cd, record, maxWriteAttempts, maxOperationTime)
	if err != nil {
		return skerr.Wrapf(err, "could not write CL %v to clstore", cl)
	}
	return nil
}

// PutPatchSet implements the clstore.Store interface.
func (s *StoreImpl) PutPatchSet(ctx context.Context, ps code_review.PatchSet) error {
	defer metrics2.FuncTimer().Stop()
	fID := s.changeListFirestoreID(ps.ChangeListID)
	pd := s.client.Collection(changelistCollection).Doc(fID).
		Collection(patchsetCollection).Doc(ps.SystemID)
	record := patchSetEntry{
		SystemID:                      ps.SystemID,
		System:                        s.crsID,
		ChangeListID:                  ps.ChangeListID,
		Order:                         ps.Order,
		GitHash:                       ps.GitHash,
		LastCheckedIfCommentNecessary: ps.LastCheckedIfCommentNecessary,
		CommentedOnCL:                 ps.CommentedOnCL,
	}
	_, err := s.client.Set(ctx, pd, record, maxWriteAttempts, maxOperationTime)
	if err != nil {
		return skerr.Wrapf(err, "could not write PS %v to clstore", ps)
	}
	return nil
}

// Make sure StoreImpl fulfills the clstore.Store interface.
var _ clstore.Store = (*StoreImpl)(nil)
