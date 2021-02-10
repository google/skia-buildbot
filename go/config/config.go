package config

import (
	"encoding"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/flynn/json5"
	"go.skia.org/infra/go/skerr"
)

// Duration is a simple struct wrapper to allow us to parse strings as durations from the incoming
// config file (e.g,. RunEvery = "5m").
type Duration struct {
	time.Duration
}

// MarshalText implements the encoding.TextMarshaler interface.
func (d *Duration) MarshalText() (text []byte, err error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (d *Duration) UnmarshalText(text []byte) error {
	val, err := time.ParseDuration(string(text))
	if err != nil {
		return skerr.Wrapf(err, "invalid duration: %s", string(text))
	}
	d.Duration = val
	return nil
}

// Verify that Duration implements encoding.TextMarshaler and encoding.TextUnmarshaler.
var _ encoding.TextMarshaler = (*Duration)(nil)
var _ encoding.TextUnmarshaler = (*Duration)(nil)

// ParseConfigFile reads path as JSON5 into out. If non-empty, errors will reference the given
// flagName.
func ParseConfigFile(path, flagName string, out interface{}) error {
	if flagName != "" {
		flagName = flagName + " "
	}
	if data, err := ioutil.ReadFile(path); err != nil {
		return fmt.Errorf("Unable to read %sfile %q: %s", flagName, path, err)
	} else if err := json5.Unmarshal(data, out); err != nil {
		return fmt.Errorf("Unable to parse %sfile %q: %s", flagName, path, err)
	}
	return nil
}
