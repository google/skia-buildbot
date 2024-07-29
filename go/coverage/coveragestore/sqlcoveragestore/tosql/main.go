// This executable generates a go file that contains the SQL schema for
// machineserver defined as a string. By doing this, we have the source of truth
// as a documented go struct, which can be used in a more flexible way than
// having the SQL as the source of truth.
package main

//go:generate bazelisk run --config=mayberemote //:go -- run ../tosql

import (
	"os"
	"path"
	"path/filepath"
	"runtime"

	coverageschema "go.skia.org/infra/go/coverage/coveragestore/sqlcoveragestore/coverageschema"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/exporter"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		sklog.Fatalf("Could not get working dir: %s, %s", err, cwd)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("No caller information")
	}

	generatedText := exporter.GenerateSQL(coverageschema.Tables{}, "coverageschema", exporter.SchemaAndColumnNames)
	out := filepath.Join(path.Dir(path.Dir(filename)), "coverageschema", "coverageschema.go")
	err = os.WriteFile(out, []byte(generatedText), 0666)
	if err != nil {
		sklog.Fatalf("Could not write SQL to %s: %s", out, err)
	}
}
