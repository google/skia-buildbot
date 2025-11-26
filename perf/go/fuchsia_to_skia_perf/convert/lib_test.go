package convert

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"go.skia.org/infra/go/deepequal/assertdeep"
)

func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "input.json")
	err := os.WriteFile(tempFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	return tempFile
}

func TestRun_FileErrors(t *testing.T) {
	cfg := Config{
		InputFile: "nonexistent.json",
		OutputDir: "output_dir/",
		Master:    "test-master",
	}
	err := Run(cfg)
	if err == nil {
		t.Errorf("Expected an error for non-existent input file, got nil")
	}
}

func TestRun_MissingMaster(t *testing.T) {
	cfg := Config{
		InputFile: "input.json",
		OutputDir: "output_dir/",
	}
	err := Run(cfg)
	if err == nil {
		t.Errorf("Expected an error for missing master, got nil")
	}
}

func TestRun_JSONUnmarshalErrors(t *testing.T) {
	testCases := []struct {
		name    string
		content string
	}{
		{"EmptyFile", ""},
		{"InvalidJSON", "[{"},
		{"NullFields", `[{"build_id": null, "builder": "test", "commit_id": "123", "perf_results": null}]`},
		{"MissingFields", `[{"builder": "test"}]`},
		{"TypeMismatch", `[{"build_id": 123}]`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputFile := createTempFile(t, tc.content)
			cfg := Config{
				InputFile: inputFile,
				OutputDir: "output_dir/",
				Master:    "test-master",
			}
			err := Run(cfg)
			if err == nil {
				t.Errorf("Expected an error for invalid JSON content: %s, got nil", tc.content)
			}
		})
	}
}

func TestMapUnitAndDirection(t *testing.T) {
	testCases := []struct {
		input    string
		wantUnit string
		wantDir  string
	}{
		{"nanoseconds", "ms", "smallerIsBetter"},
		{"ns", "ms", "smallerIsBetter"},
		{"milliseconds", "ms", "smallerIsBetter"},
		{"bytes", "sizeInBytes", "smallerIsBetter"},
		{"frames/second", "Hz", "biggerIsBetter"},
		{"percent", "n%", "smallerIsBetter"},
		{"bytes/second", "unitless", "biggerIsBetter"},
		{"bits/second", "unitless", "biggerIsBetter"},
		{"ms", "ms", "smallerIsBetter"},
		{"sizeInBytes", "sizeInBytes", "smallerIsBetter"},
		{"count", "count", "smallerIsBetter"},
		{"foobar", "unitless", "biggerIsBetter"},
		{"ms_biggerIsBetter", "ms", "biggerIsBetter"},
		{"bytes_smallerIsBetter", "sizeInBytes", "smallerIsBetter"},
		{"percent_invalidDirection", "n%", "smallerIsBetter"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			gotUnit, gotDir := MapUnitAndDirection(tc.input)
			if gotUnit != tc.wantUnit || gotDir != tc.wantDir {
				t.Errorf("mapUnitAndDirection(%q) = %q, %q; want %q, %q", tc.input, gotUnit, gotDir, tc.wantUnit, tc.wantDir)
			}
		})
	}
}

func compareStatItems(t *testing.T, got, want []StatItem) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Mismatched stat item counts: got %d, want %d", len(got), len(want))
		return
	}
	for i := range got {
		if got[i].Value != want[i].Value {
			t.Errorf("Mismatched Value at index %d: got %s, want %s", i, got[i].Value, want[i].Value)
		}
		if math.Abs(got[i].Measurement-want[i].Measurement) > 1e-9 {
			t.Errorf("Mismatched Measurement at index %d: got %f, want %f", i, got[i].Measurement, want[i].Measurement)
		}
	}
}

