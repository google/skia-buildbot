// Tool for converting CBB v3 data to Skia perf format.
//
// All input/output files must be in ~/cbb directory, with the input file
// named "cbb_v3_data.csv". All output files have .json extension.
//
// Steps for copying data from CBB v3 (google3) to CBB v4 (Skia):
// * Create a `cbb` folder in your home directory.
//		mkdir ~/cbb
// * From a google3 client window, run the following command to download CBB v3
//   data to a CSV file.
//		blaze run //experimental/users/zhanliang/cbb/download_f1 -- --output ~/cbb/cbb_v3_data.csv
// * Run this script to convert CBB v3 data into the appropriate JSON files.
//		bazelisk run //pinpoint/go/cbb/upload_v3_data
//   Then review these JSON files to make sure they were generated correctly.
// * Elevate to breakglass permission.
// * Run the following command to upload the JSON files to pers dashboard.
//		gsutil -m cp ~/cbb/*.json gs://chrome-perf-non-public/ingest/YYYY/MM/DD/ChromiumPerf/cbb-backfill

package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.skia.org/infra/perf/go/ingest/format"
)

func loadCsv(path string) [][]string {
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("Failed to open CSV file at %s: %v", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read the header row.
	_, err = reader.Read()
	if err != nil {
		log.Fatalf("Failed to read header row: %v", err)
	}

	// Read all the remaining records.
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Failed to read CSV records: %v", err)
	}

	return records
}

// Chrome version from v3 data always has the last part padded to 3 digits,
// e.g., 137.0.7151.070. Normalize it to standard 137.0.7151.70.
func normalizeChromeVersion(version string) string {
	parts := strings.Split(version, ".")
	for len(parts[3]) > 1 && strings.HasPrefix(parts[3], "0") {
		parts[3] = parts[3][1:]
	}
	return strings.Join(parts, ".")

}

// Mappings of browser versions to commit positions.
var chromeStableCP = map[string]int{
	"137.0.7151.41":  1475800,
	"137.0.7151.56":  1475850,
	"137.0.7151.69":  1475900,
	"137.0.7151.70":  1475903,
	"137.0.7151.104": 1475950,
	"137.0.7151.120": 1476000,
	"138.0.7204.35":  1476735,
	"138.0.7204.50":  1480635,
	"138.0.7204.93":  1480700,
	"138.0.7204.97":  1480704,
	"138.0.7204.101": 1480708,
	"138.0.7204.158": 1492300,
	"138.0.7204.169": 1492325,
	"138.0.7204.184": 1493944,
	"139.0.7258.66":  1494598,
	"139.0.7258.67":  1497213,
	"139.0.7258.68":  1500427,
	"139.0.7258.128": 1502986,
}
var chromeStableAndroidCP = map[string]int{
	"137.0.7151.44":  1475800,
	"137.0.7151.61":  1475850,
	"137.0.7151.72":  1475900,
	"137.0.7151.89":  1475903,
	"137.0.7151.115": 1475950,
	"138.0.7204.35":  1476735,
	"138.0.7204.45":  1480596,
	"138.0.7204.63":  1491563,
	"138.0.7204.157": 1492300,
	"138.0.7204.168": 1492325,
	"138.0.7204.179": 1493944,
	"139.0.7258.62":  1494598,
	"139.0.7258.63":  1500427,
	"139.0.7258.123": 1502986,
}
var chromeDevCP = map[string]int{
	"139.0.7232.3": 1475903,
	"139.0.7246.0": 1476735,
	"139.0.7246.2": 1476737,
	"140.0.7259.2": 1480635,
	"140.0.7299.0": 1489000,
	"140.0.7312.0": 1492325,
	"140.0.7327.6": 1497213,
	"141.0.7340.0": 1498865,
	"141.0.7354.0": 1501632,
	"141.0.7354.3": 1501632,
}
var chromeDevAndroidCP = map[string]int{
	"139.0.7232.2": 1475903,
	"139.0.7246.0": 1476735,
	"140.0.7259.0": 1491563,
	"140.0.7299.0": 1489000,
	"140.0.7313.0": 1492325,
	"140.0.7327.5": 1497213,
	"141.0.7340.3": 1498865,
	"141.0.7354.0": 1501632,
}
var safariStableCP = map[string]int{
	"18.4 (20621.1.15.11.10)": 1475903,
	"18.5 (20621.2.5.11.8)":   1488562,
	"18.6 (20621.3.11.11.3)":  1493855,
}
var safariTPCP = map[string]int{
	"26.0 (Release 223, 20622.1.18.11.5)": 1491633,
	"26.0 (Release 224, 20622.1.20.1)":    1493750,
	"26.0 (Release 225, 20623.1.1.1)":     1499125,
}
var EdgeStableCP = map[string]int{
	"136.0.3240.76":  1475903,
	"138.0.3351.83":  1488562,
	"138.0.3351.109": 1495010,
	"139.0.3405.86":  1500568,
}
var EdgeDevCP = map[string]int{
	"138.0.3324.1": 1475903,
	"140.0.3430.1": 1488562,
	"140.0.3456.0": 1495010,
	"140.0.3485.6": 1500568,
}

