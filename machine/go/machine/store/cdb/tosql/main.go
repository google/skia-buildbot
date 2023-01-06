// This executable generates a go file that contains the SQL schema for
// machineserver defined as a string. By doing this, we have the source of truth
// as a documented go struct, which can be used in a more flexible way than
// having the SQL as the source of truth.
package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/exporter"
	"go.skia.org/infra/machine/go/machine/store/cdb"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		sklog.Fatalf("Could not get working dir: %s", err)
	}

	generatedText := exporter.GenerateSQL(cdb.Tables{}, "cdb", exporter.SchemaAndColumnNames)
	out := filepath.Join(cwd, "sql.go")
	err = ioutil.WriteFile(out, []byte(generatedText), 0666)
	if err != nil {
		sklog.Fatalf("Could not write SQL to %s: %s", out, err)
	}
}
