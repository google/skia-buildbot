package main

import (
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"github.com/skia-dev/influxdb/client"
	"go.skia.org/infra/alertserver/go/alerting"
	"go.skia.org/infra/go/buildbot"
)

/*
	This file contains goroutines which trigger more complex alerts than
	can be expressed using the rule format in alerts.cfg.
*/

type BuildSlice []*buildbot.Build

func (s BuildSlice) Len() int {
	return len(s)
}

func (s BuildSlice) Less(i, j int) bool {
	return s[i].Finished < s[j].Finished
}

func (s BuildSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func StartAlertRoutines(am *alerting.AlertManager, tickInterval time.Duration, c *client.Client) {
	emailAction, err := alerting.ParseAction("Email(infra-alerts@skia.org)")
	if err != nil {
		glog.Fatal(err)
	}
	actions := []alerting.Action{emailAction}

	// Disconnected buildslaves.
	go func() {
		seriesTmpl := "buildbot.buildslaves.%s.connected"
		re := regexp.MustCompile("[^A-Za-z0-9]+")
		for _ = range time.Tick(tickInterval) {
			glog.Info("Loading buildslave data.")
			slaves, err := buildbot.GetBuildSlaves()
			if err != nil {
				glog.Error(err)
				continue
			}
			for masterName, m := range slaves {
				for _, s := range m {
					v := int64(0)
					if s.Connected {
						v = int64(1)
					}
					metric := fmt.Sprintf(seriesTmpl, re.ReplaceAllString(s.Name, "_"))
					metrics.GetOrRegisterGauge(metric, metrics.DefaultRegistry).Update(v)
					if !s.Connected {
						// This buildslave is offline. Figure out which one it is.
						if err := am.AddAlert(&alerting.Alert{
							Name:        fmt.Sprintf("Buildslave %s offline", s.Name),
							Category:    alerting.INFRA_ALERT,
							Message:     fmt.Sprintf("Buildslave %s is not connected to https://uberchromegw.corp.google.com/i/%s/buildslaves/%s", s.Name, masterName, s.Name),
							Nag:         int64(time.Hour),
							AutoDismiss: int64(2 * tickInterval),
							Actions:     actions,
						}); err != nil {
							glog.Error(err)
						}
					}
				}
			}
		}
	}()

	// Alerts for buildbot data.
	go func() {
		failedInARowThreshold := 3
		failureRateThreshold := 0.5
		for _ = range time.Tick(tickInterval) {
			now := time.Now()
			start := now.Add(-24 * time.Hour)
			builds, err := buildbot.GetBuildsFromDateRange(start, now)
			if err != nil {
				glog.Error(err)
				continue
			}
			buildsBySlave := map[string][]*buildbot.Build{}
			for _, b := range builds {
				if _, ok := buildsBySlave[b.BuildSlave]; !ok {
					buildsBySlave[b.BuildSlave] = []*buildbot.Build{}
				}
				buildsBySlave[b.BuildSlave] = append(buildsBySlave[b.BuildSlave], b)
			}
			for slave, b := range buildsBySlave {
				// We can assume there's at least one build for the slave, and
				// we can assume that the slave only connects to one master.
				master := b[0].Master

				// Sort by finish time.
				sort.Sort(BuildSlice(b))

				// N failures in a row.
				overThreshold := true
				if len(b) >= failedInARowThreshold {
					for i := 0; i < failedInARowThreshold; i++ {
						if b[len(b)-i-1].Results == 0 {
							overThreshold = false
							break
						}
					}
				}
				if overThreshold {
					if err := am.AddAlert(&alerting.Alert{
						Name:        fmt.Sprintf("Buildslave %s failed %d times", slave, failedInARowThreshold),
						Category:    alerting.INFRA_ALERT,
						Message:     fmt.Sprintf("Buildslave %s has failed its last %d builds: https://uberchromegw.corp.google.com/i/%s/buildslaves/%s", slave, failedInARowThreshold, master, slave),
						Nag:         int64(3 * time.Hour),
						AutoDismiss: int64(2 * tickInterval),
						Actions:     actions,
					}); err != nil {
						glog.Error(err)
					}
				}

				// Failure rate > k.
				failed := 0
				for _, build := range b {
					if build.Results != 0 {
						failed++
					}
				}
				failureRate := float64(failed) / float64(len(b))
				if failureRate > failureRateThreshold {
					if err := am.AddAlert(&alerting.Alert{
						Name:        fmt.Sprintf("Buildslave %s failure rate > %f", slave, failureRateThreshold),
						Category:    alerting.INFRA_ALERT,
						Message:     fmt.Sprintf("Buildslave %s failure rate exceeds %f (%f): https://uberchromegw.corp.google.com/i/%s/buildslaves/%s", slave, failureRateThreshold, failureRate, master, slave),
						Nag:         int64(3 * time.Hour),
						AutoDismiss: int64(2 * tickInterval),
						Actions:     actions,
					}); err != nil {
						glog.Error(err)
					}
				}
			}
		}
	}()
}
