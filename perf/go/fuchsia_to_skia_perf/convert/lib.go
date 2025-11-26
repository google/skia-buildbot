package convert

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

var (
	unitMap = map[string]string{
		"nanoseconds":   "ms",
		"ns":            "ms",
		"milliseconds":  "ms",
		"bytes":         "sizeInBytes",
		"frames/second": "Hz",
		"percent":       "n%",
		"bytes/second":  "unitless", // Was bytesPerSecond in C++ map key
		"bits/second":   "unitless",
		// Units that map to themselves
		"ms":              "ms",
		"msBestFitFormat": "msBestFitFormat",
		"tsMs":            "tsMs",
		"n%":              "n%",
		"sizeInBytes":     "sizeInBytes",
		"J":               "J",
		"W":               "W",
		"A":               "A",
		"Ah":              "Ah",
		"V":               "V",
		"Hz":              "Hz",
		"unitless":        "unitless",
		"count":           "count",
		"sigma":           "sigma",
	}

	defaultImprovementDirection = map[string]string{
		"unitless":    "biggerIsBetter", // covers bytes/second, bits/second
		"sizeInBytes": "smallerIsBetter",
		"J":           "smallerIsBetter",
		"W":           "smallerIsBetter",
		"A":           "smallerIsBetter",
		"V":           "smallerIsBetter",
		"Hz":          "biggerIsBetter", // covers frames/second
		"sigma":       "smallerIsBetter",
		"n%":          "smallerIsBetter", // covers percent
		"ms":          "smallerIsBetter", // covers nanoseconds, milliseconds
		"count":       "smallerIsBetter",
		// msBestFitFormat, tsMs, Ah not in the C++ defaults, will default to smallerIsBetter
	}
)

// stdDev calculates the standard deviation of a slice of float64.
func stdDev(values []float64) float64 {
	if len(values) <= 1 {
		return 0.0
	}

	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))

	variance := 0.0
	for _, v := range values {
		variance += math.Pow(v-mean, 2)
	}
	variance /= float64(len(values) - 1) // Sample standard deviation

	return math.Sqrt(variance)
}

