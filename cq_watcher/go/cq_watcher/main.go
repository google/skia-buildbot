// The CQ Watcher monitors the Skia CQ and how long trybots in the CQ take. It
// pumps the results of the monitoring into InfluxDB.
package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/skia-dev/glog"
	//"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/cq"
	"go.skia.org/infra/go/httputils"
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
	// TODO(rmistry): What should the below poll values be?
	AFTER_COMMIT_POLL_TIME = 2 * time.Minute
	IN_FLIGHT_POLL_TIME    = 1 * time.Minute
	// Go back this long more than the above poll times to make sure we do not
	// lose any edge cases.
	POLL_TIME_BUFFER = 1 * time.Minute

	// Constants for monitoring landed CLs.
	MAX_CLS_PER_POLL = 100
)

func monitorStatsForLandedCLs(cqClient *cq.Client, gerritClient *gerrit.Gerrit) {
	liveness := metrics2.NewLiveness("cq_watcher.after_commit")
	for _ = range time.Tick(time.Duration(AFTER_COMMIT_POLL_TIME)) {
		fmt.Println("Watching gerrit here here")
		// TODO(rmistry): Just query against gerrit! merged, project=skia in last 10 mins. Keep tracek of last encountered so no duplications or no ignores?

		t_delta := time.Now().Add(-AFTER_COMMIT_POLL_TIME).Add(-POLL_TIME_BUFFER)
		changes, err := gerritClient.Search(MAX_CLS_PER_POLL, gerrit.SearchModifiedAfter(t_delta), gerrit.SearchStatus("merged"), gerrit.SearchProject("skia"))
		if err != nil {
			glog.Errorf("Error searching for merged issues in Gerrit: %s", err)
			continue
		}
		for _, change := range changes {
			if err := cqClient.ReportCQStats(change.Issue); err != nil {
				glog.Errorf("Could not get CQ stats for %d: %s", change, err)
				continue
			}
		}
		liveness.Reset() // TODO(rmistry): Check that the liveness thingy is showing up!
	}
}

func monitorStatsForInFlightCLs(cqClient *cq.Client, gerritClient *gerrit.Gerrit) {
	liveness := metrics2.NewLiveness("cq_watcher.in_flight")
	for _ = range time.Tick(time.Duration(IN_FLIGHT_POLL_TIME)) {
		fmt.Println("Polling In flight stuff here here")

		/*
			t_delta := time.Now().Add(-2 * 24 * time.Hour)
			changes, err := gerritClient.Search(MAX_CLS_PER_POLL, gerrit.SearchModifiedAfter(t_delta), gerrit.SearchStatus("merged"), gerrit.SearchProject("skia"))
			if err != nil {
				glog.Errorf("Error searching for merged issues in Gerrit: %s", err)
				continue
			}
			for _, change := range changes {

			}
		*/
		liveness.Reset()
	}
}

// TODO(rmistry): Create two polls and liveness for both!
func main() {
	defer common.LogPanic()
	common.InitWithMetrics2("cq_watcher", influxHost, influxUser, influxPassword, influxDatabase, testing)

	httpClient := httputils.NewTimeoutClient()
	gerritClient, err := gerrit.NewGerrit(gerrit.GERRIT_SKIA_URL, "", httpClient)
	if err != nil {
		glog.Fatalf("Failed to create Gerrit client: %s", err)
	}
	cqClient, err := cq.NewClient(httpClient, gerritClient, nil)
	if err != nil {
		glog.Fatalf("Failed to create CQ client: %s", err)
	}

	go monitorStatsForLandedCLs(cqClient, gerritClient)

	monitorStatsForInFlightCLs(cqClient, gerritClient)

	// TODO(rmistry): Remove this!
	//time.Sleep(3 * time.Minute)

	//fmt.Println("GOT THESE TRYBOTS: %s", tryBots)
	//fmt.Println(len(tryBots))

	// Will need 2 pollers here.
	//   One to scan commits as they land and call Report CQ Stats.
	//   Other to scan things currently in the CQ. (alert when too many things in the CQ 10/15. Alert when 1.5 number of CQ bots. Duration of any CQ trybot is more than 30 mins...).
}
