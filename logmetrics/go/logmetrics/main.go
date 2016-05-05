// logmetrics runs queries over all the data store in Google Logging and then
// pushes those counts into influxdb.
package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/metrics2"
	"golang.org/x/net/context"
	"google.golang.org/api/logging/v2beta1"
)

var (
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")

	loggingService *logging.Service
	m              *metrics2.Float64Metric
)

func step() {
	// Search for all nginx log entries in the last minute.
	now := time.Now()
	ts1 := now.Add(-time.Minute).UTC().Format(time.RFC3339)
	ts2 := now.Add(-2 * time.Minute).UTC().Format(time.RFC3339)
	filter := fmt.Sprintf(`resource.type="gce_instance"
AND (
  labels."compute.googleapis.com/resource_name"="skia-skfe-1"
  OR labels."compute.googleapis.com/resource_name"="skia-skfe-2" )
AND jsonPayload.host:*
AND (timestamp <= %q)
AND (timestamp > %q)`, ts1, ts2)
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
			glog.Errorf("Request Failed: %s", err)
			return
		}
		count += len(resp.Entries)
		if resp.NextPageToken == "" {
			break
		}
		req.PageToken = resp.NextPageToken
	}
	glog.Infof("Count: %d QPS: %0.1f\n", count, float32(count)/60)
	m.Update(float64(count) / 60)
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
		glog.Fatalf("Failed to create service account client: %s", err)
	}
	loggingService, err = logging.New(client)
	if err != nil {
		glog.Fatalf("Failed to create logging client: %s", err)
	}
	m = metrics2.GetFloat64Metric("qps" /* tags */, nil)
	step()
	for _ = range time.Tick(time.Minute) {
		step()
	}
}