func TestCalculateStats(t *testing.T) {
	testCases := []struct {
		name      string
		results   []FuchsiaPerfResultItem
		wantStats []StatItem
		wantAvg   float64
		wantStd   float64
	}{
		{
			name:      "Empty",
			results:   []FuchsiaPerfResultItem{},
			wantStats: nil,
			wantAvg:   0.0,
			wantStd:   0.0,
		},
		{
			name: "SingleValue",
			results: []FuchsiaPerfResultItem{
				{Unit: "ms", Value: 100},
			},
			wantStats: []StatItem{
				{Value: "value", Measurement: 100},
				{Value: "error", Measurement: 0},
				{Value: "count", Measurement: 1},
				{Value: "max", Measurement: 100},
				{Value: "min", Measurement: 100},
				{Value: "sum", Measurement: 100},
			},
			wantAvg: 100,
			wantStd: 0,
		},
		{
			name: "MultipleValues",
			results: []FuchsiaPerfResultItem{
				{Unit: "percent", Value: 10},
				{Unit: "percent", Value: 20},
				{Unit: "percent", Value: 30},
			},
			wantStats: []StatItem{
				{Value: "value", Measurement: 10},
				{Value: "error", Measurement: 10},
				{Value: "count", Measurement: 3},
				{Value: "max", Measurement: 30},
				{Value: "min", Measurement: 10},
				{Value: "sum", Measurement: 60},
			},
			wantAvg: 20,
			wantStd: 10,
		},
		{
			name: "NanosecondsConversion",
			results: []FuchsiaPerfResultItem{
				{Unit: "nanoseconds", Value: 2000000},
				{Unit: "nanoseconds", Value: 4000000},
			},
			wantStats: []StatItem{
				{Value: "value", Measurement: 2},
				{Value: "error", Measurement: 1.414213562373095},
				{Value: "count", Measurement: 2},
				{Value: "max", Measurement: 4},
				{Value: "min", Measurement: 2},
				{Value: "sum", Measurement: 6},
			},
			wantAvg: 3,
			wantStd: 1.414213562373095,
		},
		{
			name: "BytesPerSecondConversion",
			results: []FuchsiaPerfResultItem{
				{Unit: "bytes/second", Value: 2097152}, // 2 MiB
			},
			wantStats: []StatItem{
				{Value: "value", Measurement: 2},
				{Value: "error", Measurement: 0},
				{Value: "count", Measurement: 1},
				{Value: "max", Measurement: 2},
				{Value: "min", Measurement: 2},
				{Value: "sum", Measurement: 2},
			},
			wantAvg: 2,
			wantStd: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotStats, gotAvg, gotStd := CalculateStats(tc.results)
			if tc.wantStats == nil && gotStats != nil {
				t.Errorf("calculateStats() stats = %v, want nil", gotStats)
			} else if tc.wantStats != nil && gotStats == nil {
				t.Errorf("calculateStats() stats = nil, want %v", tc.wantStats)
			} else if tc.wantStats != nil && gotStats != nil {
				compareStatItems(t, gotStats, tc.wantStats)
			}
			if math.Abs(gotAvg-tc.wantAvg) > 1e-9 {
				t.Errorf("calculateStats() avg = %v, want %v", gotAvg, tc.wantAvg)
			}
			if math.Abs(gotStd-tc.wantStd) > 1e-9 { // Compare floats with tolerance
				t.Errorf("calculateStats() std = %v, want %v", gotStd, tc.wantStd)
			}
		})
	}
}

func TestPopulateResults(t *testing.T) {
	input := []FuchsiaPerfResultItem{
		{TestSuite: "S1", TestName: "T1", Value: 10, Unit: "ms"},
		{TestSuite: "S1", TestName: "T1", Value: 20, Unit: "ms"},
		{TestSuite: "S1", TestName: "T2", Value: 100, Unit: "bytes"},
	}

	expectedKeys := map[string]bool{
		"T1":     true,
		"T1_avg": true,
		"T2":     true,
		"T2_avg": true,
	}

	got := PopulateResults(input)

	if len(got) != len(expectedKeys) {
		t.Fatalf("populateResults() returned %d items, want %d", len(got), len(expectedKeys))
	}

	foundKeys := make(map[string]bool)
	for _, item := range got {
		foundKeys[item.Key.Test] = true
		if item.Key.Unit == "" {
			t.Errorf("populateResults() item %s has empty Unit", item.Key.Test)
		}
		if item.Key.ImprovementDirection == "" {
			t.Errorf("populateResults() item %s has empty ImprovementDirection", item.Key.Test)
		}
		if len(item.Measurements.Stat) == 0 {
			t.Errorf("populateResults() item %s has empty Measurements.Stat", item.Key.Test)
		}
	}

	assertdeep.Equal(t, foundKeys, expectedKeys)
}

// TODO(eduardoyap): Add more comprehensive end-to-end tests for Run function
func TestRun_ValidJSON(t *testing.T) {
	testCases := []struct {
		name    string
		content string
	}{
		{
			name: "ValidJSON",
			content: `
[
  {
    "build_id": "8698222556732727921",
    "builder": "pixel_watch.arm64-release",
    "commit_id": "fce52a75c28903e6bbe1a079eec7097db8d89c3f",
    "perf_results": [
      {
        "test_suite": "fuchsia.wearos.app_launch.complex_layout",
        "test_name": "AtmDisplayTimeAverage",
        "value": 2150,
        "unit": "milliseconds"
      }
    ]
  }
]`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inputFile := createTempFile(t, tc.content)
			tempDir := t.TempDir() // Use a fresh temp dir for output
			outputDir := tempDir

			cfg := Config{
				InputFile: inputFile,
				OutputDir: outputDir,
				Master:    "test-master",
			}
			err := Run(cfg)
			if err != nil {
				t.Errorf("Run failed for valid JSON: %v", err)
			}
			// Check if output file was created
			expectedOutputFileName := "8698222556732727921-fuchsia.wearos.app_launch.complex_layout-pixel_watch.arm64-release-test-master.json"
			expectedOutputPath := filepath.Join(outputDir, expectedOutputFileName)
			if _, err := os.Stat(expectedOutputPath); os.IsNotExist(err) {
				t.Errorf("Expected output file %s was not created", expectedOutputPath)
			} else {
				// Basic content check
				data, err := os.ReadFile(expectedOutputPath)
				if err != nil {
					t.Fatalf("Failed to read output file: %v", err)
				}
				var result SkiaPerfResult
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatalf("Failed to unmarshal output file: %v", err)
				}
				if result.Key["benchmark"] != "fuchsia.wearos.app_launch.complex_layout" {
					t.Errorf("Output file has incorrect benchmark key: %s", result.Key["benchmark"])
				}
				if len(result.Results) != 2 { // Base + Avg
					t.Errorf("Output file should have 2 results, got %d", len(result.Results))
				}
			}
		})
	}
}
