package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"github.com/skia-dev/influxdb/client"
	"go.skia.org/infra/alertserver/go/alerting"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/buildbot"
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
	BUILDSLAVE_OFFLINE  = `Buildslave %s is not connected to https://uberchromegw.corp.google.com/i/%s/buildslaves/%s

Dashboard: https://status.skia.org/buildbots?botGrouping=buildslave&filterBy=buildslave&include=%%5E%s%%24&tab=builds
Host info: https://status.skia.org/hosts?filter=%s
`
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
							Message:     fmt.Sprintf(BUILDSLAVE_OFFLINE, s.Name, masterName, s.Name, s.Name, s.Name),
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
