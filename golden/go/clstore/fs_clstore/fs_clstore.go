// Package fs_clstore implements the clstore.Store interface with
// a FireStore backend.
package fs_clstore

import (
	"context"
	"errors"

	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
)

type StoreImpl struct {
	fsClient *firestore.Client
	crsName  string
}

func New(client *firestore.Client, crsName string) *StoreImpl {
	return &StoreImpl{
		fsClient: client,
		crsName:  crsName,
	}
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
