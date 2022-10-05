// A command-line application to create a new custom element of the given name
// in the directory 'modules'.
//
//go:generate rm -rf modules/example-control-sk
//go:generate bazelisk run --config=mayberemote //:go -- run main.go --element-name example-control-sk --app-name example-app-name
//
package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"go.skia.org/infra/go/util"
)

const (
	modulesDirectory = "modules"
)

var (
	elementName = flag.String("element-name", "", `Element name, e.g. "my-element-sk".`)
	appName     = flag.String("app-name", "", `Application name, e.g. "perf".`)
	bazelOnly   = flag.Bool("bazel-only", false, "Only generate a BUILD.bazel file.")
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
	_, filename, _, _ := runtime.Caller(1)

	return http.Dir(filepath.Join(filepath.Dir(filename), "./templates")), nil
}

func main() {
	flag.Parse()

	if *elementName == "" || *appName == "" {
		log.Fatalf(`Usage: new_element --element-name <element name> --app-name <application name>

Creates	a new custom element of the given name in the directory:

  modules/<element name>

Will not overwrite existing files.

Pass flag --bazel-only to only generate the BUILD.bazel file for the custom element.
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

	files := map[string]string{
		"file-demo.html":         filepath.Join(modulesDirectory, *elementName, *elementName+"-demo.html"),
		"file-demo.ts":           filepath.Join(modulesDirectory, *elementName, *elementName+"-demo.ts"),
		"file-demo.scss":         filepath.Join(modulesDirectory, *elementName, *elementName+"-demo.scss"),
		"file.ts":                filepath.Join(modulesDirectory, *elementName, *elementName+".ts"),
		"file_test.ts":           filepath.Join(modulesDirectory, *elementName, *elementName+"_test.ts"),
		"file_puppeteer_test.ts": filepath.Join(modulesDirectory, *elementName, *elementName+"_puppeteer_test.ts"),
		"file.scss":              filepath.Join(modulesDirectory, *elementName, *elementName+".scss"),
		"index.ts":               filepath.Join(modulesDirectory, *elementName, "index.ts"),
		"BUILD.bazel.template":   filepath.Join(modulesDirectory, *elementName, "BUILD.bazel"),
	}

	// Convert the element name, "some-element-sk", into a class name, "SomeElementSk".
	parts := strings.Split(*elementName, "-")
	for i, part := range parts {
		parts[i] = strings.Title(part)
	}
	className := strings.Join(parts, "")

	// Create directory for the element. Will fail if the directory already exists. We skip this if
	// --bazel-only is passed because presumably we're trying to generate a BUILD.bazel file for an
	// element that already exists.
	if !*bazelOnly {
		if err := os.Mkdir(filepath.Join(modulesDirectory, *elementName), 0755); err != nil {
			log.Fatalf("Failed to create element directory: %s", err)
		}
	}

	context := map[string]string{
		"ElementName": *elementName,
		"ClassName":   className,
		"AppName":     *appName,
	}

	// Write each file.
	for filename, target := range files {
		if *bazelOnly && !strings.Contains(filename, "BUILD.bazel") {
			continue
		}

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
