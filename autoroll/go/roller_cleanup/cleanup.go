package roller_cleanup

import (
	"context"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// DB provides a mechanism for users (or the roller itself) to
// request cleanup (ie. deleting local data) of a roller.
type DB interface {
	// RequestCleanup requests or cancels an existing request for cleanup.
	RequestCleanup(ctx context.Context, req *CleanupRequest) error

	// History returns the requests for cleanup for a given roller, sorted most-
	// recent first. If greater than zero, `limit` controls the number of
	// results.
	History(ctx context.Context, rollerID string, limit int) ([]*CleanupRequest, error)
}

// CleanupRequest describes a request to clean up a roller.
type CleanupRequest struct {
	RollerID      string
	NeedsCleanup  bool
	User          string
	Timestamp     time.Time
	Justification string
}

// Validate returns an error if the CleanupRequest is invalid.
func (req *CleanupRequest) Validate() error {
	if req.RollerID == "" {
		return skerr.Fmt("RollerID is required")
	}
	if req.User == "" {
		return skerr.Fmt("User is required")
	}
	if util.TimeIsZero(req.Timestamp) {
		return skerr.Fmt("Timestamp is required")
	}
	if req.Justification == "" {
		return skerr.Fmt("Justification is required")
	}
	return nil
}

// NeedsCleanup returns true if the given roller needs cleanup.
func NeedsCleanup(ctx context.Context, db DB, rollerID string) (bool, error) {
	history, err := db.History(ctx, rollerID, 1)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	if len(history) == 0 {
		return false, nil
	}
	return history[0].NeedsCleanup, nil
}
