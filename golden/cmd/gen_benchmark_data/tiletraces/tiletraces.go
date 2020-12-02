package main

import (
	"context"
	"flag"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/sql/statements"
	"golang.org/x/sync/errgroup"
)

const (
	maxSQLConnections = 1 // this number needs to be low to avoid contention in big tables.
)

func main() {
	port := flag.String("port", "26234", "Port on localhost to connect to. Only set if --local=false")
	tileWidth := flag.Int("tile_width", 100, "The width of the tile")
	startingCommitID := flag.Int("starting_commit_id", 0, "The starting commit id to tile")
	endingCommitID := flag.Int("ending_commit_id", 100, "The ending commit id")
	flag.Parse()

	sklog.Infof("Not using local db. Make sure you ran kubectl port-forward gold-cockroachdb-0 26234:26234")

	ctx := context.Background()
	conf, err := pgxpool.ParseConfig("postgresql://root@localhost:" + *port + "/benchmark_db?sslmode=disable")
	if err != nil {
		sklog.Fatalf("error getting postgres config: %s", err)
	}

	conf.MaxConns = maxSQLConnections
	db, err := pgxpool.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	defer db.Close()

	if err := computeTiledTraceDigestsInShards(ctx, db, *tileWidth, *startingCommitID, *endingCommitID); err != nil {
		sklog.Fatalf("tiling traces %s", err)
	}
}

func computeTiledTraceDigestsInShards(ctx context.Context, db *pgxpool.Pool, tileWidth, start, end int) error {
	eg, ctx := errgroup.WithContext(ctx)

	for commitID := start; commitID < end; commitID += tileWidth {
		for i := 0; i <= 255; i++ {
			b := byte(i)
			thisCommitID := commitID
			eg.Go(func() error {
				_, err := db.Exec(ctx, statements.CreateTiledTraceDigestsShard(b, thisCommitID, tileWidth))
				sklog.Infof("Shard %02x finished in range %d.", b, thisCommitID)
				return skerr.Wrapf(err, "Range starting at %02x", b)
			})
		}
	}

	return eg.Wait()
}
