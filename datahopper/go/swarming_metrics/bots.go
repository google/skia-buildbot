package swarming_metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
)

const (
	MEASUREMENT_SWARM_BOTS_BUSY        = "swarming_bots_busy"
	MEASUREMENT_SWARM_BOTS_LAST_SEEN   = "swarming_bots_last_seen"
	MEASUREMENT_SWARM_BOTS_QUARANTINED = "swarming_bots_quarantined"
	MEASUREMENT_SWARM_BOTS_LAST_TASK   = "swarming_bots_last_task"
	MEASUREMENT_SWARM_BOTS_DEVICE_TEMP = "swarming_bots_device_temp"
	MEASUREMENT_SWARM_BOTS_UPTIME      = "swarming_bots_uptime_s"
)

var ignoreBatteries = []*regexp.Regexp{
	// max77621-(cpu|gpu) are on the pixel Cs and constantly at 100. Not useful
	regexp.MustCompile("max77621-(cpu|gpu)"),
	// dram is on the Nexus players and goes between 0 and 2.
	regexp.MustCompile("dram"),
}

var device_state_guages = []string{"too_hot", "low_battery", "available", "<none>"}

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
func reportBotMetrics(ctx context.Context, now time.Time, client swarming.ApiClient, metricsClient metrics2.Client, pool, server string) ([]metrics2.Int64Metric, error) {
	sklog.Infof("Loading Swarming bot data for pool %s", pool)
	bots, err := client.ListBotsForPool(ctx, pool)
	if err != nil {
		return nil, fmt.Errorf("Could not get list of bots for pool %s: %s", pool, err)
	}
	sklog.Infof("  Found %d bots in pool %s", len(bots), pool)

	newMetrics := []metrics2.Int64Metric{}
	for _, bot := range bots {
		last, err := time.Parse("2006-01-02T15:04:05", bot.LastSeenTs)
		if err != nil {
			sklog.Errorf("Malformed last seen time in bot: %s", err)
			continue
		}

		var deviceOs string
		var deviceType string
		var os string
		var quarantined string
		for _, d := range bot.Dimensions {
			val := d.Value[len(d.Value)-1]
			if d.Key == swarming.DIMENSION_DEVICE_OS_KEY {
				deviceOs = val
			} else if d.Key == swarming.DIMENSION_DEVICE_TYPE_KEY {
				deviceType = val
			} else if d.Key == swarming.DIMENSION_OS_KEY {
				os = val
			} else if d.Key == swarming.DIMENSION_QUARANTINED_KEY {
				quarantined = val
			}
		}
		tags := map[string]string{
			"bot":                              bot.BotId,
			"pool":                             pool,
			"swarming":                         server,
			swarming.DIMENSION_OS_KEY:          os,
			swarming.DIMENSION_DEVICE_TYPE_KEY: deviceType,
			swarming.DIMENSION_DEVICE_OS_KEY:   deviceOs,
			swarming.DIMENSION_QUARANTINED_KEY: quarantined,
		}

		currDeviceState := "<none>"

		if bot.State != "" {
			st := botState{}
			if err := json.Unmarshal([]byte(bot.State), &st); err != nil {
				sklog.Errorf("Malformed bot state %q: %s", bot.State, err)
				continue
			}
			deviceStates := []string{}
			for _, device := range st.DeviceMap {
				deviceStates = append(deviceStates, device.State)
			}
			// This should always be length 0 or 1 because Skia infra is set up for
			// one host to one device. If that device is missing (or there are none),
			// device_states may be length 0, otherwise it should be length 1.
			if len(deviceStates) > 0 && util.In(deviceStates[0], device_state_guages) {
				// Some common values include "available", "too_hot", "low_battery"
				currDeviceState = deviceStates[0]
			}
		}

		// Bot last seen <duration> ago.
		m1 := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_LAST_SEEN, tags)
		m1.Update(int64(now.Sub(last)))
		newMetrics = append(newMetrics, m1)

		for _, reason := range device_state_guages {
			// Bot quarantined status.  So we can differentiate the cause (e.g. if it's a
			// low_battery or too_hot), write everything else to 0.
			quarantinedTags := util.CopyStringMap(tags)
			quarantinedTags["device_state"] = reason

			quarantined := int64(0)
			if bot.Quarantined && reason == currDeviceState {
				quarantined = int64(1)
			}
			m2 := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_QUARANTINED, quarantinedTags)
			m2.Update(quarantined)
			newMetrics = append(newMetrics, m2)
		}

		// Last task performed <duration> ago
		lastTasks, err := client.ListBotTasks(ctx, bot.BotId, 1)
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
		newMetrics = append(newMetrics, m3)

		if bot.State != "" {
			st := botState{}
			if err := json.Unmarshal([]byte(bot.State), &st); err != nil {
				sklog.Errorf("Malformed bot state %q: %s", bot.State, err)
				continue
			}

			for zone, temp := range st.BotTemperatureMap {
				tempTags := util.CopyStringMap(tags)
				tempTags["temp_zone"] = zone
				m4 := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_DEVICE_TEMP, tempTags)
				// Round to nearest whole number
				m4.Update(int64(temp + 0.5))
				newMetrics = append(newMetrics, m4)
			}

			for _, device := range st.DeviceMap {
				if device.BatteryMap != nil {
					if t, ok := device.BatteryMap["temperature"]; ok {
						// Avoid conflicts if there's a "battery" in DevTemperatureMap
						tempTags := util.CopyStringMap(tags)
						tempTags["temp_zone"] = "battery_direct"
						m4 := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_DEVICE_TEMP, tempTags)
						// Round to nearest whole number, keeping in mind that the battery
						// temperature is given in tenths of a degree C
						temp, ok := t.(float64)
						if !ok {
							sklog.Errorf("Could not do type assertion of %q to a float64", t)
							temp = 0
						}
						m4.Update(int64(temp+5) / 10)
						newMetrics = append(newMetrics, m4)
					}
				}

			outer:
				for zone, temp := range device.DevTemperatureMap {
					for _, ignoreBattery := range ignoreBatteries {
						if ignoreBattery.MatchString(zone) {
							continue outer
						}
					}
					tempTags := util.CopyStringMap(tags)
					tempTags["temp_zone"] = zone
					m4 := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_DEVICE_TEMP, tempTags)
					if strings.HasPrefix(zone, "tsens_tz_sensor") && temp > 200 {
						// These sensors are sometimes in deci°C, so we divide by 10
						m4.Update(int64(temp+5) / 10)
					} else {
						// Round to nearest whole number
						m4.Update(int64(temp + 0.5))
					}

					newMetrics = append(newMetrics, m4)
				}
				break
			}

			uptimeMetric := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_UPTIME, tags)
			uptimeMetric.Update(st.UptimeSeconds)
			newMetrics = append(newMetrics, uptimeMetric)

		}

		// Bot is currently busy/idle.
		m4 := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_BUSY, tags)
		busy := int64(0)
		if bot.TaskId != "" {
			busy = int64(1)
		}
		m4.Update(busy)
		newMetrics = append(newMetrics, m4)
	}
	return newMetrics, nil
}

