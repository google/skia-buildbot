package swarming_metrics

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
)

const (
	MEASUREMENT_SWARM_BOTS_LAST_SEEN   = "swarming_bots_last_seen"
	MEASUREMENT_SWARM_BOTS_QUARANTINED = "swarming_bots_quarantined"
	MEASUREMENT_SWARM_BOTS_LAST_TASK   = "swarming_bots_last_task"
	MEASUREMENT_SWARM_BOTS_DEVICE_TEMP = "swarming_bots_device_temp"
)

var batteryBlacklist = []*regexp.Regexp{
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
func reportBotMetrics(now time.Time, client swarming.ApiClient, metricsClient metrics2.Client, pool, server string) ([]metrics2.Int64Metric, error) {
	sklog.Infof("Loading Swarming bot data for pool %s", pool)
	bots, err := client.ListBotsForPool(pool)
	if err != nil {
		return nil, fmt.Errorf("Could not get list of bots for pool %s: %s", pool, err)
	}

	newMetrics := []metrics2.Int64Metric{}
	for _, bot := range bots {
		last, err := time.Parse("2006-01-02T15:04:05", bot.LastSeenTs)
		if strings.HasPrefix(bot.BotId, "skia-gce-24") {
			sklog.Debugf("Bot %s said last seen TS was %s, we parsed it to %s", bot.BotId, bot.LastSeenTs, last)
		}
		if err != nil {
			sklog.Errorf("Malformed last seen time in bot: %s", err)
			continue
		}

		tags := map[string]string{
			"bot":      bot.BotId,
			"pool":     pool,
			"swarming": server,
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
			if len(deviceStates) > 0 {
				// Some common values include "available", "too_hot", "low_battery"
				currDeviceState = deviceStates[0]
			}
			if currDeviceState == "" {
				currDeviceState = "<none>"
			}
		}

		// Bot last seen <duration> ago.
		m1 := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_LAST_SEEN, tags)
		m1.Update(int64(now.Sub(last)))
		newMetrics = append(newMetrics, m1)

		for _, reason := range device_state_guages {
			// Bot quarantined status.  So we can differentiate the cause (e.g. if it's a
			// low_batery or too_hot), write everything else to 0.
			quarantinedTags := map[string]string{
				"bot":          bot.BotId,
				"pool":         pool,
				"swarming":     server,
				"device_state": reason,
			}

			quarantined := int64(0)
			if bot.Quarantined && reason == currDeviceState {
				quarantined = int64(1)
			}
			m2 := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_QUARANTINED, quarantinedTags)
			m2.Update(quarantined)
			newMetrics = append(newMetrics, m2)
		}

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
		newMetrics = append(newMetrics, m3)

		if bot.State != "" {
			st := botState{}
			if err := json.Unmarshal([]byte(bot.State), &st); err != nil {
				sklog.Errorf("Malformed bot state %q: %s", bot.State, err)
				continue
			}
			for zone, temp := range st.BotTemperatureMap {
				tags["temp_zone"] = zone
				m4 := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_DEVICE_TEMP, tags)
				// Round to nearest whole number
				m4.Update(int64(temp + 0.5))
				newMetrics = append(newMetrics, m4)
			}

			for _, device := range st.DeviceMap {
				if device.BatteryMap != nil {
					if t, ok := device.BatteryMap["temperature"]; ok {
						// Avoid conflicts if there's a "battery" in DevTemperatureMap
						tags["temp_zone"] = "battery_direct"
						m4 := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_DEVICE_TEMP, tags)
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
					for _, blacklisted := range batteryBlacklist {
						if blacklisted.MatchString(zone) {
							continue outer
						}
					}
					tags["temp_zone"] = zone
					m4 := metricsClient.GetInt64Metric(MEASUREMENT_SWARM_BOTS_DEVICE_TEMP, tags)
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
		}

	}
	return newMetrics, nil
}

type botState struct {
	BotTemperatureMap map[string]float32       `json:"temp"`
	DeviceMap         map[string]androidDevice `json:"devices"`
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
