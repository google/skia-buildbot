// Package fs_clstore implements the clstore.Store interface with
// a FireStore backend.
package fs_clstore

import (
	"context"
	"errors"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
)

const (
	// These are the collections in Firestore.
	changelistCollection = "clstore_changelist"
	patchsetCollection   = "clstore_patchset"

	// These are the fields we query by
	systemIDField = "systemid"
	clIDField     = "changelistid"

	maxAttempts = 10

	maxDuration = time.Minute
)

type StoreImpl struct {
	fsClient *ifirestore.Client
	crsName  string
}

func New(client *ifirestore.Client, crsName string) *StoreImpl {
	return &StoreImpl{
		fsClient: client,
		crsName:  crsName,
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
	ChangeListID string `firestore:"changelistid"`
	Order        int    `firestore:"order"`
	GitHash      string `firestore:"githash"`
}

// GetChangeList implements the clstore.Store interface.
func (s *StoreImpl) GetChangeList(ctx context.Context, id string) (code_review.ChangeList, error) {
	return code_review.ChangeList{}, errors.New("not impl")
}

// GetPatchSet implements the clstore.Store interface.
func (s *StoreImpl) GetPatchSet(ctx context.Context, clId, psID string) (code_review.PatchSet, error) {
	return code_review.PatchSet{}, errors.New("not impl")
}

// PutChangeList implements the clstore.Store interface.
func (s *StoreImpl) PutChangeList(ctx context.Context, cl code_review.ChangeList) error {
	return errors.New("not impl")
}

// PutChangeList implements the clstore.Store interface.
func (s *StoreImpl) PutPatchSet(ctx context.Context, clID string, ps code_review.PatchSet) error {
	return errors.New("not impl")
}

// Make sure StoreImpl fulfills the clstore.Store interface.
var _ clstore.Store = (*StoreImpl)(nil)
