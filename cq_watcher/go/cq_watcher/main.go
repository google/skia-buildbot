// The CQ Watcher monitors the Skia CQ and how long trybots in the CQ take. It
// pumps the results of the monitoring into InfluxDB.
package main

import (
	"flag"
	"time"

	"github.com/skia-dev/glog"
	//"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	//"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/cq"
	//"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/metrics2"
)

var (
	runEvery = flag.Duration("run_every", 5*time.Minute, "How often to scan the repo for new commits.")
	testing  = flag.Bool("testing", false, "Set to true for local testing.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
)

const (
	DIAL_TIMEOUT        = time.Duration(5 * time.Second)
	REQUEST_TIMEOUT     = time.Duration(30 * time.Second)
	AFTER_COMMIT_PERIOD = 5 * time.Minute
)

func monitorStatsForLandedCLs(cqClient *cq.Client) {
	liveness := metrics2.NewLiveness("cq_watcher.after_commit")
	for _ = range time.Tick(AFTER_COMMIT_PERIOD) {
		// TODO(rmistry): Just query against gerrit! merged, project=skia in last 10 mins. Keep tracek of last encountered so no duplications or no ignores?

		// TODO(rmistry): Send in Gerrit object to the client and use it (not the one from the client)

		// TODO(rmistry): Extract out CL and then send it to
		liveness.Reset()
	}
}

// TODO(rmistry): Create two polls and liveness for both!
func main() {
	defer common.LogPanic()
	common.InitWithMetrics2("cq_watcher", influxHost, influxUser, influxPassword, influxDatabase, testing)

	cqClient, err := cq.NewClient(nil, nil, nil)
	if err != nil {
		glog.Fatalf("Failed to get create CQ client: %s", err)
	}

	go monitorStatsForLandedCLs(cqClient)

	// Move the below to monitorStatsForLandedCLs..
	issue := int64(3722)
	if err := cqClient.ReportCQStats(issue); err != nil {
		glog.Fatalf("Could not get CQ stats for %d: %s", issue, err)
	}

	// TODO(rmistry): Remove this!
	time.Sleep(3 * time.Minute)

	//fmt.Println("GOT THESE TRYBOTS: %s", tryBots)
	//fmt.Println(len(tryBots))

	// Will need 2 pollers here.
	//   One to scan commits as they land and call Report CQ Stats.
	//   Other to scan things currently in the CQ. (alert when too many things in the CQ 10/15. Alert when 1.5 number of CQ bots. Duration of any CQ trybot is more than 30 mins...).
}
