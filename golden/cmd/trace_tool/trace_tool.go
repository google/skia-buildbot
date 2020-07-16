// The trace_tool executable is meant for directly interacting with the traces stored in
// BigTable. It was last used to clean up bad data in a series of tiles.
package main

import (
	"context"
	"flag"
	"fmt"
	"sync/atomic"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo/bt_vcs"
	"go.skia.org/infra/golden/go/tracestore/bt_tracestore"
	"golang.org/x/sync/errgroup"
)

const (
	instanceID = "production"  // The user is expected to edit these by hand or turn them into
	projectID  = "skia-public" // flags

	gitTableID   = "git-repos2"
	traceBTTable = "gold-skia"
	gitRepoURL   = "https://skia.googlesource.com/skia.git"
)

func main() {
	flag.Parse()
	ctx := context.Background()

	btConf := &bt_gitstore.BTConfig{
		InstanceID: instanceID,
		ProjectID:  projectID,
		TableID:    gitTableID,
		AppProfile: bt.TestingAppProfile,
	}

	gitStore, err := bt_gitstore.New(ctx, btConf, gitRepoURL)
	if err != nil {
		sklog.Fatalf("Error instantiating gitstore: %s", err)
	}

	gitilesRepo := gitiles.NewRepo("", nil)
	vcs, err := bt_vcs.New(ctx, gitStore, "master", gitilesRepo)
	if err != nil {
		sklog.Fatalf("Error creating BT-backed VCS instance: %s", err)
	}

	btc := bt_tracestore.BTConfig{
		InstanceID: instanceID,
		ProjectID:  projectID,
		TableID:    traceBTTable,
		VCS:        vcs,
	}

	offending, err := vcs.IndexOf(ctx, "a0f9a3e62afad0568380f982fe1a68f787682fa6")
	if err != nil {
		sklog.Fatalf("err", err)
	}
	sklog.Infof("offending commit index %d", offending)

	n := vcs.LastNIndex(1)
	sklog.Infof("Last commit was %s at %s index %d", n[0].Hash, n[0].Timestamp, n[0].Index)
	tk, offset := bt_tracestore.GetTileKey(n[0].Index)
	sklog.Infof("This is tile key %d offset %d", tk, offset)

	tk++
	sklog.Infof("Cleaning up the past to %d", tk)

	client, err := bigtable.NewClient(ctx, btc.ProjectID, btc.InstanceID)
	if err != nil {
		sklog.Fatalf("Error creating client for project %s and instance %s: %s", btc.ProjectID, btc.InstanceID, err)
	}

	adminClient, err := bigtable.NewAdminClient(ctx, btc.ProjectID, btc.InstanceID)
	if err != nil {
		sklog.Fatalf("Error creating admin client for project %s and instance %s: %s", btc.ProjectID, btc.InstanceID, err)
	}

	must := func(err error) {
		if err != nil {
			sklog.Fatalf(err.Error())
		}
	}

	must(countTraceRowsForTile(ctx, client, int(tk)))
	must(deleteTraceDataForTile(ctx, adminClient, int(tk)))
	must(countTraceRowsForTile(ctx, client, int(tk)))

	must(countTraceOptionsForTile(ctx, client, int(tk)))
	must(deleteTraceOptionsForTile(ctx, adminClient, int(tk)))
	must(countTraceOptionsForTile(ctx, client, int(tk)))

	must(deleteOPSForTile(ctx, adminClient, int(tk)))
}

func countTraceRowsForTile(ctx context.Context, bc *bigtable.Client, tk int) error {
	count := int32(0)
	table := bc.Open(traceBTTable)

	eg, ctx := errgroup.WithContext(ctx)
	for shard := 0; shard < bt_tracestore.DefaultShards; shard++ {
		shardedPrefix := fmt.Sprintf("%02d:ts:t:%d", shard, tk)
		eg.Go(func() error {
			prefixRange := bigtable.PrefixRange(shardedPrefix)
			return table.ReadRows(ctx, prefixRange, func(row bigtable.Row) bool {
				atomic.AddInt32(&count, 1)
				return true
			}, bigtable.RowFilter(
				bigtable.ChainFilters(
					bigtable.StripValueFilter(), // https://cloud.google.com/bigtable/docs/using-filters#strip-value
					bigtable.CellsPerRowLimitFilter(1),
					bigtable.LatestNFilter(1),
				),
			))
		})
	}

	err := eg.Wait()
	sklog.Infof("Counted %d trace rows for tile %d", count, tk)
	return err
}

