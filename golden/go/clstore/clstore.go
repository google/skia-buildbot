// Package clstore defines an interface for storing Changelist-related data
// as needed for operating Gold.
package clstore

import (
	"context"
	"errors"
	"math"
	"time"

	"go.skia.org/infra/golden/go/code_review"
)

// Store (sometimes called ChangelistStore) is an interface around a database
// for storing Changelists and Patchsets. Of note, we will only store data for
// Changelists and Patchsets that have uploaded data to Gold (e.g. via ingestion);
// the purpose of this interface is not to store every CL.
// A single Store interface should only be responsible for one "system", i.e.
// Gerrit or GitHub.
// TODO(kjlubick) Just like the tryjobstore holds onto all tryjob results (from all CIS), this
//   should hold onto all CLs from all CRS.
type Store interface {
	// GetChangelist returns the Changelist corresponding to the given id.
	// Returns NotFound if it doesn't exist.
	GetChangelist(ctx context.Context, id string) (code_review.Changelist, error)
	// GetPatchset returns the Patchset matching the given Changelist ID and Patchset ID.
	// Returns NotFound if it doesn't exist.
	GetPatchset(ctx context.Context, clID, psID string) (code_review.Patchset, error)
	// GetPatchsetByOrder returns the Patchset matching the given Changelist ID and order.
	// Returns NotFound if it doesn't exist.
	GetPatchsetByOrder(ctx context.Context, clID string, psOrder int) (code_review.Patchset, error)

	// GetChangelists returns a slice of Changelist objects sorted such that the
	// most recently updated ones come first. The SearchOptions should be supplied to narrow
	// the query down and limit results. Limit is required.
	// If it is computationally cheap to do so, the second return value can be
	// a count of the total number of CLs, or CountMany otherwise.
	GetChangelists(ctx context.Context, opts SearchOptions) ([]code_review.Changelist, int, error)

	// GetPatchsets returns a slice of Patchsets belonging to the given Changelist.
	// They should be ordered in increasing Order index.
	// The returned slice could be empty, even if the CL exists.
	GetPatchsets(ctx context.Context, clID string) ([]code_review.Patchset, error)

	// PutChangelist stores the given Changelist, overwriting any values for
	// that Changelist if they already existed.
	PutChangelist(ctx context.Context, cl code_review.Changelist) error
	// PutPatchset stores the given Patchset, overwriting any values for
	// that Patchset if they already existed.
	PutPatchset(ctx context.Context, ps code_review.Patchset) error
}

var ErrNotFound = errors.New("not found")

// SearchOptions controls which Changelists to return.
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
