// Command line tool to set up the CockroachDB database for machineserver.
//
// Before running this command you must run:
//
//    $ kubectl port-forward service/machineserver-cockroachdb-public 25001:26257
package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/machine/store/cdb"
)

func main() {
	ctx := context.Background()

	connectionString := fmt.Sprintf("postgresql://root@127.0.0.1:25001/%s?sslmode=disable", cdb.DatabaseName)
	db, err := pgxpool.Connect(ctx, connectionString)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create the database in cockroachdb.
	_, err = db.Exec(ctx, fmt.Sprintf(`
		CREATE DATABASE IF NOT EXISTS %s;
		SET DATABASE = %s;`, cdb.DatabaseName, cdb.DatabaseName))
	if err != nil {
		sklog.Fatal(err)
	}

	// Create the tables.
	_, err = db.Exec(ctx, cdb.Schema)
	if err != nil {
		sklog.Fatal(err)
	}

	db.Close()
}
