package kube_conf_gen_lib

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
	"reflect"
	"text/template"

	"github.com/Masterminds/sprig"
	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// ParseConfigHelper takes the given config map and adds its values into the
// given return map in stringified form.  If strict is true, it will return an
// error for unsupported types, missing data, etc.
func ParseConfigHelper(confMap map[string]interface{}, ret map[string]interface{}, strict bool) error {
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
			if err := ParseConfigHelper(t, subMap, strict); err != nil {
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

// LoadConfigFiles reads the given JSON5-formatted config files and merges them
// into a single map[string]interface.  If parseConf is true, all config values
// are converted to strings.  If strict is true, will return an error for
// unsupported types, missing data, etc.  If emptyQuotes is true, config values
// which are empty strings are replaced with empty single quotes ('').
func LoadConfigFiles(parseConf, strict, emptyQuotes bool, configFileNames ...string) (map[string]interface{}, error) {
	ret := map[string]interface{}{}
	for _, configFile := range configFileNames {
		confMap := map[string]interface{}{}
		if err := config.ParseConfigFile(configFile, "-c", &confMap); err != nil {
			return nil, fmt.Errorf("Failed to parse config file %q: %s", configFile, err)
		}
		if parseConf {
			if err := ParseConfigHelper(confMap, ret, strict); err != nil {
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

// GenerateOutput executes the template with config as its environment and
// writes the result to outFile.
func GenerateOutput(templateFileName string, strict bool, config map[string]interface{}, outFile string) error {
	tmpl, err := template.New(path.Base(templateFileName)).Funcs(sprig.TxtFuncMap()).ParseFiles(templateFileName)
	if err != nil {
		return skerr.Wrapf(err, "error parsing template '%s'. Error:%s", templateFileName, err)
	}
	if strict {
		tmpl.Option("missingkey=error")
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return skerr.Wrap(err)
	}

	if outFile == "_" {
		fmt.Println(string(buf.Bytes()))
		return nil
	} else {
		return skerr.Wrap(ioutil.WriteFile(outFile, buf.Bytes(), 0644))
	}
}
