package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"text/template"

	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
)

func main() {
	common.Init()

	tmplFileName := flag.Arg(0)
	configFileName := flag.Arg(1)
	outputFileName := flag.Arg(2)

	tmpl, err := template.ParseFiles(tmplFileName)
	if err != nil {
		sklog.Fatalf("Unable to parse template: %s", err)
	}

	extractVals := true
	if extractVals {
		config := map[string]string{}
		extractFn := func(key string) string {
			sklog.Infof("Called with %s", key)
			config[key] = ""
			return ""
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, extractFn); err != nil {
			sklog.Fatalf("Error executing template: %s", err)
		}

		configBytes, err := json.Marshal(config)
		if err != nil {
			sklog.Fatalf("Error generating JSON: %s", err)
		}
		if err := ioutil.WriteFile(configFileName, configBytes, 0644); err != nil {
			sklog.Fatalf("Error writing config: %s", err)
		}
	} else {
		confFileContent, err := ioutil.ReadFile(configFileName)
		if err != nil {
			sklog.Fatalf("Error: %s", err)
		}
		config := map[string]string{}
		if err := json.Unmarshal(confFileContent, &config); err != nil {
			sklog.Fatalf("Error: %s", err)
		}

		sklog.Infof("Config: %s", spew.Sdump(config))

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, config); err != nil {
			sklog.Fatalf("Error: %s", err)
		}

		if err := ioutil.WriteFile(outputFileName, buf.Bytes(), 0644); err != nil {
			sklog.Fatalf("Error: %s", err)
		}
	}
}
