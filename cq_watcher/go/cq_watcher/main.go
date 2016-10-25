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
	AFTER_COMMIT_POLL_TIME = 5 * time.Minute
	IN_FLIGHT_POLL_TIME    = 20 * time.Second // TODO(rmistry):Change this back!!!
	// Go back this long more than the above poll times to make sure we do not
	// lose any edge cases.
	POLL_TIME_BUFFER = 2 * time.Minute // TODO(rmistry): Move this directly to the method. The other guy is not going to use it!

	REFRESH_CQ_TRYBOTS_TIME = time.Hour

	// Constants for monitoring landed CLs.
	MAX_CLS_PER_POLL = 100
)

func monitorStatsForLandedCLs(cqClient *cq.Client, gerritClient *gerrit.Gerrit) {
	liveness := metrics2.NewLiveness("cq_watcher.after_commit")
	previousPollChanges := []*gerrit.ChangeInfo{}
	for _ = range time.Tick(time.Duration(AFTER_COMMIT_POLL_TIME)) {
		fmt.Println("Watching landed stuff here here")
		t_delta := time.Now().Add(-AFTER_COMMIT_POLL_TIME).Add(-POLL_TIME_BUFFER)
		changes, err := gerritClient.Search(MAX_CLS_PER_POLL, gerrit.SearchModifiedAfter(t_delta), gerrit.SearchStatus("merged"), gerrit.SearchProject("skia"))
		if err != nil {
			glog.Errorf("Error searching for merged issues in Gerrit: %s", err)
			continue
		}
		for _, change := range changes {
			if gerrit.ContainsAny(change.Issue, previousPollChanges) {
				// TODO(rmistry): REMOVE BELOW
				glog.Info("IGNORIN IGNORING IGNORING %d", change.Issue)
				continue
			}
			if err := cqClient.ReportCQStats(change.Issue); err != nil {
				glog.Errorf("Could not get CQ stats for %d: %s", change, err)
				continue
			}
		}
		previousPollChanges = changes
		liveness.Reset()
	}
}

func monitorStatsForInFlightCLs(cqClient *cq.Client, gerritClient *gerrit.Gerrit) {
	liveness := metrics2.NewLiveness("cq_watcher.in_flight")
	cqMetric := metrics2.GetInt64Metric(fmt.Sprintf("cq_watcher.%s.%s", cq.INFLIGHT_METRIC_NAME, "waiting_in_cq"))
	for _ = range time.Tick(time.Duration(IN_FLIGHT_POLL_TIME)) {
		fmt.Println("Polling In flight stuff here here")
		dryRunChanges, err := gerritClient.Search(MAX_CLS_PER_POLL, gerrit.SearchStatus("open"), gerrit.SearchProject("skia"), gerrit.SearchLabel(gerrit.COMMITQUEUE_LABEL, "1"))
		if err != nil {
			glog.Errorf("Error searching for open issues with dry run in Gerrit: %s", err)
			continue
		}
		cqChanges, err := gerritClient.Search(MAX_CLS_PER_POLL, gerrit.SearchStatus("open"), gerrit.SearchProject("skia"), gerrit.SearchLabel(gerrit.COMMITQUEUE_LABEL, "2"))
		if err != nil {
			glog.Errorf("Error searching for open issues waiting for CQ in Gerrit: %s", err)
			continue
		}

		// Combine dryrun and CQ changes into one slice.
		changes := dryRunChanges
		changes = append(changes, cqChanges...)
		cqMetric.Update(int64(len(changes)))

		for _, change := range changes {
			if err := cqClient.ReportCQStats(change.Issue); err != nil {
				glog.Errorf("Could not get CQ stats for %d: %s", change, err)
				continue
			}
		}
		liveness.Reset()
	}
}

func refreshCQTryBots(cqClient *cq.Client) {
	for _ = range time.Tick(time.Duration(REFRESH_CQ_TRYBOTS_TIME)) {
		if err := cqClient.RefreshCQTryBots(); err != nil {
			glog.Errorf("Error refresing CQ trybots: %s", err)
		}
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
	cqClient, err := cq.NewClient(httpClient, gerritClient, cq.GetSkiaCQTryBots)
	if err != nil {
		glog.Fatalf("Failed to create CQ client: %s", err)
	}
	// Periodically refresh slice of trybots to make sure any changes to cq.cfg
	// are picked up without needing to restart the app.
	go refreshCQTryBots(cqClient)

	// Monitor landed CLs.
	//go monitorStatsForLandedCLs(cqClient, gerritClient)
	// Monitor in-flight CLs.
	monitorStatsForInFlightCLs(cqClient, gerritClient)

	// TODO(rmistry): Remove this!
	//time.Sleep(3 * time.Minute)

	//fmt.Println("GOT THESE TRYBOTS: %s", tryBots)
	//fmt.Println(len(tryBots))

	// Will need 2 pollers here.
	//   One to scan commits as they land and call Report CQ Stats.
	//   Other to scan things currently in the CQ. (alert when too many things in the CQ 10/15. Alert when 1.5 number of CQ bots. Duration of any CQ trybot is more than 30 mins...).
}
