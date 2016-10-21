// Package cq provides tools for interacting with the CQ tools.
package cq

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/skia-dev/glog"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/util"
)

var (
	influxHost     = flag.String("influxdb_host", "104.154.112.119:10117", "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", "root", "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", "---", "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", "skmetrics", "The InfluxDB database.")
	testing2       = flag.Bool("testing", true, "Set to true for local testing.")
)

const (
	CQ_CFG_FILE_PATH = "infra/branch-config/cq.cfg"
	SKIA_REPO        = "https://skia.googlesource.com/skia"

	CQ_STATUS_URL_TEMPLATE = "https://chromium-cq-status.appspot.com/v2/patch-summary/skia-review/%d/%d"
)

// TODO(rmistry): Remove Client if you need nothing else here!
func NewClient() *Client {
	return &Client{}
}

type Client struct {
}

// TODO(rmistry): this could be in the init of the client.
func (c *Client) GetCQTryBots() ([]string, error) {
	var buf bytes.Buffer
	if err := gitiles.NewRepo(SKIA_REPO).ReadFile(CQ_CFG_FILE_PATH, &buf); err != nil {
		return nil, err
	}
	var cqCfg Config
	if err := proto.UnmarshalText(buf.String(), &cqCfg); err != nil {
		return nil, err
	}
	tryJobs := []string{}
	for _, bucket := range cqCfg.Verifiers.GetTryJob().GetBuckets() {
		for _, builder := range bucket.GetBuilders() {
			if builder.GetExperimentPercentage() > 0 {
				// Exclude experimental builders.
				continue
			}
			// TODO: Should you exclude presubmit builders as well because they mess up numbers sometimes because of public api checks and other stuff.
			tryJobs = append(tryJobs, builder.GetName())
		}
	}
	glog.Infof("The list of CQ trybots is: %s", tryJobs)
	fmt.Printf("\nThe list of CQ trybots is: %s", tryJobs)
	return tryJobs, nil
}

//func (c *Client) GetCQStatsInProgress(cqTryBots []string) {
//	// Use the Gerrit API to find things in flight? both dry run and CQ.
//}

// Add lots of logging so that things are easy to diagnose.
func (c *Client) ReportCQStats(cqTryBots []string, issue int64) error {
	// Move into init as well!
	client := httputils.NewTimeoutClient()

	// Get the full gerrit object.
	g, err := gerrit.NewGerrit(gerrit.GERRIT_SKIA_URL, "", client)
	if err != nil {
		return err
	}
	changeInfo, err := g.GetIssueProperties(issue)
	if err != nil {
		return err
	}
	patchsetIds := changeInfo.GetPatchsetIDs()
	latestPatchsetId := patchsetIds[len(patchsetIds)-1]
	if changeInfo.Committed {
		// The last patchset in Gerrit does not contain trybot information. This
		// will be fixed with crbug.com/634944.
		latestPatchsetId = patchsetIds[len(patchsetIds)-2]
	}

	trybots, err := g.GetTrybotResults(issue, latestPatchsetId)
	if err != nil {
		return err
	}
	gerritURL := fmt.Sprintf("%s/c/%d/%d", gerrit.GERRIT_SKIA_URL, issue, latestPatchsetId)
	if len(trybots) == 0 {
		glog.Infof("No trybot results were found for %s", gerritURL)
		return nil
	}

	glog.Infof("Starting processing %s", gerritURL)
	fmt.Printf("\nStarting processing %s", gerritURL)

	// TODO(rmistry): Just for testing.
	common.InitWithMetrics2("cq_watcher", influxHost, influxUser, influxPassword, influxDatabase, testing2)

	endTimeOfCQBots := time.Time{}
	maximumTrybotDuration := int64(0)
	for _, t := range trybots {
		if !util.ContainsAny(t.Parameters.BuilderName, cqTryBots) {
			glog.Infof("\tSkipping %s because it is not a CQ trybot", t.Parameters.BuilderName)
			fmt.Printf("\n\tSkipping %s because it is not a CQ trybot", t.Parameters.BuilderName)
			// TODO(rmistry): Uncomment the below!
			// continue
		}

		createdTime := time.Time(t.Created).UTC()
		completedTime := time.Time(t.Completed).UTC()
		if endTimeOfCQBots.Before(completedTime) {
			endTimeOfCQBots = completedTime
		}

		durationTags := map[string]string{
			"issue":    fmt.Sprintf("%d", issue),
			"patchset": fmt.Sprintf("%d", latestPatchsetId),
			"trybot":   t.Parameters.BuilderName,
		}
		duration := int64(completedTime.Sub(createdTime).Seconds())
		glog.Infof("\t%s was created at %s and completed at %s. Total duration: %d", t.Parameters.BuilderName, createdTime, completedTime, duration)
		fmt.Printf("\n\t%s was created at %s and completed at %s. Total duration: %d", t.Parameters.BuilderName, createdTime, completedTime, duration)

		metrics2.RawAddInt64PointAtTime("cq_watcher.after_commit.trybot_duration", durationTags, duration, completedTime)
		if duration > maximumTrybotDuration {
			maximumTrybotDuration = duration
		}
	}

	glog.Info("\tMaximum trybot duration: %d", maximumTrybotDuration)
	fmt.Printf("\n\tMaximum trybot duration: %d", maximumTrybotDuration)
	glog.Info("\tFurthest completion time: %s", endTimeOfCQBots)
	fmt.Printf("\n\tFurthest completion time: %s", endTimeOfCQBots)
	metrics2.RawAddInt64PointAtTime("cq_watcher.after_commit.total_duration", map[string]string{"issue": fmt.Sprintf("%d", issue), "patchset": fmt.Sprintf("%d", latestPatchsetId)}, maximumTrybotDuration, endTimeOfCQBots)

	// Sleep for 3 mins for metrics to upload?
	time.Sleep(3 * time.Minute)
	return nil
}
