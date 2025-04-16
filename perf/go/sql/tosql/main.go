// This executable generates a go file that contains the SQL schema for
// machineserver defined as a string. By doing this, we have the source of truth
// as a documented go struct, which can be used in a more flexible way than
// having the SQL as the source of truth.
package main

import (
	"flag"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/exporter"
	"go.skia.org/infra/perf/go/sql"
)

func main() {
	// Command line flags.
	var (
		schemaTarget = flag.String("schemaTarget", "cockroachdb", "Target for the generated schema. Eg: CockroachDB, Spanner")
	)
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		sklog.Fatalf("Could not get working dir: %s", err)
	}

	outputFileName := "schema.go"
	packageName := "sql"
	packagePath := cwd
	schemaTargetDB := exporter.CockroachDB
	if *schemaTarget == "spanner" {
		outputFileName = "schema_spanner.go"
		schemaTargetDB = exporter.Spanner
		packageName = "spanner"
		packagePath = filepath.Join(packagePath, "spanner")
	}

	ttlExcludeTables := []string{
		"Alerts",
		"Favorites",
		"Subscriptions",
		"TraceParams",
	}
	generatedText := exporter.GenerateSQL(sql.Tables{}, packageName, exporter.SchemaAndColumnNames, schemaTargetDB, ttlExcludeTables)
	out := filepath.Join(packagePath, outputFileName)
	err = os.WriteFile(out, []byte(generatedText), 0666)
	if err != nil {
		sklog.Fatalf("Could not write SQL to %s: %s", out, err)
	}
}
