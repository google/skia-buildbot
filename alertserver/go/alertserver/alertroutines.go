package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/alertserver/go/alerting"
	"go.skia.org/infra/autoroll/go/autoroller"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/util"
)

/*
	This file contains goroutines which trigger more complex alerts than
	can be expressed using the rule format in alerts.cfg.
*/

const (
	ANDROID_DISCONNECT = `The Android device for %s appears to be disconnected.

Build: https://uberchromegw.corp.google.com/i/%s/builders/%s/builds/%d
Dashboard: https://status.skia.org/buildbots?botGrouping=buildslave&filterBy=buildslave&include=%%5E%s%%24&tab=builds
Host info: https://status.skia.org/hosts?filter=%s`
	// Allow up to 30 minutes for the device to charge. If "wait for device" has been running for longer than that, the device is probably offline.
	ANDROID_DISCONNECT_TIME_LIMIT = 30 * time.Minute
	AUTOROLL_ALERT_NAME           = "AutoRoll Failed"
	BUILDSLAVE_OFFLINE            = `Buildslave %s is not connected to https://uberchromegw.corp.google.com/i/%s/buildslaves/%s

Dashboard: https://status.skia.org/buildbots?botGrouping=buildslave&filterBy=buildslave&include=%%5E%s%%24&tab=builds
Host info: https://status.skia.org/hosts?filter=%s`
	HUNG_BUILDSLAVE = `Possibly hung buildslave (%s)

A step has been running for over %s:
https://uberchromegw.corp.google.com/i/%s/builders/%s/builds/%d
Dashboard: https://status.skia.org/buildbots?botGrouping=buildslave&filterBy=buildslave&include=%%5E%s%%24&tab=builds
Host info: https://status.skia.org/hosts?filter=%s`
	UPDATE_SCRIPTS = `update_scripts failed on %s

Build: https://uberchromegw.corp.google.com/i/%s/builders/%s/builds/%d
Dashboard: https://status.skia.org/buildbots?botGrouping=builder&filterBy=builder&include=%%5E%s%%24&tab=builds
Host info: https://status.skia.org/hosts?filter=%s`
)

var BUILDSLAVE_OFFLINE_BLACKLIST = []string{
	"build3-a3",
	"build4-a3",
	"vm255-m3",
}

type BuildSlice []*buildbot.Build

func (s BuildSlice) Len() int {
	return len(s)
}

func (s BuildSlice) Less(i, j int) bool {
	return s[j].Finished.After(s[i].Finished)
}

