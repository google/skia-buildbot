package failurestore

import (
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

type FailureStore interface {
	// UnavailableDigests returns the current list of unavailable digests for fast lookup.
	UnavailableDigests() map[types.Digest]*diff.DigestFailure

	// AddDigestFailureIfNew adds a digest failure to the database only if the
	// there is no failure recorded for the given digest.
	AddDigestFailureIfNew(failure *diff.DigestFailure) error

	// AddDigestFailure adds a digest failure to the database or updates an
	// existing failure.
	AddDigestFailure(failure *diff.DigestFailure) error

	// PurgeDigestFailures removes the failures identified by digests from the database.
	PurgeDigestFailures(digests types.DigestSlice) error
}
