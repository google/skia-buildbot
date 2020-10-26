package main

import (
	"context"
	"flag"
	"math/rand"
	"os/exec"
	"time"

	"github.com/jackc/pgx/v4"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	local := flag.Bool("local", true, "Spin up a local instance of cockroachdb. If false, will connect to local port 26257.")
	port := flag.String("port", "26257", "Port on localhost to connect to. Only set if --local=false")

	flag.Parse()
	if *local {
		storageDir, err := sqltest.StartLocalCockroachDB()
		if err != nil {
			sklog.Fatalf("Could not start local cockroachdb: %s", err)
		}
		sklog.Infof("Check out localhost:8080 and %s for storage", storageDir)
	} else {
		sklog.Infof("Not using local db. Make sure you ran kubectl port-forward gold-cockroachdb-0 26234:26234")
	}

	if err := createBenchmarkDBAndTables(*port); err != nil {
		sklog.Fatalf("Could not initialize db/tables: %s", err)
	}

	ctx := context.Background()
	conf, err := pgx.ParseConfig("postgresql://root@localhost:" + *port + "/benchmark_db?sslmode=disable")
	if err != nil {
		sklog.Fatalf("error getting postgress config: %s", err)
	}
	db, err := pgx.ConnectConfig(ctx, conf)
	if err != nil {
		sklog.Fatalf("error connecting to the database: %s", err)
	}
	defer db.Close(ctx)

	if err := fillData(ctx, db); err != nil {
		sklog.Fatalf("Could not fill with data: %s", err)
	}

	sklog.Infof("Done.")
}

func createBenchmarkDBAndTables(port string) error {
	out, err := exec.Command("cockroach", "sql", "--insecure", "--host=localhost:"+port,
		`--execute=CREATE DATABASE IF NOT EXISTS benchmark_db;`).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "creating database: %s", out)
	}

	out, err = exec.Command("cockroach", "sql", "--insecure", "--host=localhost:"+port,
		"--database=benchmark_db", // Connect to benchmark_db that we just made
		"--execute="+sql.CockroachDBSchema,
	).CombinedOutput()
	if err != nil {
		return skerr.Wrapf(err, "creating tables: %s", out)
	}
	sklog.Infof("benchmark DB setup")
	return nil
}

func fillData(ctx context.Context, db *pgx.Conn) error {
	tracePairs := generateTracesForTest(randomTestSettings{
		Corpus:               "gm",
		TestName:             "blend_modes",
		NumCommits:           10,
		MinAdditionalKeys:    8,
		MaxAdditionalKeys:    11,
		MaxAdditionalOptions: 4,
		MinTraceDensity:      0.2,
		MaxTraceDensity:      0.99,
		NumTraces:            5,
		MinTraceFlakiness:    2,
		MaxTraceFlakiness:    20,
		TraceDigestOverlap:   0.20,
		GlobalDigestOverlap:  0.05,
	})
	sklog.Infof("Should add %v", tracePairs)
	return skerr.Fmt("not impl")
}
