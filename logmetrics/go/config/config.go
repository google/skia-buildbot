// config is for reading the toml configuration file.
package config

import (
	"fmt"

	"go.skia.org/infra/go/config"
)

// Metric is used to parse the entries in metrics.json5 files.
type Metric struct {
	// Name is the measurement name.
	Name string

	// Filter is the Google Logging V2 query string to execute.
	Filter string
}

// ReadMetrics loads the config file at the given location.
func ReadMetrics(filename string) ([]Metric, error) {
	var m struct {
		Metrics []Metric
	}
	if err := config.ParseConfigFile(filename, "", &m); err != nil {
		return nil, fmt.Errorf("Failed to decode metrics file %q: %s", filename, err)
	}
	if len(m.Metrics) == 0 {
		return nil, fmt.Errorf("Didn't find any metrics in the file %q", filename)
	}
	return m.Metrics, nil
}
