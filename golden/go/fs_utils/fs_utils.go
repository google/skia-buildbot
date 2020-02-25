package fs_utils

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"cloud.google.com/go/firestore"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	// recoverTime is the minimum amount of time to wait before recreating any QuerySnapshotIterator
	// if it fails. A random amount of time should be added to this, proportional to recoverTime.
	recoverTime = 30 * time.Second
)

// ShardQueryOnDigest splits a query up to work on a subset of the data based on
// the digests. We split the MD5 space up into N shards by making N-1 shard points
// and adding Where clauses to make N queries that are between those points.
func ShardQueryOnDigest(baseQuery firestore.Query, digestField string, shards int) []firestore.Query {
	queries := make([]firestore.Query, 0, shards)
	zeros := strings.Repeat("0", 16)
	s := uint64(0)
	for i := 0; i < shards-1; i++ {
		// An MD5 hash is 128 bits, which we encode to hexadecimal (32 chars).
		// We can produce an MD5 hash by taking a 64 bit unsigned int, turning
		// that to hexadecimal (16 chars), then appending 16 zeros.
		startHash := fmt.Sprintf("%016x%s", s, zeros)

		s += math.MaxUint64/uint64(shards) + 1
		endHash := fmt.Sprintf("%016x%s", s, zeros)

		// The first n queries are formulated to be between two shard points
		queries = append(queries, baseQuery.Where(digestField, ">=", startHash).Where(digestField, "<", endHash))
	}
	lastHash := fmt.Sprintf("%016x%s", s, zeros)
	// The last query is just a greater than the last shard point
	queries = append(queries, baseQuery.Where(digestField, ">=", lastHash))
	return queries
}

// ListenAndRecover will listen to the QuerySnapshotIterator provided by snapFactory and execute
// callback with the result. If getting the next snapshot fails (e.g. temporary Firestore error,
// out of quota errors, etc), it sleeps for a randomized amount of time and then creates a new
// QuerySnapshotIterator to listen to. This is due to the fact that once a SnapshotQueryIterator
// returns an error, it seems to always return that error. If the passed in context returns an
// error (e.g. it was cancelled), this function will return with no error.
func ListenAndRecover(ctx context.Context, initialSnap *firestore.QuerySnapshotIterator, snapFactory func() *firestore.QuerySnapshotIterator, callback func(context.Context, *firestore.QuerySnapshot) error) error {
	snap := initialSnap
	if snap == nil {
		snap = snapFactory()
	}
	for {
		if err := ctx.Err(); err != nil {
			sklog.Debugf("Stopping query of ignores due to context error: %s", err)
			snap.Stop()
			return nil
		}
		qs, err := snap.Next()
		if err != nil {
			sklog.Errorf("reading query snapshot: %s", err)
			snap.Stop()
			if err := ctx.Err(); err != nil {
				// Oh, it was from a context cancellation (e.g. a test), don't recover.
				return nil
			}
			// sleep and rebuild the snapshot query.
			t := recoverTime + time.Duration(float32(recoverTime)*rand.Float32())
			sklog.Infof("Sleeping for %s and then will recreate query snapshot", t)
			time.Sleep(t)
			sklog.Infof("Trying to recreate query snapshot after having slept %s", t)
			snap = snapFactory()
			continue
		}
		if err := callback(ctx, qs); err != nil {
			return skerr.Wrap(err)
		}
	}
}
