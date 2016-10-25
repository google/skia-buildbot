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
	AFTER_COMMIT_POLL_TIME  = 5 * time.Minute
	IN_FLIGHT_POLL_TIME     = 30 * time.Second
	REFRESH_CQ_TRYBOTS_TIME = time.Hour

	MAX_CLS_PER_POLL = 100
	METRIC_NAME      = "cq_watcher"
)

func monitorStatsForInFlightCLs(cqClient *cq.Client, gerritClient *gerrit.Gerrit) {
	liveness := metrics2.NewLiveness(fmt.Sprintf("%s.%s", METRIC_NAME, cq.INFLIGHT_METRIC_NAME))
	cqMetric := metrics2.GetInt64Metric(fmt.Sprintf("%s.%s.%s", METRIC_NAME, cq.INFLIGHT_METRIC_NAME, cq.INFLIGHT_WAITING_IN_CQ))
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

func monitorStatsForLandedCLs(cqClient *cq.Client, gerritClient *gerrit.Gerrit) {
	liveness := metrics2.NewLiveness(fmt.Sprintf("%s.%s", METRIC_NAME, cq.LANDED_METRIC_NAME))
	previousPollChanges := []*gerrit.ChangeInfo{}
	for _ = range time.Tick(time.Duration(AFTER_COMMIT_POLL_TIME)) {
		fmt.Println("Watching landed stuff here here")
		// Add a short (2 min) buffer to overlap with the last poll to make sure
		// we do not lose any edge cases.
		t_delta := time.Now().Add(-AFTER_COMMIT_POLL_TIME).Add(-2 * time.Minute)
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

func refreshCQTryBots(cqClient *cq.Client) {
	for _ = range time.Tick(time.Duration(REFRESH_CQ_TRYBOTS_TIME)) {
		if err := cqClient.RefreshCQTryBots(); err != nil {
			glog.Errorf("Error refresing CQ trybots: %s", err)
		}
	}
}

func main() {
	defer common.LogPanic()
	common.InitWithMetrics2(METRIC_NAME, influxHost, influxUser, influxPassword, influxDatabase, testing)

	httpClient := httputils.NewTimeoutClient()
	gerritClient, err := gerrit.NewGerrit(gerrit.GERRIT_SKIA_URL, "", httpClient)
	if err != nil {
		glog.Fatalf("Failed to create Gerrit client: %s", err)
	}
	cqClient, err := cq.NewClient(gerritClient, cq.GetSkiaCQTryBots, METRIC_NAME)
	if err != nil {
		glog.Fatalf("Failed to create CQ client: %s", err)
	}
	// Periodically refresh slice of trybots to make sure any changes to cq.cfg
	// are picked up without needing to restart the app.
	go refreshCQTryBots(cqClient)

	// Monitor in-flight CLs.
	go monitorStatsForInFlightCLs(cqClient, gerritClient)
	// Monitor landed CLs.
	monitorStatsForLandedCLs(cqClient, gerritClient)
}
