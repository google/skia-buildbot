// This executable generates a go file that contains the SQL schema for
// questagent defined as a string.
package main

import (
	"os"
	"path/filepath"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/exporter"
	sql "go.skia.org/infra/perf/go/questagent/db"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		sklog.Fatalf("Could not get working dir: %s", err)
	}

	generatedText := exporter.GenerateSQL(sql.Tables{}, "sql", exporter.SchemaAndColumnNames)
	out := filepath.Join(cwd, "schema.go")
	err = os.WriteFile(out, []byte(generatedText), 0666)
	if err != nil {
		sklog.Fatalf("Could not write SQL to %s: %s", out, err)
	}
}
