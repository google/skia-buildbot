// Application exportschema exports the expected schema as a serialized schema.Description.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/schema"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine/store/cdb"
)

var (
	out = flag.String("out", "", "Filename of the schema Description.")
)

func main() {
	flag.Parse()
	if *out == "" {
		sklog.Fatal("--out flag must be supplied")
	}

	ctx := context.Background()
	rand.Seed(time.Now().UnixNano())
	databaseName := fmt.Sprintf("%s_%d", "export", rand.Uint64())
	host := emulators.GetEmulatorHostEnvVar(emulators.CockroachDB)
	connectionString := fmt.Sprintf("postgresql://root@%s/%s?sslmode=disable", host, databaseName)
	db, err := pgxpool.Connect(ctx, connectionString)
	if err != nil {
		sklog.Fatal(err)
	}

	// Create the database in cockroachdb.
	_, err = db.Exec(ctx, fmt.Sprintf(`
		CREATE DATABASE %s;
		SET DATABASE = %s;`, databaseName, databaseName))
	if err != nil {
		sklog.Fatal(err)
	}

	// Create the tables.
	_, err = db.Exec(ctx, cdb.Schema)
	if err != nil {
		sklog.Fatal(err)
	}

	sch, err := schema.GetDescription(db, cdb.Tables{})
	if err != nil {
		sklog.Fatal(err)
	}

	err = util.WithWriteFile(*out, func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(sch)
	})
	if err != nil {
		sklog.Fatal(err)
	}

	db.Close()
}
