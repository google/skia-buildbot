// This executable generates a go file that contains the SQL schema for
// machineserver defined as a string. By doing this, we have the source of truth
// as a documented go struct, which can be used in a more flexible way than
// having the SQL as the source of truth.
package main

//go:generate bazelisk run --config=mayberemote //:go -- run ../tosql
//go:generate bazelisk run --config=mayberemote //:go -- run ../tosql --schemaTarget spanner

import (
	"flag"
	"os"
	"path"
	"path/filepath"
	"runtime"

	coverageschema "go.skia.org/infra/go/coverage/coveragestore/sqlcoveragestore/coverageschema"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/exporter"
)

func main() {
	// Command line flags.
	var (
		schemaTarget = flag.String("schemaTarget", "cockroachdb", "Target for the generated schema. Eg: CockroachDB, Spanner")
	)

	// Parse the cmdline flags.
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		sklog.Fatalf("Could not get working dir: %s, %s", err, cwd)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("No caller information")
	}

	outputFileName := "coverageschema.go"
	packageName := "coverageschema"
	packagePath := "coverageschema"
	schemaTargetDB := exporter.CockroachDB
	if *schemaTarget == "spanner" {
		outputFileName = "coverageschema_spanner.go"
		schemaTargetDB = exporter.Spanner
		packageName = "spanner"
		packagePath = filepath.Join(packagePath, "spanner")
	}
	generatedText := exporter.GenerateSQL(coverageschema.Tables{}, packageName, exporter.SchemaAndColumnNames, schemaTargetDB)
	out := filepath.Join(path.Dir(path.Dir(filename)), packagePath, outputFileName)
	err = os.WriteFile(out, []byte(generatedText), 0666)
	if err != nil {
		sklog.Fatalf("Could not write SQL to %s: %s", out, err)
	}
}
