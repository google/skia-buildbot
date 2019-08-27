package btts

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/sklog"
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
func ParamIndex(ctx context.Context, table *bigtable.Table, tileKey TileKey, key, value string, errCh chan<- error) <-chan string {
	ret := make(chan string)

	go func() {
		defer close(ret)
		prefix := fmt.Sprintf("%s:%s:%s", tileKey.IndexRowPrefix(), key, value)
		if err := table.ReadRows(ctx, bigtable.PrefixRange(prefix), func(row bigtable.Row) bool {
			parts := strings.Split(row.Key(), ":")
			if len(parts) != 4 {
				sklog.Errorf("Invalid index row key doesn't contain 4 parts: %q", row.Key())
			}
			ret <- parts[3]
			return true
		}, bigtable.RowFilter(
			bigtable.ChainFilters(
				bigtable.LatestNFilter(1),
				bigtable.FamilyFilter(INDEX_FAMILY),
			))); err != nil {
			errCh <- fmt.Errorf("BigTable failure at Tile: %d key=%q value=%q: %s", tileKey, key, value, err)
		}
	}()

	return ret
}
