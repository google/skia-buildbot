package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/sklog"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

// flags
var (
	databaseName = flag.String("databasename", "gold", "Name of the database.")
	databaseUrl  = flag.String("database_url", "postgresql://root@127.0.0.1:26257/?sslmode=disable", "Connection url to the database.")
)

func main() {
	ctx := context.Background()
	flag.Parse()

	// Connect to database.
	conn, err := pgxpool.Connect(ctx, *databaseUrl)
	defer conn.Close()
	if err != nil {
		sklog.Fatal(err)
	}

	// Create the database.
	_, err = conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %s;`, *databaseName))
	if err != nil {
		sklog.Infof("Database %s already exists.", databaseName)
	}

	_, err = conn.Exec(ctx, fmt.Sprintf(`SET DATABASE = %s;`, *databaseName))
	if err != nil {
		sklog.Fatal(err)
	}

	// Apply the schema.
	_, err = conn.Exec(ctx, schema.Schema)
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Inserting test data...")
	data := dks.Build()
	err = sqltest.BulkInsertDataTables(ctx, conn, data)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Test data successfully added to the database.")
}
