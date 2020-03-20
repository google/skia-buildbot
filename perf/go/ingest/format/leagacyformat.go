// Package format is the format for ingestion files.
package format

import (
	"encoding/json"
	"fmt"
	"io"
)

// BenchResult represents a single test result.
//
// Used in BenchData.
//
// Expected to be a map of strings to float64s, with the
// exception of the "options" entry which should be a
// map[string]string.
type BenchResult map[string]interface{}

// BenchResults is the dictionary of individual BenchResult structs.
//
// Used in BenchData.
type BenchResults map[string]BenchResult

// BenchData is the top level struct for decoding the nanobench JSON format. All
// other ingestion should use the FileFormat format.
type BenchData struct {
	Hash         string                  `json:"gitHash"`
	Issue        string                  `json:"issue"`
	PatchSet     string                  `json:"patchset"`
	Source       string                  `json:"source"` // Where the data came from.
	Key          map[string]string       `json:"key"`
	Options      map[string]string       `json:"options"`
	Results      map[string]BenchResults `json:"results"`
	PatchStorage string                  `json:"patch_storage"`
}

// ParseBenchDataFromReader parses the stream out of the io.Reader into
// BenchData. The caller is responsible for calling Close on the reader.
func ParseLegacyFormat(r io.Reader) (*BenchData, error) {
	var benchData BenchData
	if err := json.NewDecoder(r).Decode(&benchData); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON: %s", err)
	}
	return &benchData, nil
}
