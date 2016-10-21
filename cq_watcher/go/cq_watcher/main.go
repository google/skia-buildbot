// The CQ Watcher monitors the Skia CQ and how long trybots in the CQ take. It
// pumps the results of the monitoring into InfluxDB.
package main

import (
	"flag"
	"time"

	//"github.com/skia-dev/glog"
	//"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	//"go.skia.org/infra/go/httputils"
	//"go.skia.org/infra/go/cq"
	//"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/metrics2"
)

var (
	runEvery = flag.Duration("run_every", 5*time.Minute, "How often to scan the repo for new commits.")
	testing2 = flag.Bool("testing2", true, "Set to true for local testing.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
)

const (
	DIAL_TIMEOUT         = time.Duration(5 * time.Second)
	REQUEST_TIMEOUT      = time.Duration(30 * time.Second)
	ISSUE_TRACKER_PERIOD = 15 * time.Minute
)

func main() {
	defer common.LogPanic()
	common.InitWithMetrics2("cqwatcher", influxHost, influxUser, influxPassword, influxDatabase, testing2)

	// Just testing to see what happens.
	durationMetric := metrics2.GetInt64Metric("cqwatchertest", map[string]string{"issue": "123", "patchset": "2"})
	durationMetric.Update(1000)

	time.Sleep(10 * time.Minute)

	//cqClient := cq.NewClient()
	//tryBots, err := cqClient.GetCQTryBots()
	//if err != nil {
	//	glog.Fatalf("Failed to get CQ trybots from cq.cfg: %s", err)
	//}

	//fmt.Println("GOT THESE TRYBOTS: %s", tryBots)
	//fmt.Println(len(tryBots))

	// Will need 2 pollers here.
	//   One to scan commits as they land.
	//   Other to scan things currently in the CQ. (alert when too many things in the CQ 10/15).
	// Maybe do this last.

	// Then something else looking at a CL.

	//client, err := auth.NewJWTServiceAccountClient("", "", &http.Transport{Dial: httputils.DialTimeout}, "https://www.googleapis.com/auth/userinfo.email")
	//if err != nil {
	//	glog.Fatalf("Failed to create client for talking to the issue tracker: %s", err)
	//}
	//go monitorIssueTracker(client)

	//liveness := metrics2.NewLiveness("probes")
}