func (s BuildSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func StartAlertRoutines(am *alerting.AlertManager, tickInterval time.Duration, c *influxdb.Client, db buildbot.DB) {
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
					if util.In(s.Name, BUILDSLAVE_OFFLINE_BLACKLIST) {
						continue
					}
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
		getDepsRollStatus := func() (*autoroller.AutoRollStatus, error) {
			resp, err := http.Get(autoroll.AUTOROLL_STATUS_URL)
			if err != nil {
				return nil, err
			}

			defer util.Close(resp.Body)
			var status autoroller.AutoRollStatus
			if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
				return nil, err
			}
			return &status, nil
		}

		for _ = range time.Tick(time.Minute) {
			glog.Infof("Searching for DEPS rolls.")
			status, err := getDepsRollStatus()
			if err != nil {
				util.LogErr(fmt.Errorf("Failed to search for DEPS rolls: %v", err))
				continue
			}

			activeAlert := am.ActiveAlert(AUTOROLL_ALERT_NAME)
			if status.LastRoll != nil {
				if status.LastRoll.Closed {
					if status.LastRoll.Succeeded() {
						if activeAlert != 0 {
							msg := fmt.Sprintf("Subsequent roll succeeded: %s/%d", autoroll.RIETVELD_URL, status.LastRoll.Issue)
							if err := am.Dismiss(activeAlert, alerting.USER_ALERTSERVER, msg); err != nil {
								util.LogErr(err)
							}
						}
					} else if status.LastRoll.Failed() {
						if err := am.AddAlert(&alerting.Alert{
							Name:    AUTOROLL_ALERT_NAME,
							Message: fmt.Sprintf("DEPS roll failed: %s/%d", autoroll.RIETVELD_URL, status.LastRoll.Issue),
							Nag:     int64(3 * time.Hour),
							Actions: actions,
						}); err != nil {
							util.LogErr(err)
						}
					}
				}
			}
		}
	}()

	// Android device disconnects, hung buildslaves.
	go func() {
		// These builders are frequently slow. Ignore them when looking for hung buildslaves.
		hungSlavesIgnore := []string{
			"Housekeeper-Nightly-RecreateSKPs_Canary",
			"Housekeeper-Weekly-RecreateSKPs",
			"Linux Builder",
			"Mac Builder",
			"Test-Ubuntu-GCC-GCE-CPU-AVX2-x86_64-Release-Valgrind",
			"Test-Ubuntu-GCC-ShuttleA-GPU-GTX550Ti-x86_64-Release-Valgrind",
			"Win Builder",
		}
		hangTimePeriod := 3 * time.Hour
		for _ = range time.Tick(tickInterval) {
			glog.Infof("Searching for hung buildslaves and disconnected Android devices.")
			builds := []*buildbot.Build{}
			for _, m := range buildbot.MASTER_NAMES {
				b, err := db.GetUnfinishedBuilds(m)
				if err != nil {
					glog.Error(err)
					continue
				}
				builds = append(builds, b...)
			}
			for _, b := range builds {
				// Disconnected Android device?
				disconnectedAndroid := false
				if strings.Contains(b.Builder, "Android") && !strings.Contains(b.Builder, "Build") {
					for _, s := range b.Steps {
						if strings.Contains(s.Name, "wait for device") {
							if !s.IsFinished() && time.Since(s.Started) > ANDROID_DISCONNECT_TIME_LIMIT {
								if err := am.AddAlert(&alerting.Alert{
									Name:     fmt.Sprintf("Android device disconnected (%s)", b.BuildSlave),
									Category: alerting.INFRA_ALERT,
									Message:  fmt.Sprintf(ANDROID_DISCONNECT, b.BuildSlave, b.Master, b.Builder, b.Number, b.BuildSlave, b.BuildSlave),
									Nag:      int64(3 * time.Hour),
									Actions:  actions,
								}); err != nil {
									glog.Error(err)
								}
								disconnectedAndroid = true
							}
						}
					}
				}
				if !disconnectedAndroid && !util.ContainsAny(b.Builder, hungSlavesIgnore) {
					// Hung buildslave?
					for _, s := range b.Steps {
						if s.Name == "steps" {
							continue
						}
						// If the step has been running for over an hour, it's probably hung.
						if !s.IsFinished() && time.Since(s.Started) > hangTimePeriod {
							if err := am.AddAlert(&alerting.Alert{
								Name:        fmt.Sprintf("Possibly hung buildslave (%s)", b.BuildSlave),
								Category:    alerting.INFRA_ALERT,
								Message:     fmt.Sprintf(HUNG_BUILDSLAVE, b.BuildSlave, hangTimePeriod.String(), b.Master, b.Builder, b.Number, b.BuildSlave, b.BuildSlave),
								Nag:         int64(time.Hour),
								Actions:     actions,
								AutoDismiss: int64(10 * tickInterval),
							}); err != nil {
								glog.Error(err)
							}
						}
					}
				}
			}
		}
	}()

	// Failed update_scripts.
	go func() {
		lastSearch := time.Now()
		for _ = range time.Tick(tickInterval) {
			glog.Infof("Searching for builds which failed update_scripts.")
			currentSearch := time.Now()
			builds, err := db.GetBuildsFromDateRange(lastSearch, currentSearch)
			lastSearch = currentSearch
			if err != nil {
				glog.Error(err)
				continue
			}
			for _, b := range builds {
				for _, s := range b.Steps {
					if s.Name == "update_scripts" {
						if s.Results != 0 {
							if err := am.AddAlert(&alerting.Alert{
								Name:     "update_scripts failed",
								Category: alerting.INFRA_ALERT,
								Message:  fmt.Sprintf(UPDATE_SCRIPTS, b.Builder, b.Master, b.Builder, b.Number, b.Builder, b.BuildSlave),
								Actions:  actions,
							}); err != nil {
								glog.Error(err)
							}
						}
						break
					}
				}
			}
		}
	}()
}
