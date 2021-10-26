// Package clstore defines an interface for storing Changelist-related data
// as needed for operating Gold.
package clstore

import (
	"errors"
	"math"
	"time"

	"go.skia.org/infra/golden/go/code_review"
)

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
	URLTemplate string
}
