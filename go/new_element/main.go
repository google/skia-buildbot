// A command-line application to create a new custom element of the given name in the directory 'modules'.
package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"text/template"

	"go.skia.org/infra/go/util"
)

const (
	modulesDirectory = "modules"
)

func exists(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	}
	return true
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf(`Usage: new-element name-of-element

Creates	a new custom element of the given name in the directory:

  modules/name-of-element

Will not overwrite existing files.
`)
	}

	// Locate and parse the templates.
	_, filename, _, _ := runtime.Caller(0)
	templateDir := filepath.Join(filepath.Dir(filename), "templates")
	templates := template.Must(template.ParseGlob(filepath.Join(templateDir, "*")))

	// Make sure 'modules' directory exists.
	if _, err := os.Stat(modulesDirectory); err != nil && os.IsNotExist(err) {
		log.Fatalf("The %q directory doesn't exist.", modulesDirectory)
	}
	name := os.Args[1]

	// Create directory for the element. Will fail if the directory already exists.
	if err := os.Mkdir(filepath.Join(modulesDirectory, name), 0755); err != nil {
		log.Fatalf("Failed to create element directory: %s", err)
	}

	context := map[string]string{
		"ElementName": name,
	}

	files := map[string]string{
		"file-demo.html": filepath.Join(modulesDirectory, name, name+"-demo.html"),
		"file-demo.js":   filepath.Join(modulesDirectory, name, name+"-demo.js"),
		"file.js":        filepath.Join(modulesDirectory, name, name+".js"),
		"file.scss":      filepath.Join(modulesDirectory, name, name+".scss"),
		"index.js":       filepath.Join(modulesDirectory, name, "index.js"),
	}

	// Write each file.
	for key, target := range files {
		err := util.WithWriteFile(target, func(w io.Writer) error {
			return templates.ExecuteTemplate(w, key, context)
		})
		if err != nil {
			log.Fatalf("Failed to write %q: %s", target, err)
		}
	}
}
