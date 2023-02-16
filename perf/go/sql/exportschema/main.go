// Application exportschema exports the expected schema as a serialized schema.Description.
package main

import (
	"os"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/schema/exportschema"
	"go.skia.org/infra/perf/go/sql"
)

func main() {
	err := exportschema.Main(os.Args, sql.Tables{}, sql.Schema)
	if err != nil {
		sklog.Fatal(err)
	}
}
