// Package clstore defines an interface for storing ChangeList-related data
// as needed for operating Gold.
package clstore

import (
	"context"
	"errors"

	"go.skia.org/infra/golden/go/code_review"
)

// Store (sometimes called ChangeListStore) is an interface around a database
// for storing ChangeLists and PatchSets. Of note, we will only store data for
// ChangeLists and PatchSets that have uploaded data to Gold (e.g. via ingestion);
// the purpose of this interface is not to store every CL.
type Store interface {
	// GetChangeList returns the ChangeList corresponding to the given id.
	// Returns NotFound if it doesn't exist.
	GetChangeList(ctx context.Context, id string) (code_review.ChangeList, error)
	// GetPatchSet returns the PatchSet matching the given ChangeList ID and PatchSet ID.
	// Returns NotFound if it doesn't exist.
	GetPatchSet(ctx context.Context, clId, psID string) (code_review.PatchSet, error)

	// PutChangeList stores the given ChangeList, overwriting any values for
	// that ChangeList if they already existed.
	PutChangeList(ctx context.Context, cl code_review.ChangeList) error
	// PutChangeList stores the given PatchSet (which belongs to the ChangeList of the given id),
	// overwriting any values for that ChangeList if they already existed.
	PutPatchSet(ctx context.Context, clID string, ps code_review.PatchSet) error
}

var NotFound = errors.New("not found")
