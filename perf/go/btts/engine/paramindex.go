package engine

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/perf/go/btts"
)

// ParamIndex returns a channel.
func ParamIndex(ctx context.Context, table *bigtable.Table, tileKey btts.TileKey, key, value string) <-chan string {
	ret := make(chan string)

	go func() error {
		defer close(ret)
		prefix := fmt.Sprintf("%s:%s:%s", tileKey.IndexRowPrefix(), key, value)
		return table.ReadRows(ctx, bigtable.PrefixRange(prefix), func(row bigtable.Row) bool {
			parts := strings.Split(row.Key(), ":")
			if len(parts) != 4 {
				// Report error.
			}
			ret <- parts[3]
			return true
		}, bigtable.RowFilter(
			bigtable.ChainFilters(
				bigtable.LatestNFilter(1),
				bigtable.FamilyFilter(btts.INDEX_FAMILY),
			)))
	}()

	return ret
}