// Run performs the JSON conversion.
func Run(cfg Config) error {
	if cfg.Master == "" {
		return fmt.Errorf("master is required")
	}
	fmt.Printf("Input file: %s\n", cfg.InputFile)
	fmt.Printf("Output directory: %s\n", cfg.OutputDir)
	fmt.Printf("Master: %s\n", cfg.Master)

	// Read the input file
	inputData, err := os.ReadFile(cfg.InputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Unmarshal the JSON data
	var fuchsiaResults FuchsiaPerfResults
	if err := json.Unmarshal(inputData, &fuchsiaResults); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	fmt.Printf("Successfully unmarshaled %d records\n", len(fuchsiaResults))

	// Validate the unmarshaled data
	for i, result := range fuchsiaResults {
		if result.BuildID == "" {
			return fmt.Errorf("record %d: build_id is empty", i)
		}
		if result.Builder == "" {
			return fmt.Errorf("record %d: builder is empty", i)
		}
		if result.CommitID == "" {
			return fmt.Errorf("record %d: commit_id is empty", i)
		}
		if result.PerfResults == nil || len(result.PerfResults) == 0 {
			return fmt.Errorf("record %d: perf_results is empty or null", i)
		}
		for j, perf := range result.PerfResults {
			if perf.TestSuite == "" {
				return fmt.Errorf("record %d, perf_result %d: test_suite is empty", i, j)
			}
			if perf.TestName == "" {
				return fmt.Errorf("record %d, perf_result %d: test_name is empty", i, j)
			}
			if perf.Unit == "" {
				return fmt.Errorf("record %d, perf_result %d: unit is empty", i, j)
			}
		}
	}

	// Prepare output directory
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	fmt.Printf("Output directory: %s\n", cfg.OutputDir)

	// Process each top-level record (build) separately
	for _, record := range fuchsiaResults {
		// Group results within this record by test_suite
		benchmarks := make(map[string][]FuchsiaPerfResultItem)
		for _, perf := range record.PerfResults {
			benchmarks[perf.TestSuite] = append(benchmarks[perf.TestSuite], perf)
		}

		// Create an output file for each benchmark in this record
		for benchmark, results := range benchmarks {
			skiaResult := SkiaPerfResult{
				Version: 1,
				GitHash: record.CommitID,
				Key: map[string]string{
					"benchmark": benchmark,
					"bot":       record.Builder,
					"master":    cfg.Master,
				},
				Results: PopulateResults(results),
				Links: map[string]string{
					"Test stdio": fmt.Sprintf("[Build Log](https://ci.chromium.org/b/%s)", record.BuildID),
				},
			}

			outputFileName := fmt.Sprintf("%s-%s-%s-%s.json", record.BuildID, benchmark, record.Builder, cfg.Master)
			outputFilePath := filepath.Join(cfg.OutputDir, outputFileName)

			fmt.Printf("  Writing to Output File Path: %s\n", outputFilePath)
			skiaResultJSON, err := json.MarshalIndent(skiaResult, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal SkiaPerfResult for build %s, benchmark %s: %w", record.BuildID, benchmark, err)
			}

			err = os.WriteFile(outputFilePath, skiaResultJSON, 0644)
			if err != nil {
				return fmt.Errorf("failed to write output file for build %s, benchmark %s: %w", record.BuildID, benchmark, err)
			}
		}
	}
	return nil
}

// PopulateResults creates a slice of SkiaResultItem from FuchsiaPerfResultItem slice.
func PopulateResults(perfResults []FuchsiaPerfResultItem) []SkiaResultItem {
	// Group by TestName
	tests := make(map[string][]FuchsiaPerfResultItem)
	for _, res := range perfResults {
		tests[res.TestName] = append(tests[res.TestName], res)
	}

	var skiaResults []SkiaResultItem

	for testName, results := range tests {
		newUnit, direction := MapUnitAndDirection(results[0].Unit)
		unitStr := newUnit + "_" + direction

		improvementDirection := "up"
		if strings.Contains(unitStr, "smallerIsBetter") {
			improvementDirection = "down"
		}

		// Calculate stats
		if len(results) == 0 {
			continue
		}

		baseStats, avgVal, stdev := CalculateStats(results)

		// Base item with all stats
		skiaResults = append(skiaResults, SkiaResultItem{
			Key: SkiaResultKey{
				Test:                 testName,
				Unit:                 unitStr,
				ImprovementDirection: improvementDirection,
			},
			Measurements: Measurements{Stat: baseStats},
		})

		// Average item
		skiaResults = append(skiaResults, SkiaResultItem{
			Key: SkiaResultKey{
				Test:                 testName + "_avg",
				Unit:                 unitStr,
				ImprovementDirection: improvementDirection,
			},
			Measurements: Measurements{
				Stat: []StatItem{{Value: "value", Measurement: avgVal}, {Value: "error", Measurement: stdev}},
			},
		})
	}

	return skiaResults
}

// CalculateStats calculates the statistics for a slice of FuchsiaPerfResultItem.
func CalculateStats(results []FuchsiaPerfResultItem) ([]StatItem, float64, float64) {
	if len(results) == 0 {
		return nil, 0.0, 0.0
	}

	// Get the original unit to determine if conversion is needed
	originalUnit := strings.SplitN(results[0].Unit, "_", 2)[0]

	convertValue := func(val float64) float64 {
		switch originalUnit {
		case "nanoseconds", "ns":
			return val / 1e6 // to milliseconds
		case "bytes/second":
			return val / (1024 * 1024) // to MiB/s, but unit becomes unitless
		}
		return val
	}

	var values []float64
	for _, r := range results {
		values = append(values, convertValue(r.Value))
	}

	if len(values) == 0 {
		return nil, 0.0, 0.0
	}

	minVal := values[0]
	maxVal := values[0]
	sumVal := 0.0
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
		sumVal += v
	}
	count := float64(len(values))
	avgVal := sumVal / count
	stdev := stdDev(values)

	stats := []StatItem{
		{Value: "value", Measurement: values[0]},
		{Value: "error", Measurement: stdev},
		{Value: "count", Measurement: count},
		{Value: "max", Measurement: maxVal},
		{Value: "min", Measurement: minVal},
		{Value: "sum", Measurement: sumVal},
	}

	return stats, avgVal, stdev
}

// MapUnitAndDirection converts the input unit and direction to the Skia Perf format.
// It returns the new unit and the improvement direction.
func MapUnitAndDirection(input string) (string, string) {
	parts := strings.SplitN(input, "_", 2)
	inputUnit := parts[0]
	inputDirection := ""
	if len(parts) > 1 {
		inputDirection = parts[1]
	}

	newUnit, ok := unitMap[inputUnit]
	if !ok {
		fmt.Printf("Warning: Unrecognized unit: %s, defaulting to unitless\n", inputUnit)
		newUnit = "unitless"
	}

	direction := ""
	if inputDirection == "biggerIsBetter" || inputDirection == "smallerIsBetter" {
		direction = inputDirection
	} else if inputDirection == "" {
		if val, ok := defaultImprovementDirection[newUnit]; ok {
			direction = val
		} else {
			direction = "smallerIsBetter" // Default direction
		}
	} else {
		fmt.Printf("Warning: Invalid direction: %s, using default\n", inputDirection)
		direction = "smallerIsBetter" // Default direction
	}

	return newUnit, direction
}
