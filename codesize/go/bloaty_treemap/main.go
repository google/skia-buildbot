package main

// This program is a Golang port of bloaty_treemap.py[1]. It reads the output of Bloaty[2] via stdin
// and produces an HTML page with a treemap view of the data produced by Bloaty.
//
// Sample use:
//
//     $ bloaty <path/to/binary> -d compileunits,symbols -n 0 --tsv | bloaty_treemap > index.html
//
// [1] https://skia.googlesource.com/skia/+/5a7d91c35beb48afce9362852f1f5e26f7550ba8/tools/bloaty_treemap.py
// [2] https://github.com/google/bloaty

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
	"text/template"

	"go.skia.org/infra/codesize/go/bloaty"
)

//go:embed template.html
var templateHTML string

func main() {
	stdin, err := os.ReadFile(os.Stdin.Name())
	ifErrThenDie(err)

	bloatyOutputItems, err := bloaty.ParseTSVOutput(string(stdin))
	ifErrThenDie(err)

	tmpl, err := template.New("template").Parse(templateHTML) // The template name does not matter.
	ifErrThenDie(err)

	var jsArrayRows []string
	for _, row := range bloaty.GenTreeMapDataTableRows(bloatyOutputItems, 200) {
		name := strings.ReplaceAll(row.Name, "'", "\\'")
		parent := "null"
		if row.Parent != "" {
			parent = fmt.Sprintf("'%s'", row.Parent)
		}
		jsArrayRows = append(jsArrayRows, fmt.Sprintf("['%s', %s, %d],", name, parent, row.Size))
	}

	data := map[string]string{
		"rows": strings.Join(jsArrayRows, "\n"),
	}

	err = tmpl.Execute(os.Stdout, data)
	ifErrThenDie(err)
}

func ifErrThenDie(err error) {
	if err != nil {
		panic(err)
	}
}