type botState struct {
	BotTemperatureMap map[string]float32       `json:"temp"`
	DeviceMap         map[string]androidDevice `json:"devices"`
	UptimeSeconds     int64                    `json:"uptime"`
}

type androidDevice struct {
	// BatteryMap can map to either numbers or array of numbers, so we use interface{} and
	// do a type assertion above.
	BatteryMap        map[string]interface{} `json:"battery"`
	DevTemperatureMap map[string]float32     `json:"temp"`
	State             string                 `json:"state"`
}

// StartSwarmingBotMetrics spins up several go routines to begin reporting
// metrics every 2 minutes.
func StartSwarmingBotMetrics(ctx context.Context, swarmingServer string, swarmingPools []string, client swarming.ApiClient, metricsClient metrics2.Client) {
	for _, pool := range swarmingPools {
		pool := pool
		lvReportBotMetrics := metrics2.NewLiveness("last_successful_report_bot_metrics", map[string]string{
			"server": swarmingServer,
			"pool":   pool,
		})
		oldMetrics := map[metrics2.Int64Metric]struct{}{}
		go util.RepeatCtx(ctx, 2*time.Minute, func(ctx context.Context) {
			newMetrics, err := reportBotMetrics(ctx, time.Now(), client, metricsClient, pool, swarmingServer)
			if err != nil {
				sklog.Error(err)
				return
			}
			newMetricsMap := make(map[metrics2.Int64Metric]struct{}, len(newMetrics))
			for _, m := range newMetrics {
				newMetricsMap[m] = struct{}{}
			}
			var cleanup []metrics2.Int64Metric
			for m := range oldMetrics {
				if _, ok := newMetricsMap[m]; !ok {
					cleanup = append(cleanup, m)
				}
			}
			if len(cleanup) > 0 {
				failedDelete := cleanupOldMetrics(cleanup)
				for _, m := range failedDelete {
					newMetricsMap[m] = struct{}{}
				}
			}
			oldMetrics = newMetricsMap
			lvReportBotMetrics.Reset()
		})
	}
}
