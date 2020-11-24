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
	maxSQLConnections = 4
)

func main() {
	port := flag.String("port", "26234", "Port on localhost to connect to. Only set if --local=false")
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

	if err := computeDiffMetricsViewInShards(ctx, db); err != nil {
		sklog.Fatalf("Computing data %s", err)
	}
}

func computeDiffMetricsViewInShards(ctx context.Context, db *pgxpool.Pool) error {
	eg, ctx := errgroup.WithContext(ctx)
	for i := byte(0); i < 255; i++ {
		eg.Go(func() error {
			_, err := db.Exec(ctx, statements.CreateDiffMetricsClosestViewShard(i))
			sklog.Infof("Range %x finished.", i)
			return skerr.Wrapf(err, "Range starting at %x", i)
		})
	}
	eg.Go(func() error {
		_, err := db.Exec(ctx, statements.CreateDiffMetricsClosestViewShard(255))
		sklog.Infof("Range %x finished.", 255)
		return skerr.Wrapf(err, "Range starting at %x", 255)
	})

	return eg.Wait()
}
