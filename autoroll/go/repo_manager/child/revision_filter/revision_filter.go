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
	Skip(context.Context, revision.Revision) (string, error)
	// Update any cached data stored by the RevisionFilter. This should be done
	// once before each batch of calls to Skip().
	Update(context.Context) error
}

type RevisionFilters []RevisionFilter

// Update all of the RevisionFilters.
func (rfs RevisionFilters) Update(ctx context.Context) error {
	for _, rf := range rfs {
		if err := rf.Update(ctx); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// Skip runs rf.Skip(rev) for each RevisionFilter.
func (rfs RevisionFilters) Skip(ctx context.Context, rev revision.Revision) (string, error) {
	for _, rf := range rfs {
		invalidReason, err := rf.Skip(ctx, rev)
		if err != nil {
			return "", skerr.Wrap(err)
		}
		if invalidReason != "" {
			return invalidReason, nil
		}
	}
	return "", nil
}

// MaybeSetInvalid uses the each RevisionFilter to determine whether the given
// Revision is invalid and should be skipped. If the Revision is invalid, the
// InvalidReason field is set to the message returned by RevisionFilter.Skip.
func (rfs RevisionFilters) MaybeSetInvalid(ctx context.Context, rev *revision.Revision) error {
	invalidReason, err := rfs.Skip(ctx, *rev)
	if err != nil {
		return skerr.Wrap(err)
	}
	if invalidReason != "" {
		rev.InvalidReason = invalidReason
	}
	return nil
}
