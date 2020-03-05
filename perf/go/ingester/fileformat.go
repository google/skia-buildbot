// Package ingester provides an interface that all sources of raw Perf data to
// be ingested must implement.
package ingester

const FileFormatVersion = 1

type Result struct {
	Key   map[string]string
	Value float64
}

// FileFormat is the
type FileFormat struct {
	Version int               `json:"version"` // Should be 1 for this format.
	Hash    string            `json:"hash"`    // The Git hash of the repo when these tests were run.
	Common  map[string]string `json:"common"`
	Results []Result          `json:"results"`
	Links   map[string]string `json:"links"`
}
