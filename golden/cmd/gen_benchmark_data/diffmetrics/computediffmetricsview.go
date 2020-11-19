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
	maxSQLConnections = 4
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
InitialRankings AS (
	SELECT DiffMetrics.*, rank() over (
		PARTITION BY left_digest
		ORDER BY num_diff_pixels ASC, pixel_diff_percent ASC, max_channel_diff ASC
	) AS initialRank
	FROM DiffMetrics
	where left_digest > $1 and left_digest < $2
),
DiffsWithLabels AS (
    SELECT DISTINCT InitialRankings.*, max(label) OVER(PARTITION BY right_digest) AS max_label
    FROM InitialRankings
    JOIN Expectations ON Expectations.digest = InitialRankings.right_digest
    WHERE initialRank < 100
),
RankedDiffs AS (
	SELECT DiffsWithLabels.*, rank() over (
		PARTITION BY left_digest, max_label
		ORDER BY num_diff_pixels ASC, pixel_diff_percent ASC, max_channel_diff ASC
	) AS diffRank
	from DiffsWithLabels
),
TopOfEachLabel AS (
	SELECT RankedDiffs.*, rank() OVER (
		PARTITION BY left_digest
		ORDER BY num_diff_pixels ASC, pixel_diff_percent ASC, max_channel_diff ASC
	) AS overallRank
	FROM RankedDiffs
	WHERE (RankedDiffs.max_label = 0 AND RankedDiffs.diffRank <= 3) OR
	      (RankedDiffs.max_label = 1 AND RankedDiffs.diffRank <= 5) OR
	      (RankedDiffs.max_label = 2 AND RankedDiffs.diffRank <= 2)
)
UPSERT INTO DiffMetricsClosestView
SELECT left_digest, overallRank, right_digest, num_diff_pixels, pixel_diff_percent,
	max_channel_diff, max_rgba_diff, dimensions_differ
FROM TopOfEachLabel;`

const lastStatement = `
WITH
InitialRankings AS (
	SELECT DiffMetrics.*, dense_rank() over (
		PARTITION BY left_digest
		ORDER BY num_diff_pixels ASC, pixel_diff_percent ASC, max_channel_diff ASC
	) AS initialRank
	from DiffMetrics
	where left_digest > x'ff'
),
DiffsWithLabels AS (
    SELECT DISTINCT InitialRankings.*, max(label) OVER(PARTITION BY right_digest) AS max_label
    FROM InitialRankings
    JOIN Expectations ON Expectations.digest = InitialRankings.right_digest
    WHERE initialRank < 100
),
RankedDiffs AS (
	SELECT DiffsWithLabels.*, dense_rank() over (
		PARTITION BY left_digest, max_label
		ORDER BY num_diff_pixels ASC, pixel_diff_percent ASC, max_channel_diff ASC
	) AS diffRank
	from DiffsWithLabels
),
TopOfEachLabel AS (
	SELECT RankedDiffs.*, dense_rank() OVER (
		PARTITION BY left_digest
		ORDER BY num_diff_pixels ASC, pixel_diff_percent ASC, max_channel_diff ASC
	) AS overallRank
	FROM RankedDiffs
	WHERE (RankedDiffs.max_label = 0 AND RankedDiffs.diffRank <= 3) OR
	      (RankedDiffs.max_label = 1 AND RankedDiffs.diffRank <= 5) OR
	      (RankedDiffs.max_label = 2 AND RankedDiffs.diffRank <= 2)
)
UPSERT INTO DiffMetricsClosestView
SELECT left_digest, overallRank, right_digest, num_diff_pixels, pixel_diff_percent,
	max_channel_diff, max_rgba_diff, dimensions_differ
FROM TopOfEachLabel;`

func computeDiffMetricsViewInShards(ctx context.Context, db *pgxpool.Pool) error {
	eg, ctx := errgroup.WithContext(ctx)
	for i := byte(0); i < 255; i++ {
		start := []byte{i}
		end := []byte{i + 1}
		eg.Go(func() error {
			_, err := db.Exec(ctx, rangeStatement, start, end)
			sklog.Infof("Range %x finished.", start)
			return skerr.Wrapf(err, "Range starting at %x", start)
		})
	}

	eg.Go(func() error {
		_, err := db.Exec(ctx, lastStatement)
		return skerr.Wrapf(err, "Range 0xff finished")
	})

	return eg.Wait()
}
