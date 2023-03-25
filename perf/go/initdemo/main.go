// This application creates the 'demo' database on a local CockroachDB instance
// and also applies the latest schema.
package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/sql"
)

const (
	databaseName = "demo"

	connectionString = "postgresql://root@127.0.0.1:26257/?sslmode=disable"
)

func main() {
	ctx := context.Background()

	// Connect to database.
	conn, err := pgxpool.Connect(ctx, connectionString)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create the database.
	_, err = conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %s;`, databaseName))
	if err != nil {
		sklog.Infof("Database %s already exists.", databaseName)
	}

	_, err = conn.Exec(ctx, fmt.Sprintf(`SET DATABASE = %s;`, databaseName))
	if err != nil {
		sklog.Fatal(err)
	}

	// Apply the schema.
	_, err = conn.Exec(ctx, sql.Schema)
	if err != nil {
		sklog.Fatal(err)
	}

	conn.Close()
}
