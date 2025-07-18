// This application creates the 'demo' database on a local Spanner instance
// and also applies the latest schema.
package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/sql/spanner"
)

// flags
var (
	databaseName = flag.String("databasename", "demo", "Name of the database.")
	databaseUrl  = flag.String("database_url", "postgresql://root@127.0.0.1:26257/?sslmode=disable", "Connection url to the database.")
)

func main() {
	ctx := context.Background()
	flag.Parse()

	// Connect to database.
	conn, err := pgxpool.Connect(ctx, *databaseUrl)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create the database.
	_, err = conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %s;`, *databaseName))
	if err != nil {
		sklog.Infof("Database %s already exists.", *databaseName)
	}

	dbSchema := spanner.Schema

	// Apply the schema.
	_, err = conn.Exec(ctx, dbSchema)
	if err != nil {
		sklog.Fatal(err)
	}

	conn.Close()
}
