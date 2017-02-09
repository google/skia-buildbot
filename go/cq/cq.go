// Package cq provides tools for interacting with the CQ tools.
package cq

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/util"
)

const (
	CQ_CFG_FILE_PATH = "infra/branch-config/cq.cfg"

	// Constants for in-flight metrics.
	INFLIGHT_METRIC_NAME     = "in_flight"
	INFLIGHT_TRYBOT_DURATION = "trybot_duration"
	INFLIGHT_TRYBOT_NUM      = "trybot_num"
	INFLIGHT_WAITING_IN_CQ   = "waiting_in_cq"

	// Constants for landed metrics.
	LANDED_METRIC_NAME     = "after_commit"
	LANDED_TRYBOT_DURATION = "trybot_duration"
	LANDED_TOTAL_DURATION  = "total_duration"
)

var (
	// Slice of all known presubmit bot names.
	PRESUBMIT_BOTS = []string{"skia_presubmit-Trybot"}

	// Mutext to control access to the slice of CQ trybots.
	cqTryBotsMutex sync.RWMutex
)

// NewClient creates a new client for interacting with CQ tools.
func NewClient(gerritClient *gerrit.Gerrit, cqTryBotsFunc GetCQTryBots, metricName string) (*Client, error) {
	cqTryBots, err := cqTryBotsFunc()
	if err != nil {
		return nil, err
	}
	return &Client{gerritClient, cqTryBots, cqTryBotsFunc, metricName}, err
}

// GetCQTryBots is an interface for returing the CQ trybots of a project.
type GetCQTryBots func() ([]string, error)

type Client struct {
	gerritClient  *gerrit.Gerrit
	cqTryBots     []string
	cqTryBotsFunc GetCQTryBots
	metricName    string
}

// GetSkiaCQTryBots is a Skia implementation of GetCQTryBots.
func GetSkiaCQTryBots() ([]string, error) {
	return getCQTryBots(common.REPO_SKIA)
}

// GetSkiaInfraCQTryBots is a Skia Infra implementation of GetCQTryBots.
func GetSkiaInfraCQTryBots() ([]string, error) {
	return getCQTryBots(common.REPO_SKIA)
}

// getCQTryBots is a convenience method for the Skia and Skia Infra CQ TryBots.
func getCQTryBots(repo string) ([]string, error) {
	var buf bytes.Buffer
	if err := gitiles.NewRepo(repo).ReadFile(CQ_CFG_FILE_PATH, &buf); err != nil {
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
				// Exclude presubmit bots because they could fail or be delayed
				// due to factors such as owners approval and other project
				// specific checks.
				continue
			}
			tryJobs = append(tryJobs, builder.GetName())
		}
	}
	sklog.Infof("The list of CQ trybots is: %s", tryJobs)
	return tryJobs, nil
}

// RefreshCQTryBots refreshes the slice of CQ trybots on the instance. Access
// to the trybots is protected by a RWMutex.
func (c *Client) RefreshCQTryBots() error {
	tryBots, err := c.cqTryBotsFunc()
	if err != nil {
		return err
	}

	cqTryBotsMutex.Lock()
	defer cqTryBotsMutex.Unlock()
	c.cqTryBots = tryBots
	return nil
}

// ReportCQStats reports all relevant stats for the specified Gerrit change.
// Note: Different stats are reported depending on whether the change has been
// merged or not.
func (c *Client) ReportCQStats(change int64) error {
	changeInfo, err := c.gerritClient.GetIssueProperties(change)
	if err != nil {
		return err
	}
	patchsetIds := changeInfo.GetPatchsetIDs()
	latestPatchsetId := patchsetIds[len(patchsetIds)-1]
	if changeInfo.Committed {
		// TODO(rmistry): The last patchset in Gerrit does not contain trybot
		// information so we have to look at the one immediately before it.
		// This will be fixed with crbug.com/634944.
		latestPatchsetId = patchsetIds[len(patchsetIds)-2]
	}

	builds, err := c.gerritClient.GetTrybotResults(change, latestPatchsetId)
	if err != nil {
		return err
	}
	// Consider only CQ bots.
	cqBuilds := []*buildbucket.Build{}
	for _, b := range builds {
		if c.isCQTryBot(b.Parameters.BuilderName) {
			cqBuilds = append(cqBuilds, b)
		}
	}
	gerritURL := fmt.Sprintf("%s/c/%d/%d", gerrit.GERRIT_SKIA_URL, change, latestPatchsetId)
	if len(cqBuilds) == 0 {
		sklog.Infof("No trybot results were found for %s", gerritURL)
		return nil
	}

	sklog.Infof("Starting processing %s. Merged status: %t", gerritURL, changeInfo.Committed)

	if changeInfo.Committed {
		c.ReportCQStatsForLandedCL(change, latestPatchsetId, cqBuilds)
	} else {
		c.ReportCQStatsForInFlightCL(change, latestPatchsetId, cqBuilds)
	}
	return nil
}

