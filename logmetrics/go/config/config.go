// config is for reading the toml configuration file.
package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

// Metric is used to parse the toml entries in metrics.cfg files.
type Metric struct {
	// Name is the measurement name.
	Name string

	// Filter is the Google Logging V2 query string to execute.
	Filter string
}

// ReadMetrics loads the toml file at the given location.
func ReadMetrics(filename string) ([]Metric, error) {
	var m struct {
		Metrics []Metric
	}
	_, err := toml.DecodeFile(filename, &m)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode metrics file %q: %s", filename, err)
	}
	if len(m.Metrics) == 0 {
		return nil, fmt.Errorf("Didn't find any metrics in the file %q", filename)
	}
	return m.Metrics, nil
}
