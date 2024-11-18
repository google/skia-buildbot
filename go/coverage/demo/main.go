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
	"go.skia.org/infra/go/coverage/coveragestore/sqlcoveragestore/coverageschema/spanner"
	"go.skia.org/infra/go/sklog"
)

var (
	configFile = flag.String("config_filename", "demo.json", "Config file to use.")
)

const (
	CockroachDB string = "cockroachdb"
	Spanner     string = "spanner"
)

func main() {
	sklog.Debug("Running Demo...")
	ctx := context.Background()
	flag.Parse()
	sklog.Infof("CONFIG: %s", *configFile)
	var coverageConfig config.CoverageConfig
	config, err := coverageConfig.LoadCoverageConfig(*configFile)
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

	if config.DatabaseType == CockroachDB {
		_, err = conn.Exec(ctx, fmt.Sprintf(`SET DATABASE = %s;`, databaseName))
		if err != nil {
			sklog.Fatal(err)
		}
	}

	schema := coverageschema.Schema
	if config.DatabaseType == Spanner {
		schema = spanner.Schema
	}

	// Apply the schema.
	_, err = conn.Exec(ctx, schema)
	if err != nil {
		sklog.Fatal(err)
	}

	conn.Close()
}
