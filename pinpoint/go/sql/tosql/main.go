// This executable generates a go file that contains the SQL schema for
// Jobs defined as a string. By doing this, we have the source of truth
// as a documented go struct, which can be used in a more flexible way than
// having the SQL as the source of truth.

package main

import (
	"os"
	"path/filepath"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/exporter"
	"go.skia.org/infra/pinpoint/go/sql/schema"
)

const (
	outputFileName = "spanner.go"
	packageName    = "spanner"
)

// Schema for a Job is defined as a struct in pinpoint/go/sql/schema
type spannerTables struct {
	Jobs []schema.JobSchema
}

func main() {

	cwd, err := os.Getwd()
	if err != nil {
		sklog.Fatalf("Could not get working directory: %s", err)
	}

	packagePath := cwd
	schemaTarget := exporter.Spanner
	packagePath = filepath.Join(packagePath, "schema/spanner")

	generatedSqlSchema := exporter.GenerateSQL(
		spannerTables{},
		packageName,
		exporter.SchemaOnly,
		schemaTarget,
		&exporter.SpannerConverter{SkipCreatedAt: true})

	out := filepath.Join(packagePath, outputFileName)
	err = os.WriteFile(out, []byte(generatedSqlSchema), 0666)
	if err != nil {
		sklog.Fatalf("Could not write SQL to %s: %s", out, err)
	}
	sklog.Debugf("Tables schema written to %s.", out)
}
