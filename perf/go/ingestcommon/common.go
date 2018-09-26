package ingestcommon

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

// BenchData is the top level struct for decoding the nanobench JSON format.
type BenchData struct {
	Hash         string                  `json:"gitHash"`
	Issue        string                  `json:"issue"`
	PatchSet     string                  `json:"patchset"`
	Key          map[string]string       `json:"key"`
	Options      map[string]string       `json:"options"`
	Results      map[string]BenchResults `json:"results"`
	PatchStorage string                  `json:"patch_storage"`
}

// TODO(stephana): Remove isGerritIssue once we switch to Gerrit.

// IsGerritIssue returns true if the issue comes from an instance of the Gerrit
// code review system.
func (b *BenchData) IsGerritIssue() bool {
	return (b.Issue != "") && (b.PatchStorage == "gerrit")
}

// parseBenchDataFromReader parses the stream out of the io.Reader into
// BenchData. The caller is responsible for calling Close on the reader.
func ParseBenchDataFromReader(r io.Reader) (*BenchData, error) {
	dec := json.NewDecoder(r)
	benchData := &BenchData{}
	if err := dec.Decode(benchData); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON: %s", err)
	}
	return benchData, nil
}
