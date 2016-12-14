// Parser parses incoming JSON files from Android Testing and converts them
// into a format acceptable to Skia Perf.
package parser

import (
	"encoding/json"
	"fmt"
	"io"
)

type Incoming struct {
	BuildId     string             `json:"build_id"`
	BuildFlavor string             `json:"build_flavor"`
	Branch      string             `json:"branch"`
	Metrics     map[string]Results `json:"metrics"`
}

type Results map[string]string

func Parse(incoming io.Reader) (*Incoming, error) {
	ret := &Incoming{}
	if err := json.NewDecoder(incoming).Decode(ret); err != nil {
		return nil, fmt.Errorf("Failed to decode incoming JSON: %s", err)
	}
	return ret, nil
}
