// Application exportschema exports the expected schema as a serialized schema.Description.
package main

import (
	"flag"
	"os"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/schema/exportschema"
	"go.skia.org/infra/machine/go/machine/store/cdb"
	"go.skia.org/infra/machine/go/machine/store/cdb/spanner"
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

	schema := cdb.Schema
	if dbType == "spanner" {
		schema = spanner.Schema
	}
	err = exportschema.Main(out, dbType, cdb.Tables{}, schema)
	if err != nil {
		sklog.Fatal(err)
	}
}
