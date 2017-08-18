package swarming_metrics

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
)

const (
	MEASUREMENT_SWARM_BOTS_LAST_SEEN   = "swarming_bots_last_seen"
	MEASUREMENT_SWARM_BOTS_QUARANTINED = "swarming_bots_quarantined"
	MEASUREMENT_SWARM_BOTS_LAST_TASK   = "swarming_bots_last_task"
)

// cleanupOldMetrics deletes old metrics, replace with new ones. This fixes the case where
// bots are removed but their metrics hang around, or where dimensions
// change resulting in duplicate metrics with the same bot ID.
func cleanupOldMetrics(oldMetrics []metrics2.Int64Metric) []metrics2.Int64Metric {
	failedDelete := []metrics2.Int64Metric{}
	for _, m := range oldMetrics {
		if err := m.Delete(); err != nil {
			sklog.Warningf("Failed to delete metric: %s", err)
			failedDelete = append(failedDelete, m)
		}
	}
	return failedDelete
}

// reportBotMetrics reports information about the bots in the given pool
// to the metrics client. This includes if the bot is quarantined and
// how long ago we saw the bot.
func reportBotMetrics(now time.Time, client swarming.ApiClient, metricsClient metrics2.Client, pool, server string) ([]metrics2.Int64Metric, error) {
	sklog.Infof("Loading Swarming bot data for pool %s", pool)
	bots, err := client.ListBotsForPool(pool)
	if err != nil {
		return nil, fmt.Errorf("Could not get list of bots for pool %s: %s", pool, err)
	}

	newMetrics := []metrics2.Int64Metric{}
	for _, bot := range bots {
		last, err := time.Parse("2006-01-02T15:04:05", bot.LastSeenTs)
		if err != nil {
			sklog.Errorf("Malformed last seen time in bot: %s", err)
			continue
		}

		tags := map[string]string{
			"bot":      bot.BotId,
			"pool":     pool,
			"swarming": server,
		}

		// Bot last seen <duration> ago.
		m1 := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_LAST_SEEN, tags)
		m1.Update(int64(now.Sub(last)))
		newMetrics = append(newMetrics, m1)

		// Bot quarantined status.
		quarantined := int64(0)
		if bot.Quarantined {
			quarantined = int64(1)
		}
		m2 := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_QUARANTINED, tags)
		m2.Update(quarantined)
		newMetrics = append(newMetrics, m2)

		// Last task performed <duration> ago
		lastTasks, err := client.ListBotTasks(bot.BotId, 1)
		if err != nil {
			sklog.Errorf("Problem getting tasks that bot %s has run: %s", bot.BotId, err)
			continue
		}
		ts := ""
		if len(lastTasks) == 0 {
			ts = bot.FirstSeenTs
		} else {
			ts = lastTasks[0].ModifiedTs
		}

		last, err = time.Parse("2006-01-02T15:04:05", ts)
		if err != nil {
			sklog.Errorf("Malformed last modified time in bot %s's last task %q: %s", bot.BotId, ts, err)
			continue
		}
		m3 := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_LAST_TASK, tags)
		m3.Update(int64(now.Sub(last)))
		newMetrics = append(newMetrics, m1)
	}
	return newMetrics, nil
}

// StartSwarmingBotMetrics spins up several go routines to begin reporting
// metrics every 2 minutes.
func StartSwarmingBotMetrics(swarmingClients map[string]swarming.ApiClient, swarmingPools map[string][]string, metricsClient metrics2.Client) {
	for swarmingServer, client := range swarmingClients {
		for _, pool := range swarmingPools[swarmingServer] {
			go func(server, pool string, client swarming.ApiClient) {
				oldMetrics := []metrics2.Int64Metric{}
				for range time.Tick(2 * time.Minute) {
					oldMetrics = cleanupOldMetrics(oldMetrics)
					newMetrics, err := reportBotMetrics(time.Now(), client, metricsClient, pool, server)
					if err != nil {
						sklog.Error(err)
						continue
					}
					oldMetrics = append(oldMetrics, newMetrics...)
				}
			}(swarmingServer, pool, client)
		}
	}
}
