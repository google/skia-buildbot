package revision_filter

import (
	"context"

	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
)

// RevisionFilter determines whether Revisions should be skipped when
// considering what Revision to roll.
type RevisionFilter interface {
	// Skip returns a non-empty string if the revision should be skipped. The
	// string will contain the reason the revision should be skipped. An empty
	// string is returned if the revision should not be skipped.
	// If an error is returned then an empty string will be returned.
	Skip(context.Context, *revision.Revision) (string, error)
}

// MaybeSetInvalid uses the given RevisionFilter to determine whether the given
// Revision is invalid and should be skipped. If the Revision is invalid, the
// InvalidReason field is set to the message returned by RevisionFilter.Skip.
func MaybeSetInvalid(ctx context.Context, rf RevisionFilter, rev *revision.Revision) error {
	invalidReason, err := rf.Skip(ctx, rev)
	if err != nil {
		return skerr.Wrap(err)
	}
	if invalidReason != "" {
		rev.InvalidReason = invalidReason
	}
	return nil
}
