package btts

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/btts/engine"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ParamIndex returns a channel that emits the OPS encoded trace keys in
// alphabetical order for all traces that match the given key=value in their
// params for the tile specified by tileKey.
//
// The key and value are OPS encoded.
//
// BigTable errors are sent back over the errCh channel, which is not closed
// when the function finishes.
//
// The returned channel is closed once the last trace id is sent, or the context
// is cancelled.
//
// The desc is a descriptive string to add to any error logs this function produces.
func ParamIndex(ctx context.Context, table *bigtable.Table, tileKey TileKey, key, value, desc string) <-chan string {
	ch := make(chan string, engine.QUERY_ENGINE_CHANNEL_SIZE)

	go func(ch chan string) {
		defer close(ch)
		prefix := fmt.Sprintf("%s:%s:%s:", tileKey.IndexRowPrefix(), key, value)
		if err := table.ReadRows(ctx, bigtable.PrefixRange(prefix), func(row bigtable.Row) bool {
			parts := strings.Split(row.Key(), ":")
			if len(parts) != 4 {
				sklog.Errorf("Invalid index row key doesn't contain 4 parts: %q", row.Key())
			}
			b := &strings.Builder{}
			b.WriteString(parts[3])
			ch <- b.String()
			return true
		}, bigtable.RowFilter(
			bigtable.ChainFilters(
				bigtable.LatestNFilter(1),
				bigtable.FamilyFilter(INDEX_FAMILY),
			))); err != nil {
			// It is completely possible that this request gets cancelled before
			// it is finished if it feeds into Intersect and the other query
			// runs out of keys first. But we want to report an error into the
			// logs if it's not that kind of error. So we convert it to a
			// grpc.Status, and if our context cancelled the operation then the
			// status code will be 'unknown'. Not sure why it isn't 'timeout',
			// but I think that's because the timeout occurred outside the grpc
			// error space. See:
			// https://godoc.org/google.golang.org/grpc/codes#Code
			st := status.FromContextError(err)
			if st != nil && st.Code() != codes.Unknown {
				sklog.Errorf("ReadRows failed: %q, %v", desc, st)
			}
		}
	}(ch)

	return ch
}
