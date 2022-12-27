// This executable generates a go file that contains the SQL schema for Gold defined as a string.
// By doing this, we have the source of truth as a documented go struct, which can be used in a
// more flexible way than having the SQL as the source of truth.
package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/exporter"
	"go.skia.org/infra/golden/go/sql/schema"
)

func main() {
	outputFile := flag.String("output_file", "", "The name of the file to write to.")
	outputPkg := flag.String("output_pkg", "", "The name of the package to output to.")
	flag.Parse()
	cwd, err := os.Getwd()
	if err != nil {
		sklog.Fatalf("Could not get working dir")
	}

	generatedText := exporter.GenerateSQL(schema.Tables{}, *outputPkg)
	out := filepath.Join(cwd, *outputFile)
	err = ioutil.WriteFile(out, []byte(generatedText), 0666)
	if err != nil {
		sklog.Fatalf("Could not write SQL to %s: %s", out, err)
	}
	sklog.Infof("Tables schema written to %s.\n", out)
}
