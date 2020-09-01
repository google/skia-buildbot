// A command-line application to create a new custom element of the given name in the directory 'modules'.
package main

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	rice "github.com/GeertJohan/go.rice"
	"go.skia.org/infra/go/skerr"
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

// new returns an http.FileSystem with all the templates.
func new() (http.FileSystem, error) {
	conf := rice.Config{
		LocateOrder: []rice.LocateMethod{rice.LocateFS, rice.LocateEmbedded},
	}
	box, err := conf.FindBox("templates")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return box.HTTPBox(), nil
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf(`Usage: new-element name-of-element

Creates	a new custom element of the given name in the directory:

  modules/name-of-element

Will not overwrite existing files.
`)
	}

	fs, err := new()
	if err != nil {
		log.Fatal(err)
	}

	// Make sure 'modules' directory exists.
	if _, err := os.Stat(modulesDirectory); err != nil && os.IsNotExist(err) {
		log.Fatalf("The %q directory doesn't exist.", modulesDirectory)
	}
	name := os.Args[1]

	files := map[string]string{
		"file-demo.html":         filepath.Join(modulesDirectory, name, name+"-demo.html"),
		"file-demo.ts":           filepath.Join(modulesDirectory, name, name+"-demo.ts"),
		"file.ts":                filepath.Join(modulesDirectory, name, name+".ts"),
		"file_test.ts":           filepath.Join(modulesDirectory, name, name+"_test.ts"),
		"file_puppeteer_test.ts": filepath.Join(modulesDirectory, name, name+"_puppeteer_test.ts"),
		"file.scss":              filepath.Join(modulesDirectory, name, name+".scss"),
		"index.ts":               filepath.Join(modulesDirectory, name, "index.ts"),
	}

	// Convert the element name, "some-element-sk", into a class name, "SomeElementSk".
	parts := strings.Split(name, "-")
	for i, part := range parts {
		parts[i] = strings.Title(part)
	}
	className := strings.Join(parts, "")

	// Create directory for the element. Will fail if the directory already exists.
	if err := os.Mkdir(filepath.Join(modulesDirectory, name), 0755); err != nil {
		log.Fatalf("Failed to create element directory: %s", err)
	}

	context := map[string]string{
		"ElementName": name,
		"ClassName":   className,
	}

	// Write each file.
	for filename, target := range files {
		// Load and parse the template.
		file, err := fs.Open(filename)
		if err != nil {
			log.Fatal(err)
		}
		b, err := ioutil.ReadAll(file)
		if err != nil {
			log.Fatal(err)
		}
		t := template.New(filename)
		if _, err := t.Parse(string(b)); err != nil {
			log.Fatal(err)
		}

		// Use the parsed template to create the file.
		err = util.WithWriteFile(target, func(w io.Writer) error {
			return t.ExecuteTemplate(w, filename, context)
		})
		if err != nil {
			log.Fatalf("Failed to write %q: %s", target, err)
		}
	}
}
