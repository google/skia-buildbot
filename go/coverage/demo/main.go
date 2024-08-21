// This application creates the 'coverage' database on a local CockroachDB instance
// and also applies the latest schema.
package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/coverage/config"
	coverageschema "go.skia.org/infra/go/coverage/coveragestore/sqlcoveragestore/coverageschema"
	"go.skia.org/infra/go/sklog"
)

func main() {
	sklog.Debug("Running Demo...")
	ctx := context.Background()
	flag.Parse()
	var coverageConfig config.CoverageConfig
	config, err := coverageConfig.LoadCoverageConfig("demo.json")
	if err != nil {
		sklog.Fatal(err)
	}
	databaseName := config.GetDatabaseName()
	connectionString := config.GetConnectionString()

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
	_, err = conn.Exec(ctx, coverageschema.Schema)
	if err != nil {
		sklog.Fatal(err)
	}

	conn.Close()
}
