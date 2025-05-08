package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/sql"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/schema/spanner"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

// flags
var (
	databaseName  = flag.String("databasename", "gold", "Name of the database.")
	databaseUrl   = flag.String("database_url", "postgresql://root@127.0.0.1:26257/?sslmode=disable", "Connection url to the database.")
	enableSpanner = flag.Bool("spanner", false, "Set to true if running against the spanner emulator.")
)

func main() {
	ctx := context.Background()
	flag.Parse()

	// Connect to database.
	conn, err := pgxpool.Connect(ctx, *databaseUrl)
	if err != nil {
		sklog.Fatalf("Failed to connect to database using URL %q: %s", *databaseUrl, err)
	}
	defer conn.Close()

	// Create the database.
	_, err = conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %s;`, *databaseName))
	if err != nil {
		sklog.Infof("Database %q may already exist: %s", *databaseName, err)
	} else {
		sklog.Infof("Database %q created or creation statement executed.", *databaseName)
	}
	dbSchema := schema.Schema
	dbSchemaName := "CockroachDB"
	if *enableSpanner {
		dbSchema = spanner.Schema
		dbSchemaName = "Spanner"
		sklog.Info("Using Spanner schema.")
	} else {
		sklog.Info("Using CockroachDB schema.")
	}

	// Apply the selected schema.
	sklog.Infof("Applying %s database schema...", dbSchemaName)
	_, err = conn.Exec(ctx, dbSchema)
	if err != nil {
		sklog.Fatalf("Failed to apply schema: %s", err)
	}
	sklog.Info("Schema successfully applied.")

	sklog.Infof("Inserting test data...")
	data := dks.Build()
	err = sqltest.BulkInsertDataTables(ctx, conn, data)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Base test data successfully added to the database.")

	sklog.Info("Deleting pre-calculated diffs for test pair (runs for both CDB and Spanner)...")
	digestBytesA01, _ := sql.DigestToBytes(dks.DigestA01Pos)
	digestBytesA05, _ := sql.DigestToBytes(dks.DigestA05Unt)
	deletePair := newDigestPairBytes(digestBytesA01, digestBytesA05)
	deleteStatement := `DELETE FROM DiffMetrics WHERE left_digest = $1 AND right_digest = $2`
	_, err = conn.Exec(ctx, deleteStatement, deletePair.left, deletePair.right)
	if err != nil {
		sklog.Warningf("Could not delete diff metric (%x, %x): %s", deletePair.left, deletePair.right, err)
	}
	_, err = conn.Exec(ctx, deleteStatement, deletePair.right, deletePair.left) // Delete symmetric pair
	if err != nil {
		sklog.Warningf("Could not delete symmetric diff metric (%x, %x): %s", deletePair.right, deletePair.left, err)
	} else {
		sklog.Infof("Deleted pre-calculated diff metric for pair (%s, %s) (if it existed).", dks.DigestA01Pos, dks.DigestA05Unt)
	}

	sklog.Info("Upserting diffcalculator work items (runs for both CDB and Spanner)...")
	groupingIDForWork := dks.SquareGroupingID
	digestsForWork := []string{string(dks.DigestA01Pos), string(dks.DigestA05Unt)}
	pastTime := time.Now().Add(-1 * time.Hour)
	currentTime := time.Now()

	// 1. Upsert Primary Branch Work Item
	primaryWorkStatement := `
        INSERT INTO PrimaryBranchDiffCalculationWork
          (grouping_id, last_calculated_ts, calculation_lease_ends)
        VALUES ($1, $2, $3)
        ON CONFLICT (grouping_id) DO UPDATE SET
          last_calculated_ts = excluded.last_calculated_ts,
          calculation_lease_ends = excluded.calculation_lease_ends,
          grouping_id = excluded.grouping_id`
	_, err = conn.Exec(ctx, primaryWorkStatement, groupingIDForWork, pastTime, pastTime)
	if err != nil {
		sklog.Fatalf("Failed to upsert primary branch work for SquareGroupingID: %s", err)
	} else {
		sklog.Info("Successfully upserted primary branch work item for SquareGroupingID.")
	}

	// 2. Upsert Secondary Branch Work Item
	secondaryWorkStatement := `
        INSERT INTO SecondaryBranchDiffCalculationWork
          (branch_name, grouping_id, digests, last_updated_ts, last_calculated_ts, calculation_lease_ends)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (branch_name, grouping_id) DO UPDATE SET
          digests = excluded.digests,
          last_updated_ts = excluded.last_updated_ts,
          last_calculated_ts = excluded.last_calculated_ts,
          calculation_lease_ends = excluded.calculation_lease_ends,
          branch_name = excluded.branch_name,
          grouping_id = excluded.grouping_id`
	branchName := "cl_trigger_worker_test"
	_, err = conn.Exec(ctx, secondaryWorkStatement, branchName, groupingIDForWork, digestsForWork, currentTime, pastTime, pastTime)
	if err != nil {
		sklog.Fatalf("Failed to upsert secondary branch work for SquareGroupingID: %s", err)
	} else {
		sklog.Info("Successfully upserted secondary branch work item for SquareGroupingID.")
	}

	sklog.Infof("Local setup script finished successfully.")
}

type digestPairBytes struct {
	left  []byte
	right []byte
}

func newDigestPairBytes(one, two []byte) digestPairBytes {
	if bytes.Compare(one, two) < 0 {
		return digestPairBytes{left: one, right: two}
	}
	return digestPairBytes{left: two, right: one}
}
