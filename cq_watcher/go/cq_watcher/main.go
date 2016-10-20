// The CQ Watcher monitors the Skia CQ and how long trybots in the CQ take. It
// pumps the results of the monitoring into InfluxDB.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/skia-dev/glog"
	//"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	//"go.skia.org/infra/go/httputils"
	"github.com/golang/protobuf/proto"
	"go.skia.org/infra/go/cq"
	"go.skia.org/infra/go/gitiles"
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
	DIAL_TIMEOUT         = time.Duration(5 * time.Second)
	REQUEST_TIMEOUT      = time.Duration(30 * time.Second)
	ISSUE_TRACKER_PERIOD = 15 * time.Minute
)

func main() {
	defer common.LogPanic()
	//common.InitWithMetrics2("cq_watcher", influxHost, influxUser, influxPassword, influxDatabase, testing)

	cqClient := cq.NewClient()

	// Then gitrepo to scan the commits.

	// Then something else looking at a CL.

	//client, err := auth.NewJWTServiceAccountClient("", "", &http.Transport{Dial: httputils.DialTimeout}, "https://www.googleapis.com/auth/userinfo.email")
	//if err != nil {
	//	glog.Fatalf("Failed to create client for talking to the issue tracker: %s", err)
	//}
	//go monitorIssueTracker(client)

	//liveness := metrics2.NewLiveness("probes")
}