// ReportCQStatsForLandedCL reports the following metrics for the specified
// change and patchsetID:
// * The total time the change spent waiting for CQ trybots to complete.
// * The time each CQ trybot took to complete.
func (c *Client) ReportCQStatsForLandedCL(change, patchsetId int64, cqBuilds []*buildbucket.Build) {
	endTimeOfCQBots := time.Time{}
	maximumTrybotDuration := int64(0)
	for _, b := range cqBuilds {
		createdTime := time.Time(b.Created).UTC()
		completedTime := time.Time(b.Completed).UTC()
		if (completedTime == time.Time{}.UTC()) {
			sklog.Warningf("\tSkipping %s. The correct completed time has not shown up in Buildbucket yet.", b.Parameters.BuilderName)
			continue
		}
		if endTimeOfCQBots.Before(completedTime) {
			endTimeOfCQBots = completedTime
		}

		durationTags := map[string]string{
			"issue":    fmt.Sprintf("%d", change),
			"patchset": fmt.Sprintf("%d", patchsetId),
			"trybot":   b.Parameters.BuilderName,
		}
		duration := int64(completedTime.Sub(createdTime).Seconds())
		sklog.Infof("\t%s was created at %s and completed at %s. Total duration: %d", b.Parameters.BuilderName, createdTime, completedTime, duration)
		landedTrybotDurationMetric := metrics2.GetInt64Metric(fmt.Sprintf("%s_%s_%s", c.metricName, LANDED_METRIC_NAME, LANDED_TRYBOT_DURATION), durationTags)
		landedTrybotDurationMetric.Update(duration)

		if duration > maximumTrybotDuration {
			maximumTrybotDuration = duration
		}
	}

	sklog.Infof("\tMaximum trybot duration: %d", maximumTrybotDuration)
	sklog.Infof("\tFurthest completion time: %s", endTimeOfCQBots)
	totalDurationTags := map[string]string{
		"issue":    fmt.Sprintf("%d", change),
		"patchset": fmt.Sprintf("%d", patchsetId),
	}
	landedTotalDurationMetric := metrics2.GetInt64Metric(fmt.Sprintf("%s_%s_%s", c.metricName, LANDED_METRIC_NAME, LANDED_TOTAL_DURATION), totalDurationTags)
	landedTotalDurationMetric.Update(maximumTrybotDuration)
}

// ReportCQStatsForInFlightCL reports the following metrics for the specified
// change and patchsetID:
// * How long CQ trybots have been running for.
// * How many CQ trybots have been triggered.
func (c *Client) ReportCQStatsForInFlightCL(issue, patchsetId int64, cqBuilds []*buildbucket.Build) {
	totalTriggeredCQBots := int(0)
	currentTime := time.Now()
	for _, b := range cqBuilds {
		totalTriggeredCQBots++

		createdTime := time.Time(b.Created).UTC()
		completedTime := time.Time(b.Completed).UTC()
		if (completedTime != time.Time{}.UTC()) {
			// If build has completed then move on.
			continue
		}

		duration := int64(currentTime.Sub(createdTime).Seconds())
		sklog.Infof("\t%s was created at %s and is running after %d seconds", b.Parameters.BuilderName, createdTime, duration)
		durationTags := map[string]string{
			"issue":    fmt.Sprintf("%d", issue),
			"patchset": fmt.Sprintf("%d", patchsetId),
			"trybot":   b.Parameters.BuilderName,
		}
		inflightTrybotDurationMetric := metrics2.GetInt64Metric(fmt.Sprintf("%s_%s_%s", c.metricName, INFLIGHT_METRIC_NAME, INFLIGHT_TRYBOT_DURATION), durationTags)
		inflightTrybotDurationMetric.Update(duration)

	}
	cqTryBotsMutex.RLock()
	sklog.Infof("\t%d CQ bots have been triggered by %d/%d. There are %d CQ bots in cq.cfg", totalTriggeredCQBots, issue, patchsetId, len(c.cqTryBots))
	cqTryBotsMutex.RUnlock()
	numTags := map[string]string{
		"issue":    fmt.Sprintf("%d", issue),
		"patchset": fmt.Sprintf("%d", patchsetId),
	}
	trybotNumDurationMetric := metrics2.GetInt64Metric(fmt.Sprintf("%s_%s_%s", c.metricName, INFLIGHT_METRIC_NAME, INFLIGHT_TRYBOT_NUM), numTags)
	trybotNumDurationMetric.Update(int64(totalTriggeredCQBots))
}

func (c *Client) isCQTryBot(builderName string) bool {
	cqTryBotsMutex.RLock()
	isCQTrybot := util.ContainsAny(builderName, c.cqTryBots)
	cqTryBotsMutex.RUnlock()
	return isCQTrybot
}
