package convert

import "cloud.google.com/go/storage"

// Config for the conversion process.
type Config struct {
	InputFile string
	OutputDir string
	Master    string
	GCSClient *storage.Client
	GCSBucket string
	Date      string
}

// FuchsiaPerfResultItem represents a single performance result from the input.
type FuchsiaPerfResultItem struct {
	TestSuite string  `json:"test_suite"`
	TestName  string  `json:"test_name"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
}

// FuchsiaPerfResults represents the structure of the input Fuchsia JSON file.
type FuchsiaPerfResults []struct {
	BuildID     string                  `json:"build_id"`
	Builder     string                  `json:"builder"`
	CommitID    string                  `json:"commit_id"`
	PerfResults []FuchsiaPerfResultItem `json:"perf_results"`
}

// SkiaPerfResult represents the structure of the output Skia Perf JSON file.
type SkiaPerfResult struct {
	Version int               `json:"version"`
	GitHash string            `json:"git_hash"`
	Key     map[string]string `json:"key"`
	Results []SkiaResultItem  `json:"results"`
	Links   map[string]string `json:"links"`
}

// SkiaResultItem represents a single item in the results array.
type SkiaResultItem struct {
	Key          SkiaResultKey `json:"key"`
	Measurements Measurements  `json:"measurements"`
}

// SkiaResultKey represents the key for a single result item.
type SkiaResultKey struct {
	ImprovementDirection string `json:"improvement_direction"`
	Test                 string `json:"test"`
	Unit                 string `json:"unit"`
}

// Measurements represents the measurements for a single result item.
type Measurements struct {
	Stat []StatItem `json:"stat"`
}

// StatItem represents a single key-value pair in the stat array.
type StatItem struct {
	Value       string  `json:"value"`
	Measurement float64 `json:"measurement"`
}
