/*
	Periodically report the status of all attached Android devices to InfluxDB.
*/

package main

import (
	"encoding/json"
	"flag"
	"reflect"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
)

var (
	frequency      = flag.String("frequency", "1m", "How often to send data.")
	graphiteServer = flag.String("graphite server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	statsScript    = flag.String("stats_script", "", "Script to run which generates stats.")
)

// parseAndReportStats drills down recursively until it hits a non-map value,
// then reports that value with the metric name as the path of keys to that
// value, eg. "Nexus_5X.002e3da61560d3d4.battery.level".
func parseAndReportStats(name string, stats map[string]interface{}) {
	for k, v := range stats {
		measurement := name + "." + k
		kind := reflect.ValueOf(v).Kind()
		if kind == reflect.Map {
			parseAndReportStats(measurement, v.(map[string]interface{}))
		} else {
			glog.Infof("%s = %v", measurement, v)
			metrics.GetOrRegisterGaugeFloat64(measurement, metrics.DefaultRegistry).Update(v.(float64))
		}
	}
}

// generateStats runs the statistics generation script, parses its output, and
// reports the data into InfluxDB.
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
func generateStats() error {
	output, err := exec.RunSimple(*statsScript)
	if err != nil {
		return err
	}

	res := map[string]interface{}{}
	if err := json.Unmarshal([]byte(output), &res); err != nil {
		return err
	}

	parseAndReportStats("androidstats", res)

	return nil
}

func main() {
	defer common.LogPanic()
	common.InitWithMetrics("android_stats", graphiteServer)

	pollFreq, err := time.ParseDuration(*frequency)
	if err != nil {
		glog.Fatalf("Invalid value for frequency %q: %s", *frequency, err)
	}

	if err := generateStats(); err != nil {
		glog.Fatal(err)
	}
	for _ = range time.Tick(pollFreq) {
		if err := generateStats(); err != nil {
			glog.Error(err)
		}
	}
}