func countTraceOptionsForTile(ctx context.Context, bc *bigtable.Client, tk int) error {
	count := int32(0)
	table := bc.Open(traceBTTable)

	eg, ctx := errgroup.WithContext(ctx)
	for shard := 0; shard < bt_tracestore.DefaultShards; shard++ {
		shardedPrefix := fmt.Sprintf("%02d:ts:p:%d", shard, tk)
		eg.Go(func() error {
			prefixRange := bigtable.PrefixRange(shardedPrefix)
			return table.ReadRows(ctx, prefixRange, func(row bigtable.Row) bool {
				atomic.AddInt32(&count, 1)
				return true
			}, bigtable.RowFilter(
				bigtable.ChainFilters(
					bigtable.StripValueFilter(), // https://cloud.google.com/bigtable/docs/using-filters#strip-value
					bigtable.CellsPerRowLimitFilter(1),
					bigtable.LatestNFilter(1),
				),
			))
		})
	}

	err := eg.Wait()
	sklog.Infof("Counted %d trace options for tile %d", count, tk)
	return err
}

func deleteTraceDataForTile(ctx context.Context, ac *bigtable.AdminClient, tk int) error {
	for shard := 0; shard < bt_tracestore.DefaultShards; shard++ {
		shardedPrefix := fmt.Sprintf("%02d:ts:t:%d", shard, tk)
		sklog.Infof("Drop range %s", shardedPrefix)
		err := ac.DropRowRange(ctx, traceBTTable, shardedPrefix)
		if err != nil {
			return err
		}
	}
	sklog.Infof("finished dropping traces in tile %d", tk)
	return nil
}

func deleteTraceOptionsForTile(ctx context.Context, ac *bigtable.AdminClient, tk int) error {
	for shard := 0; shard < bt_tracestore.DefaultShards; shard++ {
		shardedPrefix := fmt.Sprintf("%02d:ts:p:%d", shard, tk)
		sklog.Infof("Drop range %s", shardedPrefix)
		err := ac.DropRowRange(ctx, traceBTTable, shardedPrefix)
		if err != nil {
			return err
		}
	}
	sklog.Infof("finished dropping options in tile %d", tk)
	return nil
}

func deleteOPSForTile(ctx context.Context, ac *bigtable.AdminClient, tk int) error {
	prefix := fmt.Sprintf(":ts:o:%d", tk)
	sklog.Infof("Drop range %s", prefix)
	err := ac.DropRowRange(ctx, traceBTTable, prefix)
	if err != nil {
		return err
	}
	sklog.Infof("finished dropping OPS in tile %d", tk)
	return nil
}

func readTracesForTile(ctx context.Context, btc bt_tracestore.BTConfig, tk bt_tracestore.TileKey) {
	traceStore, err := bt_tracestore.New(ctx, btc, false)
	if err != nil {
		sklog.Fatalf("Could not instantiate BT tracestore: %s", err)
	}

	sklog.Infof("Going to fetch tile %d", tk)

	traceMap, _, err := traceStore.DEBUG_getTracesInRange(ctx, tk, tk, 0, 255)
	if err != nil {
		sklog.Fatalf("Could not get traces: %s", err)
	}
	for id, trace := range traceMap {
		if trace.Keys["extra_config"] == "Direct3D" && trace.Keys["cpu_or_gpu_value"] == "RadeonHD7770" {
			sklog.Infof("trace %s has digests %q", id, trace.Digests)
		}
	}
}
