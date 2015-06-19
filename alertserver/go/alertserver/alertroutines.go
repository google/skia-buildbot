package main

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"github.com/skia-dev/influxdb/client"
	"go.skia.org/infra/alertserver/go/alerting"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/util"
)

/*
	This file contains goroutines which trigger more complex alerts than
	can be expressed using the rule format in alerts.cfg.
*/

const (
	ANDROID_DISCONNECT = `The Android device for %s appears to be disconnected.

Build: https://uberchromegw.corp.google.com/i/%s/builders/%s/builds/%d

Host info: https://status.skia.org/hosts?filter=%s`
	AUTOROLL_ALERT_NAME = "AutoRoll Failed"
)

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
		// Don't generate alerts for these buildslaves, since they always fail.
		buildslaveBlacklist := []string{
			"build4-a3",
			"build5-a3",
			"build5-m3",
			"skiabot-shuttle-ubuntu12-galaxys3-001",
			"skiabot-shuttle-ubuntu12-galaxys3-002",
			"skiabot-shuttle-ubuntu12-galaxys4-001",
			"skiabot-shuttle-ubuntu12-galaxys4-002",
		}

		// Generate an alert if the failure rate exceeds the mean by
		// this many standard deviations,
		sigStdDevs := 0.5
		for _ = range time.Tick(tickInterval) {
			now := time.Now()
			start := now.Add(-24 * time.Hour)
			builds, err := buildbot.GetBuildsFromDateRange(start, now)
			if err != nil {
				glog.Error(err)
				continue
			}
			buildsByBuilder := map[string][]*buildbot.Build{}
			buildsBySlave := map[string][]*buildbot.Build{}
			for _, b := range builds {
				if _, ok := buildsByBuilder[b.Builder]; !ok {
					buildsByBuilder[b.Builder] = []*buildbot.Build{}
				}
				buildsByBuilder[b.Builder] = append(buildsByBuilder[b.Builder], b)
				if _, ok := buildsBySlave[b.BuildSlave]; !ok {
					buildsBySlave[b.BuildSlave] = []*buildbot.Build{}
				}
				buildsBySlave[b.BuildSlave] = append(buildsBySlave[b.BuildSlave], b)
			}
			failuresByBuilder := map[string]int{}
			for builder, builds := range buildsByBuilder {
				failures := 0
				for _, b := range builds {
					if b.Results == 0 || b.Results == 1 || b.Results == 3 {
						// Success | Warnings | Skipped
					} else {
						// Failure | Exception | Retry
						failures++
					}
				}
				failuresByBuilder[builder] = failures
			}
			for slave, b := range buildsBySlave {
				if util.In(slave, buildslaveBlacklist) {
					glog.Warningf("Skipping blacklisted buildslave %s", slave)
					continue
				}

				// We can assume there's at least one build for the slave, and
				// we can assume that the slave only connects to one master.
				master := b[0].Master

				// Loop through the builds:
				// 1. Calculate the failure rate for this slave.
				// 2. Calculate a mean failure rate: the combined
				//    failure rate of all builders that this slave ran.
				builders := map[string]bool{}
				failed := 0
				for _, build := range b {
					if build.Results != 0 {
						failed++
					}
					builders[build.Builder] = true
				}
				failureRate := float64(failed) / float64(len(b))
				if failureRate == 0 {
					continue
				}
				failedOnAllBuilders := 0
				ranOnAllBuilders := 0
				for builder, _ := range builders {
					ranOnAllBuilders += len(buildsByBuilder[builder])
					failedOnAllBuilders += failuresByBuilder[builder]
				}
				meanFailureRate := float64(failedOnAllBuilders) / float64(ranOnAllBuilders)

				// Calculate the standard deviation.
				sumSquares := float64(0.0)
				// (val - mean)^2 for failures.
				f := float64(1.0) - meanFailureRate
				f = f * f
				// (val - mean)^2 for successes.
				s := float64(0.0) - meanFailureRate
				s = s * s
				for builder, _ := range builders {
					sumSquares += f * float64(failuresByBuilder[builder])
					sumSquares += s * float64(len(buildsByBuilder[builder])-failuresByBuilder[builder])
				}
				stddev := math.Sqrt(sumSquares / float64(ranOnAllBuilders))

				threshold := meanFailureRate + sigStdDevs*stddev
				if failureRate > threshold {
					if err := am.AddAlert(&alerting.Alert{
						Name:        fmt.Sprintf("Buildslave %s failure rate is too high", slave),
						Category:    alerting.INFRA_ALERT,
						Message:     fmt.Sprintf("Buildslave %s failure rate (%f) is significantly higher than the average failure rate of the builders it runs. Mean: %f StdDev: %f .Significance defined as %f standard deviations for a threshold of %f. https://uberchromegw.corp.google.com/i/%s/buildslaves/%s", slave, failureRate, meanFailureRate, stddev, sigStdDevs, threshold, master, slave),
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

	// AutoRoll failure.
	go func() {
		lastSearch := time.Now()
		for now := range time.Tick(time.Minute) {
			glog.Infof("Searching for DEPS rolls.")
			results, err := autoroll.GetRecentRolls(lastSearch)
			if err != nil {
				glog.Errorf("Failed to search for DEPS rolls: %v", err)
				continue
			}
			lastSearch = now
			activeAlert := am.ActiveAlert(AUTOROLL_ALERT_NAME)
			for _, issue := range results {
				if issue.Closed {
					if issue.Committed {
						if activeAlert != 0 {
							msg := fmt.Sprintf("Subsequent roll succeeded: %s/%d", autoroll.RIETVELD_URL, issue.Issue)
							if err := am.Dismiss(activeAlert, alerting.USER_ALERTSERVER, msg); err != nil {
								glog.Error(err)
							}
						}
					} else {
						if err := am.AddAlert(&alerting.Alert{
							Name:    AUTOROLL_ALERT_NAME,
							Message: fmt.Sprintf("DEPS roll failed: %s/%d", autoroll.RIETVELD_URL, issue.Issue),
							Nag:     int64(3 * time.Hour),
							Actions: actions,
						}); err != nil {
							glog.Error(err)
						}
					}
				}
			}
		}
	}()

	// Android device disconnects.
	go func() {
		for _ = range time.Tick(tickInterval) {
			glog.Infof("Searching for disconnected Android devices.")
			builds, err := buildbot.GetUnfinishedBuilds()
			if err != nil {
				glog.Error(err)
				continue
			}
			for _, b := range builds {
				if strings.Contains(b.Builder, "Android") && !strings.Contains(b.Builder, "Build") {
					for _, s := range b.Steps {
						if strings.Contains(s.Name, "wait for device") {
							// If "wait for device" has been running for 10 minutes, the device is probably offline.
							if s.Finished == 0 && time.Since(time.Unix(int64(s.Started), 0)) > 10*time.Minute {
								if err := am.AddAlert(&alerting.Alert{
									Name:     fmt.Sprintf("Android device disconnected (%s)", b.BuildSlave),
									Category: alerting.INFRA_ALERT,
									Message:  fmt.Sprintf(ANDROID_DISCONNECT, b.BuildSlave, b.Master, b.Builder, b.Number, b.BuildSlave),
									Nag:      int64(3 * time.Hour),
									Actions:  actions,
								}); err != nil {
									glog.Error(err)
								}
							}
						}
					}
				}
			}
		}
	}()

}
