package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"text/template"

	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/common"
)

// Command line flags.
var (
	templateFileName = flag.String("t", "", "Template file name.")
	configFileNames  = common.NewMultiStringFlag("c", nil, "Config file name")
	outputFileName   = flag.String("o", "", "Output file name. Use \"-o _\" to write to stdout.")
	extraVars        = common.NewMultiStringFlag("extra", nil, "Extra key value pair(s), separated by a colon, eg. \"key:value\"")
	strict           = flag.Bool("strict", false, "If true, error out for unsupported types, missing data, etc.")
)

func main() {
	common.Init()

	if *templateFileName == "" {
		sklog.Fatal("-t is required.")
	}
	if *outputFileName == "" {
		sklog.Fatal("-o is required.")
	}
	if len(*configFileNames) == 0 {
		sklog.Fatal("-c is required.")
	}
	extraVarsMap := map[string]string{}
	for _, pair := range *extraVars {
		split := strings.SplitN(pair, ":", 2)
		if len(split) != 2 {
			sklog.Fatalf("Invalid key/value pair for --extra: %q; should be \"key:value\"", pair)
		}
		extraVarsMap[split[0]] = split[1]
	}

	tmpl, err := template.ParseFiles(*templateFileName)
	if err != nil {
		sklog.Fatalf("Error parsing template '%s'. Error:%s", *templateFileName, err)
	}
	if *strict {
		tmpl.Option("missingkey=error")
	}

	if err := generateOutput(tmpl, *configFileNames, extraVarsMap, *outputFileName); err != nil {
		sklog.Fatalf("Error: %s", err)
	}
}

func generateOutput(tmpl *template.Template, configFileNames []string, extraVarsMap map[string]string, outFile string) error {
	config, err := loadConfigFiles(configFileNames...)
	if err != nil {
		return err
	}
	for k, v := range extraVarsMap {
		config[k] = v
	}

	sklog.Infof("Config: %s", spew.Sdump(config))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		sklog.Fatalf("Error: %s", err)
	}

	if outFile == "_" {
		fmt.Println(string(buf.Bytes()))
		return nil
	} else {
		return ioutil.WriteFile(outFile, buf.Bytes(), 0644)
	}
}

func parseConfigHelper(prefix string, confMap map[string]interface{}, ret map[string]string) error {
	for k, v := range confMap {
		val := ""
		switch t := v.(type) {
		case string:
			val = t
		case bool:
			if t {
				val = "true"
			} else {
				val = "false"
			}
		case map[string]interface{}:
			if err := parseConfigHelper(prefix+k+".", t, ret); err != nil {
				return err
			}
		default:
			if *strict {
				return fmt.Errorf("Key %q has unsupported type %q", k, t)
			} else {
				sklog.Warningf("Key %q has unsupported type %q", k, reflect.ValueOf(v).Type().String())
			}
		}
		if val != "" {
			ret[prefix+k] = val
		}
	}
	return nil
}

func loadConfigFiles(configFileNames ...string) (map[string]string, error) {
	ret := map[string]string{}
	for _, configFile := range configFileNames {
		confMap := map[string]interface{}{}
		if err := config.ParseConfigFile(configFile, "-c", &confMap); err != nil {
			return nil, fmt.Errorf("Failed to parse config file %q: %s", configFile, err)
		}
		if err := parseConfigHelper("", confMap, ret); err != nil {
			return nil, fmt.Errorf("Failed to parse config file %q: %s", configFile, err)
		}
	}
	return ret, nil
}
