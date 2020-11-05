package main

import (
	"context"
	"flag"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/sync/errgroup"
)

const (
	maxSQLConnections = 24
)

func main() {
	port := flag.String("port", "26234", "Port on localhost to connect to. Only set if --local=false")
	flag.Parse()

	sklog.Infof("Not using local db. Make sure you ran kubectl port-forward gold-cockroachdb-0 26234:26234")

	ctx := context.Background()
	conf, err := pgxpool.ParseConfig("postgresql://root@localhost:" + *port + "/benchmark_db?sslmode=disable")
	if err != nil {
		sklog.Fatalf("error getting postgress config: %s", err)
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

const rangeStatement = `
WITH
RankedDiffs AS (
	SELECT DiffMetrics.*, dense_rank() over (
		PARTITION BY left_digest
		ORDER BY num_diff_pixels ASC, pixel_diff_percent ASC, max_channel_diff ASC
	) AS diffRank
	from DiffMetrics
	where left_digest > $1 and left_digest < $2
),
TopTenDiffs AS (
	SELECT * FROM RankedDiffs where diffRank <= 10
)
UPSERT INTO DiffMetricsClosestView
SELECT left_digest, diffRank, right_digest, num_diff_pixels, pixel_diff_percent,
	max_channel_diff, max_rgba_diff, dimensions_differ
FROM TopTenDiffs;`

const lastStatement = `
WITH
RankedDiffs AS (
	SELECT DiffMetrics.*, dense_rank() over (
		PARTITION BY left_digest
		ORDER BY num_diff_pixels ASC, pixel_diff_percent ASC, max_channel_diff ASC
	) AS diffRank
	from DiffMetrics
	where left_digest > x'ff'
),
TopTenDiffs AS (
	SELECT * FROM RankedDiffs where diffRank <= 10
)
UPSERT INTO DiffMetricsClosestView
SELECT left_digest, diffRank, right_digest, num_diff_pixels, pixel_diff_percent,
	max_channel_diff, max_rgba_diff, dimensions_differ
FROM TopTenDiffs;`

func computeDiffMetricsViewInShards(ctx context.Context, db *pgxpool.Pool) error {
	eg, ctx := errgroup.WithContext(ctx)
	for i := byte(0); i < 255; i++ {
		start := []byte{i}
		end := []byte{i + 1}
		eg.Go(func() error {
			_, err := db.Exec(ctx, rangeStatement, start, end)
			sklog.Infof("Range starting at %x finished.", start)
			return skerr.Wrapf(err, "Range starting at %x", start)
		})
	}

	eg.Go(func() error {
		_, err := db.Exec(ctx, lastStatement)
		return skerr.Wrapf(err, "Range starting at 0xff")
	})

	return eg.Wait()
}
