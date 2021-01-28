package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"path"
	"reflect"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/sklog"
)

func main() {
	var (
		// Define the flags and then parse them in common.Init.
		templateFileName = flag.String("t", "", "Template file name.")
		configFileNames  = common.NewMultiStringFlag("c", nil, "Config file name")
		outputFileName   = flag.String("o", "", "Output file name. Use \"-o _\" to write to stdout.")
		emptyQuotes      = flag.Bool("quote", false, "Replace config values that are empty strings with empty single quotes ('').")
		parseConf        = flag.Bool("parse_conf", true, "Convert config options to string.")
		extraVars        = common.NewMultiStringFlag("extra", nil, "Extra key value pair(s), separated by a colon, eg. \"key:value\"")
		strict           = flag.Bool("strict", false, "If true, error out for unsupported types, missing data, etc.")
	)
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

	// Assemble the config map.
	config, err := loadConfigFiles(*parseConf, *strict, *emptyQuotes, *configFileNames...)
	if err != nil {
		sklog.Fatalf("Error loading config files: %s", err)
	}
	for k, v := range extraVarsMap {
		config[k] = v
	}

	// Generate the output.
	tmpl, err := template.New(path.Base(*templateFileName)).Funcs(sprig.TxtFuncMap()).ParseFiles(*templateFileName)
	if err != nil {
		sklog.Fatalf("Error parsing template '%s'. Error:%s", *templateFileName, err)
	}
	if *strict {
		tmpl.Option("missingkey=error")
	}

	if err := generateOutput(tmpl, config, *outputFileName); err != nil {
		sklog.Fatalf("Error: %s", err)
	}
}

// generateOutput executes the template with config as its environment and writes the result to outFile.
func generateOutput(tmpl *template.Template, config map[string]interface{}, outFile string) error {
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

func parseConfigHelper(confMap map[string]interface{}, ret map[string]interface{}, strict bool) error {
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
		case int, int32, int64:
			val = fmt.Sprintf("%d", v)
		case float32, float64:
			val = fmt.Sprintf("%f", v)
		case []interface{}:
			ret[k] = t
		case map[string]interface{}:
			subMap := map[string]interface{}{}
			if err := parseConfigHelper(t, subMap, strict); err != nil {
				return err
			}
			ret[k] = subMap
		default:
			if strict {
				return fmt.Errorf("Key %q has unsupported type %q", k, t)
			}
			reflectVal := reflect.ValueOf(v)
			if !reflectVal.IsValid() {
				sklog.Warningf("Key %q has unsupported type %q", k, t)
			} else {
				sklog.Warningf("Key %q has unsupported type %q", k, reflectVal.Type().String())
			}
		}
		if val != "" {
			ret[k] = val
		}
	}
	return nil
}

func loadConfigFiles(parseConf, strict, emptyQuotes bool, configFileNames ...string) (map[string]interface{}, error) {
	ret := map[string]interface{}{}
	for _, configFile := range configFileNames {
		confMap := map[string]interface{}{}
		if err := config.ParseConfigFile(configFile, "-c", &confMap); err != nil {
			return nil, fmt.Errorf("Failed to parse config file %q: %s", configFile, err)
		}
		if parseConf {
			if err := parseConfigHelper(confMap, ret, strict); err != nil {
				return nil, fmt.Errorf("Failed to parse config file %q: %s", configFile, err)
			}
		} else {
			for k, v := range confMap {
				ret[k] = v
			}
		}
	}

	// Go through the result and replace empty strings with empty single quotes if requested.
	if emptyQuotes {
		for k, v := range ret {
			if strVal, ok := v.(string); ok && strVal == "" {
				ret[k] = "''"
			}
		}
	}

	return ret, nil
}
