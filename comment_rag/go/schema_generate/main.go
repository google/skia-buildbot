// This executable generates a go file that contains the SQL schema for
// the comment rag service defined as a string.
package main

import (
	"flag"
	"os"
	"path/filepath"

	"go.skia.org/infra/comment_rag/go/spanner"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/exporter"
)

// Collection of the tables in the SQL database.
type Tables struct {
	CLInfo         []spanner.CLInfo
	CommentHistory []spanner.CommentHistory
}

func main() {
	// Command line flags.
	var ()
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		sklog.Fatalf("Could not get working dir: %s", err)
	}

	outputFileName := "schema.go"
	packageName := "spanner"
	packagePath := filepath.Join(cwd, "spanner")
	schemaTargetDB := exporter.Spanner

	spannerConverter := exporter.DefaultSpannerConverter()
	spannerConverter.GoogleSQL = true
	spannerConverter.TtlExcludeTables = []string{
		"CommentHistory",
		"CLInfo",
	}
	spannerConverter.SkipCreatedAt = true
	generatedText := exporter.GenerateSQL(Tables{}, packageName, exporter.SchemaAndColumnNames, schemaTargetDB, spannerConverter)
	out := filepath.Join(packagePath, outputFileName)

	if err := os.MkdirAll(packagePath, 0755); err != nil {
		sklog.Fatalf("Could not create package path %s: %s", packagePath, err)
	}

	err = os.WriteFile(out, []byte(generatedText), 0666)
	if err != nil {
		sklog.Fatalf("Could not write SQL to %s: %s", out, err)
	}
}
