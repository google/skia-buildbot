// Package exportschema contains a re-usable Main function that exports an SQL
// schema as a JSON seriailized schema.Description.
package exportschema

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/schema"
	"go.skia.org/infra/go/util"
)

// Main exports the expected schema as a serialized schema.Description.
//
// Note that this works by requiring a local instance of the CockroachDB
// emulator to be running, so this is not appropriate to call in a go:generate
// statement.
func Main(out string, dbType string, tables interface{}, schemaAsString string) error {
	if out == "" {
		return skerr.Fmt("--out flag must be supplied")
	}

	ctx := context.Background()
	rand.Seed(time.Now().UnixNano())
	databaseName := fmt.Sprintf("%s_%d", "export", rand.Uint64())
	host := emulators.GetEmulatorHostEnvVar(emulators.CockroachDB)
	if dbType == schema.SpannerDBType {
		host = "localhost:5432"
	}
	connectionString := fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", host, databaseName)
	db, err := pgxpool.Connect(ctx, connectionString)
	if err != nil {
		sklog.Fatal(err)
	}

	if dbType != schema.SpannerDBType {
		// Create the database in cockroachdb.
		_, err = db.Exec(ctx, fmt.Sprintf(`
				CREATE DATABASE %s;
				SET DATABASE = %s;`, databaseName, databaseName))
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Create the tables.
	_, err = db.Exec(ctx, schemaAsString)
	if err != nil {
		sklog.Fatal(err)
	}

	sch, err := schema.GetDescription(ctx, db, tables, dbType)
	if err != nil {
		sklog.Fatal(err)
	}

	err = util.WithWriteFile(out, func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(sch)
	})
	if err != nil {
		sklog.Fatal(err)
	}

	db.Close()
	return nil
}