// Given a browser version, return the equivalent commit position,
// which is used as the x-axis label in Skia perf dashboard.
//   - If the version string is in one of the look-up tables, use table result.
//   - If Safari or Edge version is not in the table, fall back to the Chrome
//     version that the browser was paired with.
//   - If the Chrome version is not in the table, use a heuristic to calculate
//     a "fake" commit position.
func getCP(device, browser, version, chromeVersion string) int {
	// Start with checking Safari or Edge lookup table.
	var cp int
	var found bool
	switch browser {
	case "safari stable":
		cp, found = safariStableCP[version]
	case "safari preview":
		cp, found = safariTPCP[version]
	case "edge stable":
		cp, found = EdgeStableCP[version]
	case "edge dev":
		cp, found = EdgeDevCP[version]
	}
	if found {
		return cp
	}

	// Try Chrome lookup table.
	isStable := browser == "chrome stable" || browser == "safari stable" || browser == "edge stable"
	isAndroid := device == "Pixel Tablet"
	if isStable {
		if isAndroid {
			cp, found = chromeStableAndroidCP[chromeVersion]
		} else {
			cp, found = chromeStableCP[chromeVersion]
		}
	} else {
		if isAndroid {
			cp, found = chromeDevAndroidCP[chromeVersion]
		} else {
			cp, found = chromeDevCP[chromeVersion]
		}
	}
	if found {
		return cp
	}

	// Fallback to heuristics, which doesn't work well with new Chrome versions.
	if isStable {
		if chromeVersion > "137.0.7151" {
			log.Fatalf("Unable to translate Chrome Stable version %s to commit position (device %s)", chromeVersion, device)
		}
	} else {
		if chromeVersion > "139.0.7232" {
			log.Fatalf("Unable to translate Chrome Dev version %s to commit position (device %s)", chromeVersion, device)
		}
	}

	var parts [4]int
	for i, s := range strings.Split(chromeVersion, ".") {
		v, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			log.Fatalf("Unable to parse component %s of version %s: %v", s, chromeVersion, err)
		}
		parts[i] = int(v)
	}

	// Since we can't go back in time to create CBB reference files for old releases,
	// we use fake commit positions. The following formulas are selected to convert
	// Chrome commit numbers to relatively even-spaced fake commit positions
	// between 1350000 and 1470000, roughly corresponding to the time period covered
	// by the legacy CBB data we are copying.
	if isStable {
		return parts[0]*14000 + parts[3]*100 - 445000
	} else {
		return parts[2]*215 + parts[3]*20 - 80000
	}
}

