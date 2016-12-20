// logmetrics runs queries over all the data store in Google Logging and then
// pushes those counts into influxdb.
package main

import (
	"flag"
	"fmt"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/logmetrics/go/config"
	"golang.org/x/net/context"
	"google.golang.org/api/logging/v2beta1"
)

var (
	influxDatabase  = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
	influxHost      = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxPassword  = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxUser      = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	local           = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	metricsFilename = flag.String("metrics_filename", "metrics.toml", "The file with all the metrics and their filters.")
	validateOnly    = flag.Bool("validate_only", false, "Exits after successfully reading the config file.")

	loggingService *logging.Service
	metrics        []config.Metric
)

func oneMetric(metric config.Metric, now time.Time) {
	ts1 := now.Add(-time.Minute).UTC().Format(time.RFC3339)
	ts2 := now.Add(-2 * time.Minute).UTC().Format(time.RFC3339)
	filter := fmt.Sprintf(`(%s)
AND (timestamp <= %q)
AND (timestamp > %q)`, metric.Filter, ts1, ts2)
	req := &logging.ListLogEntriesRequest{
		Filter:     filter,
		OrderBy:    "timestamp desc",
		ProjectIds: []string{"google.com:skia-buildbots"},
		PageSize:   1000,
	}
	// Count all the results, handling paging.
	count := 0
	for {
		resp, err := loggingService.Entries.List(req).Fields("entries(timestamp),nextPageToken").Context(context.Background()).Do()
		if err != nil {
			sklog.Errorf("Request Failed: %s", err)
			return
		}
		count += len(resp.Entries)
		if resp.NextPageToken == "" {
			break
		}
		req.PageToken = resp.NextPageToken
	}
	sklog.Infof("Name: %s Count: %d QPS: %0.2f\n", metric.Name, count, float32(count)/60)
	metrics2.GetFloat64Metric(metric.Name, nil /* tags */).Update(float64(count) / 60)
}

func step() {
	now := time.Now()
	for _, metric := range metrics {
		oneMetric(metric, now)
	}
}

func main() {
	defer common.LogPanic()
	if *local {
		common.Init()
	} else {
		common.InitWithMetrics2("logmetrics", influxHost, influxUser, influxPassword, influxDatabase, local)
	}
	client, err := auth.NewDefaultJWTServiceAccountClient(logging.LoggingReadScope)
	if err != nil {
		sklog.Fatalf("Failed to create service account client: %s", err)
	}
	loggingService, err = logging.New(client)
	if err != nil {
		sklog.Fatalf("Failed to create logging client: %s", err)
	}
	metrics, err = config.ReadMetrics(*metricsFilename)
	if err != nil {
		sklog.Fatalf("Failed to read metrics file %q: %s", *metricsFilename, err)
	}
	if *validateOnly {
		fmt.Printf("Successfully validated.\n")
		return
	}
	step()
	for _ = range time.Tick(time.Minute) {
		step()
	}
}
