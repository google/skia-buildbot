package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/sql/datakitchensink"
)

func main() {
	outputDir := flag.String("output_folder", "", "The name of the folder to write to.")
	flag.Parse()
	cwd, err := os.Getwd()
	if err != nil {
		sklog.Fatalf("Could not get working dir: %s", err)
	}

	data := datakitchensink.Build()

	if err := os.MkdirAll(filepath.Join(cwd, *outputDir), 0777); err != nil {
		sklog.Fatalf("Could not make output dir %s", err)
	}

	generatedText := generateTSV(data)
	for dst, content := range generatedText {
		out := filepath.Join(cwd, *outputDir, dst+".tsv")
		err = ioutil.WriteFile(out, []byte(content), 0666)
		if err != nil {
			sklog.Fatalf("Could not write SQL to %s: %s", out, err)
		}
	}

	sklog.Infof("Data TSV written to %s.\n", *outputDir)
}

// The TSVExporter interface allows structs to be exported to TSV files such that they can be
// ingested by a SQL database (such as CockroachDB).
type TSVExporter interface {
	// ToTSV returns the object as a single TSV row. It is important that the column order lines
	// up with the order in which the columns were used to create the table.
	ToTSV() string
}

// generateTSV takes in a "table type", that is a table whose fields are slices. Each field
// will be interpreted as a table. Each field should be a slice of objects that implement the
// TSVExporter interface. This function will call ToTSV on each "row" of each table and concatenate
// them together. The return value is a map of the field name (i.e. the table name) to the
// concatenated TSV rows.
func generateTSV(inputType interface{}) map[string]string {
	rv := map[string]string{}
	body := strings.Builder{}
	v := reflect.ValueOf(inputType)
	for i := 0; i < v.NumField(); i++ {
		tableName := v.Type().Field(i).Name
		table := v.Field(i) // Fields of the outer type are expected to be tables.
		if table.Kind() != reflect.Slice {
			panic(`Expected table should be a slice: ` + tableName)
		}

		// Go through each element of the table slice, cast it to TSVExporter and then call that
		// function on it.
		for j := 0; j < table.Len(); j++ {
			r := table.Index(j)
			row, ok := r.Interface().(TSVExporter)
			if !ok {
				panic(`Expected table should be a slice of types that implement ToTSV: ` + tableName)
			}
			body.WriteString(row.ToTSV())
			body.WriteRune('\n')
		}
		rv[tableName] = body.String()
		body.Reset()
	}
	return rv
}
