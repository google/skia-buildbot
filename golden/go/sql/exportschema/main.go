// Application exportschema exports the expected schema as a serialized schema.Description.
package main

import (
	"flag"
	"os"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/schema/exportschema"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/schema/spanner"
)

func main() {
	var dbType string
	var out string
	fs := flag.NewFlagSet("exportschema", flag.ExitOnError)
	fs.StringVar(&dbType, "databaseType", "", "Database type for the schema.")
	fs.StringVar(&out, "out", "", "Filename of the schema Description to write.")

	err := fs.Parse(os.Args[1:])
	if err != nil {
		sklog.Fatalf("Error parsing arguments: %v", err)
	}

	current_schema := schema.Schema
	if dbType == "spanner" {
		current_schema = spanner.Schema
	}
	err = exportschema.Main(out, dbType, schema.Tables{}, current_schema)
	if err != nil {
		sklog.Fatal(err)
	}
}