func createPerfFile(device, browser, version, benchmark string, commitPosition int, value float32, outPath string) error {
	var bot string
	switch device {
	case "Apple M3":
		bot = "mac-m3-pro-perf-cbb"
	case "Intel":
		bot = "win-victus-perf-cbb"
	case "Pixel Tablet":
		bot = "android-pixel-tangor-perf-cbb"
	default:
		return fmt.Errorf("invalid device name: %s", device)
	}

	var br string
	switch browser {
	case "chrome stable":
		br = "Chrome Stable"
	case "chrome dev":
		br = "Chrome Dev"
	case "safari stable":
		br = "Safari Stable"
	case "safari preview":
		br = "Safari Technology Preview"
	case "edge stable":
		br = "Edge Stable"
	case "edge dev":
		br = "Edge Dev"
	default:
		return fmt.Errorf("invalid browser name: %s", browser)
	}
	br += " (Legacy Data)"

	var test string
	switch benchmark {
	case "speedometer3":
		test = "Score"
	case "jetstream2":
		test = "Total"
	case "motionmark1.3":
		test = "score"
	default:
		return fmt.Errorf("invalid benchmark name: %s", benchmark)
	}

	data := format.Format{
		Version: 1,
		GitHash: fmt.Sprintf("CP:%d", commitPosition),
		Key: map[string]string{
			"master":    "ChromiumPerf",
			"bot":       bot,
			"benchmark": benchmark + ".crossbench",
		},
		Links: map[string]string{
			"Browser Version": version,
		},
		Results: []format.Result{
			{
				Key: map[string]string{
					"test":                  test,
					"subtest_1":             br,
					"unit":                  "unitless",
					"improvement_direction": "up",
				},
				Measurements: map[string][]format.SingleMeasurement{
					"stat": {
						{
							Value:       "value",
							Measurement: value,
						},
					},
				},
			},
		},
	}

	filename := fmt.Sprintf("%s.%s.%s.%d.json", bot, benchmark, strings.ReplaceAll(browser, " ", "-"), commitPosition)
	file, err := os.Create(filepath.Join(outPath, filename))
	if err != nil {
		return err
	}
	defer file.Close()

	err = data.Write(file)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get user home directory: %v", err)
	}
	cbbPath := filepath.Join(home, "cbb")
	csvPath := filepath.Join(cbbPath, "cbb_v3_data.csv")

	records := loadCsv(csvPath)
	for _, row := range records {
		if len(row) >= 8 {
			device := row[0]
			controlBrowserName := row[1]
			controlBrowserVersion := normalizeChromeVersion(row[2])
			treatmentBrowserName := row[3]
			treatmentBrowserVersion := row[4]
			benchmark := row[5]

			controlMedian, err := strconv.ParseFloat(row[6], 32)
			if err != nil {
				log.Fatalf("Could not parse Control_browser_median value %q: %v.", row[6], err)
			}
			treatmentMedian, err := strconv.ParseFloat(row[7], 32)
			if err != nil {
				log.Fatalf("Could not parse Treatment_browser_median value %q: %v.", row[7], err)
			}

			err = createPerfFile(
				device, controlBrowserName, controlBrowserVersion, benchmark,
				getCP(device, controlBrowserName, controlBrowserVersion, controlBrowserVersion),
				float32(controlMedian), cbbPath)
			if err != nil {
				log.Fatalf(
					"Unable to create file for Chrome (%s %s %s %s): %v.",
					device, controlBrowserName, controlBrowserVersion, benchmark, err)
			}
			if device != "Pixel Tablet" {
				err = createPerfFile(
					device, treatmentBrowserName, treatmentBrowserVersion, benchmark,
					getCP(device, treatmentBrowserName, treatmentBrowserVersion, controlBrowserVersion),
					float32(treatmentMedian), cbbPath)
				if err != nil {
					log.Fatalf(
						"Unable to create file for alternative browser (%s %s %s %s): %v.",
						device, treatmentBrowserName, treatmentBrowserVersion, benchmark, err)
				}
			}
		}
	}
}
