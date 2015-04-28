package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/generator"
	"go.skia.org/infra/go/util"
)

import (
	_ "go.skia.org/infra/fuzzer/go/generator/dummy"
)

var (
	configFilename                    = flag.String("config", "fuzzer.toml", "Configuration filename")
	codeTemplate   *template.Template = nil
	gypTemplate    *template.Template = nil
)

func setDefaults() {
	config.Config.Fuzzer.Indentation = 2
}

// setup does some app-wide initialization, initia and returns the path to the
// resources directory
func setup() (string, error) {
	if config.Config.Common.ResourcePath == "" {
		_, filename, _, _ := runtime.Caller(0)
		config.Config.Common.ResourcePath = filepath.Join(filepath.Dir(filename), "../..")
	}

	path, err := filepath.Abs(config.Config.Common.ResourcePath)
	if err != nil {
		return path, fmt.Errorf("Couldn't get abs path for %s: %s", config.Config.Common.ResourcePath, err)

	}
	if err := os.Chdir(path); err != nil {
		return path, fmt.Errorf("Couldn't change to directory %s: %s", path, err)

	}
	gypTemplate = template.Must(template.ParseFiles(filepath.Join(path, "templates/template.gyp")))
	return path, nil
}

// writeTemplate creates a given output file and writes the template
// result there.
func writeTemplate(filename string, t *template.Template, context interface{}) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Couldn't create file %s: %s", filename, err)
	}
	defer util.Close(f)
	return t.Execute(f, context)
}

func writeFuzz(code string) (string, error) {
	h := md5.New()
	h.Write([]byte(code))
	hash := fmt.Sprintf("%x", h.Sum(nil))
	err := writeTemplate(fmt.Sprintf(filepath.Join(config.Config.Fuzzer.CachePath, "src/%s.cpp"), hash),
		codeTemplate,
		struct{ Code string }{code})

	if err != nil {
		return hash, fmt.Errorf("Coudln't write CPP template: %s", err)
	}

	err = writeTemplate(fmt.Sprintf(filepath.Join(config.Config.Fuzzer.CachePath, "%s.gyp"), hash),
		gypTemplate,
		struct{ Hash string }{hash})

	if err != nil {
		return hash, fmt.Errorf("Coudln't write GYP template: %s", err)
	}

	return hash, err
}

// createCodeTemplate builds the .cpp template that we will build each fuzz into.  We
// search the Skia source directory for any public include files and add them to the output.
func createCodeTemplate(outputPath string) {
	includeDirs := []string{"core", "effects", "pathops"}
	includeFiles := []string{}

	for _, dir := range includeDirs {
		includePath := filepath.Join(config.Config.Fuzzer.SkiaSourceDir, "include", dir)
		infos, err := ioutil.ReadDir(includePath)
		if err != nil {
			glog.Fatalf("Couldn't read include dir: %s", err)
		}
		for _, info := range infos {
			includeFiles = append(includeFiles, info.Name())
		}
	}

	sort.Strings(includeFiles)

	out, err := os.Create(outputPath)
	if err != nil {
		glog.Fatalf("Couldn't create code template: %s", err)
	}
	defer util.Close(out)

	for _, filename := range includeFiles {
		out.WriteString(fmt.Sprintf("#include \"%s\"\n", filename))
	}
	out.WriteString(`#include "sk_tool_utils.h"
#include "SkCommandLineFlags.h"

SkBitmap source;
void draw(SkCanvas* canvas) {
{{.Code}}
}
`)
}

// checkCPPTemplate checks for the existence of the CPP template that each fuzz will be
// build against, and creates it if it's not there.
func checkCPPTemplate(path string) {
	templatePath := filepath.Join(path, "templates/template.cpp")

	if _, err := os.Stat(templatePath); err != nil {
		createCodeTemplate(templatePath)
	}
	codeTemplate = template.Must(template.ParseFiles(templatePath))
}

func main() {
	flag.Parse()

	setDefaults()

	if _, err := toml.DecodeFile(*configFilename, &config.Config); err != nil {
		glog.Fatalf("Failed to decode config file: %s", err)
	}

	resourcePath, err := setup()
	if err != nil {
		glog.Fatalf("Couldn't setup: %s", err)
	}

	checkCPPTemplate(resourcePath)

	for {
		fuzz, err := generator.Fuzz()
		if err != nil {
			glog.Fatalf("Couldn't create a fuzz: %s", err)
		}

		_, err = writeFuzz(fuzz)
		if err != nil {
			glog.Fatalf("Couldn't create the fuzz hash: %s", err)
		}

	}
}
