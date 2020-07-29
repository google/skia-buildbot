// Package clstore defines an interface for storing ChangeList-related data
// as needed for operating Gold.
package clstore

import (
	"context"
	"errors"
	"math"
	"time"

	"go.skia.org/infra/golden/go/code_review"
)

// Store (sometimes called ChangeListStore) is an interface around a database
// for storing ChangeLists and PatchSets. Of note, we will only store data for
// ChangeLists and PatchSets that have uploaded data to Gold (e.g. via ingestion);
// the purpose of this interface is not to store every CL.
// A single Store interface should only be responsible for one "system", i.e.
// Gerrit or GitHub.
// TODO(kjlubick) Just like the tryjobstore holds onto all tryjob results (from all CIS), this
//   should hold onto all CLs from all CRS.
type Store interface {
	// GetChangeList returns the ChangeList corresponding to the given id.
	// Returns NotFound if it doesn't exist.
	GetChangeList(ctx context.Context, id string) (code_review.ChangeList, error)
	// GetPatchSet returns the PatchSet matching the given ChangeList ID and PatchSet ID.
	// Returns NotFound if it doesn't exist.
	GetPatchSet(ctx context.Context, clID, psID string) (code_review.PatchSet, error)
	// GetPatchSetByOrder returns the PatchSet matching the given ChangeList ID and order.
	// Returns NotFound if it doesn't exist.
	GetPatchSetByOrder(ctx context.Context, clID string, psOrder int) (code_review.PatchSet, error)

	// GetChangeLists returns a slice of ChangeList objects sorted such that the
	// most recently updated ones come first. The SearchOptions should be supplied to narrow
	// the query down and limit results. Limit is required.
	// If it is computationally cheap to do so, the second return value can be
	// a count of the total number of CLs, or CountMany otherwise.
	GetChangeLists(ctx context.Context, opts SearchOptions) ([]code_review.ChangeList, int, error)

	// GetPatchSets returns a slice of PatchSets belonging to the given ChangeList.
	// They should be ordered in increasing Order index.
	// The returned slice could be empty, even if the CL exists.
	GetPatchSets(ctx context.Context, clID string) ([]code_review.PatchSet, error)

	// PutChangeList stores the given ChangeList, overwriting any values for
	// that ChangeList if they already existed.
	PutChangeList(ctx context.Context, cl code_review.ChangeList) error
	// PutPatchSet stores the given PatchSet, overwriting any values for
	// that PatchSet if they already existed.
	PutPatchSet(ctx context.Context, ps code_review.PatchSet) error
}

var ErrNotFound = errors.New("not found")

// SearchOptions controls which ChangeLists to return.
type SearchOptions struct {
	StartIdx    int
	Limit       int
	OpenCLsOnly bool
	After       time.Time
}

// CountMany indicates it is computationally expensive to determine exactly how many
// items there are.
var CountMany = math.MaxInt32

// ReviewSystem combines the data needed to interface with a single CRS.
type ReviewSystem struct {
	ID          string // e.g. "gerrit", "gerrit-internal"
	Client      code_review.Client
	Store       Store
	URLTemplate string
}
