/*
	Periodically report the status of all attached Android devices to InfluxDB.
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

const (
	MEASUREMENT = "android-stats"
)

var (
	frequency   = flag.String("frequency", "1m", "How often to send data.")
	local       = flag.Bool("local", false, "Whether or not we're running in local testing mode.")
	promPort    = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	statsScript = flag.String("stats_script", "", "Script to run which generates stats.")
)

// reportStats drills down recursively until it hits a non-map value,
// then reports that value with the measurement as given above, tags for device,
// serial, and stat name, which is composed of the remaining map keys, eg.
// device=Nexus_5X serial=002e3da61560d3d4 stat=battery-level
func reportStats(device, serial, stat string, val interface{}) {
	float, ok := val.(float64)
	if ok {
		tags := map[string]string{
			"device": device,
			"serial": serial,
			"stat":   stat,
		}
		sklog.Infof("%s %v = %v", MEASUREMENT, tags, float)
		metrics2.GetFloat64Metric(MEASUREMENT, map[string]string{}).Update(float)
		return
	}
	m, ok := val.(map[string]interface{})
	if ok {
		for k, v := range m {
			reportStats(device, serial, stat+"-"+k, v)
		}
	}
}

// generateStats runs the statistics generation script, parses its output, and
// reports the data into metrics.
//
// The script produces data in this format:
// {
//   "Nexus_5X": {
//     "002e3da61560d3d4": {
//       "battery": {
//         "ac": 0,
//         "health": 2,
//         "level": 100,
//         "max": 500000,
//         "present": 1,
//         "status": 5,
//         "temp": 282,
//         "usb": 1,
//         "voltage": 4311,
//         "wireless": 0
//       },
//       "temperature": 28.0
//     }
//   }
// }
//
func generateStats(ctx context.Context) error {
	output, err := exec.RunSimple(ctx, *statsScript)
	if err != nil {
		return err
	}

	res := map[string]map[string]map[string]interface{}{}
	if err := json.Unmarshal([]byte(output), &res); err != nil {
		return err
	}

	for device, deviceStats := range res {
		for serial, statMap := range deviceStats {
			for stat, val := range statMap {
				reportStats(device, serial, stat, val)
			}
		}
	}

	return nil
}

func main() {
	common.InitWithMust(
		"android_stats",
		common.PrometheusOpt(promPort),
	)

	ctx := context.Background()

	if *statsScript == "" {
		sklog.Fatal("You must provide --stats_script.")
	}

	pollFreq, err := time.ParseDuration(*frequency)
	if err != nil {
		sklog.Fatalf("Invalid value for frequency %q: %s", *frequency, err)
	}

	if err := generateStats(ctx); err != nil {
		sklog.Fatal(err)
	}
	for range time.Tick(pollFreq) {
		if err := generateStats(ctx); err != nil {
			sklog.Error(err)
		}
	}
}
