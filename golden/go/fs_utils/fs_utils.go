package fs_utils

import (
	"fmt"
	"math"
	"strings"

	"cloud.google.com/go/firestore"
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
