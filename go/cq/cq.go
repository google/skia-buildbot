// Package cq provides tools for interacting with the CQ tools.
package cq

import (
	"bytes"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/skia-dev/glog"
	"net/http"
	"time"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/util"
)

const (
	CQ_CFG_FILE_PATH = "infra/branch-config/cq.cfg"
	SKIA_REPO        = "https://skia.googlesource.com/skia"

	// Constants for in-flight metrics.
	INFLIGHT_METRIC_NAME     = "in_flight"
	INFLIGHT_TRYBOT_DURATION = "trybot_duration"
	INFLIGHT_TRYBOT_NUM      = "trybot_num"

	// Constants for landed metrics.
)

var (
	PRESUBMIT_BOTS = []string{"skia_presubmit-Trybot"}
)

// NewClient creates a new client for interacting with CQ tools.
func NewClient(httpClient *http.Client, gerritClient *gerrit.Gerrit, cqTryBotsFunc GetCQTryBots) (*Client, error) {
	cqTryBots, err := cqTryBotsFunc()
	if err != nil {
		return nil, err
	}
	return &Client{httpClient, gerritClient, cqTryBots, cqTryBotsFunc}, err
}

type Client struct {
	httpClient    *http.Client // TODO(rmistry): If ends up not being used then remove it
	gerritClient  *gerrit.Gerrit
	cqTryBots     []string
	cqTryBotsFunc GetCQTryBots
}

type GetCQTryBots func() ([]string, error)

func GetSkiaCQTryBots() ([]string, error) {
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
			if util.ContainsAny(builder.GetName(), PRESUBMIT_BOTS) {
				// Exclude presubmit bots.
				continue
			}
			tryJobs = append(tryJobs, builder.GetName())
		}
	}
	glog.Infof("The list of CQ trybots is: %s", tryJobs)
	return tryJobs, nil
}

func (c *Client) RefreshCQTryBots() error {
	tryBots, err := c.cqTryBotsFunc()
	if err != nil {
		return err
	}
	// TODO(rmistry) RW Mutex needed here!.
	c.cqTryBots = tryBots
	return nil
}

// TODO(rmistry): Add lots of documentation here.
// Does different things for landed vs not landed...
func (c *Client) ReportCQStats(issue int64) error {
	changeInfo, err := c.gerritClient.GetIssueProperties(issue)
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

	builds, err := c.gerritClient.GetTrybotResults(issue, latestPatchsetId)
	if err != nil {
		return err
	}
	gerritURL := fmt.Sprintf("%s/c/%d/%d", gerrit.GERRIT_SKIA_URL, issue, latestPatchsetId)
	if len(builds) == 0 {
		glog.Infof("No trybot results were found for %s", gerritURL)
		return nil
	}

	glog.Infof("Starting processing %s. Merged status: %t", gerritURL, changeInfo.Committed)

	if changeInfo.Committed {
		c.ReportCQStatsForLandedCL(issue, latestPatchsetId, builds)
	} else {
		c.ReportCQStatsForInFlightCL(issue, latestPatchsetId, builds)
	}

	return nil
}

func (c *Client) ReportCQStatsForLandedCL(issue, latestPatchsetId int64, builds []*buildbucket.Build) {
	endTimeOfCQBots := time.Time{}
	maximumTrybotDuration := int64(0)
	for _, b := range builds {
		if !util.ContainsAny(b.Parameters.BuilderName, c.cqTryBots) {
			glog.Infof("\tSkipping %s because it is not a CQ trybot", b.Parameters.BuilderName)
			continue
		}

		createdTime := time.Time(b.Created).UTC()
		completedTime := time.Time(b.Completed).UTC()
		if endTimeOfCQBots.Before(completedTime) {
			endTimeOfCQBots = completedTime
		}

		durationTags := map[string]string{
			"issue":    fmt.Sprintf("%d", issue),
			"patchset": fmt.Sprintf("%d", latestPatchsetId),
			"trybot":   b.Parameters.BuilderName,
		}
		duration := int64(completedTime.Sub(createdTime).Seconds())
		glog.Infof("\t%s was created at %s and completed at %s. Total duration: %d", b.Parameters.BuilderName, createdTime, completedTime, duration)

		metrics2.RawAddInt64PointAtTime("cq_watcher.after_commit.trybot_duration", durationTags, duration, completedTime)
		if duration > maximumTrybotDuration {
			maximumTrybotDuration = duration
		}
	}

	glog.Infof("\tMaximum trybot duration: %d", maximumTrybotDuration)
	glog.Infof("\tFurthest completion time: %s", endTimeOfCQBots)
	metrics2.RawAddInt64PointAtTime("cq_watcher.after_commit.total_duration", map[string]string{"issue": fmt.Sprintf("%d", issue), "patchset": fmt.Sprintf("%d", latestPatchsetId)}, maximumTrybotDuration, endTimeOfCQBots)
}

// Also look for an alert that shows a long time before CQ picks it up somehow. Time when label was set to when trybots run?
// TODO(rmistry): Add alert for how many total things are in the QUEUE! have a flag for dry run vs the other one.
func (c *Client) ReportCQStatsForInFlightCL(issue, latestPatchsetId int64, builds []*buildbucket.Build) {
	totalTriggeredCQBots := int(0)
	currentTime := time.Now()
	for _, b := range builds {
		if !util.ContainsAny(b.Parameters.BuilderName, c.cqTryBots) {
			glog.Infof("\tSkipping %s because it is not a CQ trybot", b.Parameters.BuilderName)
			continue
		}
		totalTriggeredCQBots++

		createdTime := time.Time(b.Created).UTC()
		completedTime := time.Time(b.Completed).UTC()
		if (completedTime != time.Time{}.UTC()) {
			// If build has completed then move on.
			continue
		}

		duration := int64(currentTime.Sub(createdTime).Seconds())
		glog.Infof("\t%s was created at %s and is running after %d seconds", b.Parameters.BuilderName, createdTime, duration)
		durationTags := map[string]string{
			"issue":    fmt.Sprintf("%d", issue),
			"patchset": fmt.Sprintf("%d", latestPatchsetId),
			"trybot":   b.Parameters.BuilderName,
		}
		metrics2.RawAddInt64PointAtTime(fmt.Sprintf("cq_watcher.%s.%s", INFLIGHT_METRIC_NAME, INFLIGHT_TRYBOT_DURATION), durationTags, duration, currentTime)
	}
	numTags := map[string]string{
		"issue":    fmt.Sprintf("%d", issue),
		"patchset": fmt.Sprintf("%d", latestPatchsetId),
	}
	glog.Warningf("\t%d CQ bots have been triggered by %d/%d. There are %d CQ bots in cq.cfg", totalTriggeredCQBots, issue, latestPatchsetId, len(c.cqTryBots))
	metrics2.RawAddInt64PointAtTime(fmt.Sprintf("cq_watcher.%s.%s", INFLIGHT_METRIC_NAME, INFLIGHT_TRYBOT_NUM), numTags, int64(totalTriggeredCQBots), currentTime)
}
