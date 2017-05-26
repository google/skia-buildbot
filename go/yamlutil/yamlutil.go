package yamlutil

import (
	"io/ioutil"

	"go.skia.org/infra/go/sklog"
	yaml "gopkg.in/yaml.v2"
)

// MustParseYamlFile reads path as YAML into out. If an error occurs, logs a fatal error referencing
// the given flagName.
func MustParseYamlFile(path, flagName string, out interface{}) {
	if data, err := ioutil.ReadFile(path); err != nil {
		sklog.Fatalf("Unable to read %s file %q: %s", flagName, path, err)
	} else if err := yaml.Unmarshal(data, out); err != nil {
		sklog.Fatalf("Unable to parse %s file %q: %s", flagName, path, err)
	}
}
