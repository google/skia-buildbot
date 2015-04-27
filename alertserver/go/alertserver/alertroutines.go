package main

import (
	"fmt"
	"regexp"
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

func StartAlertRoutines(am *alerting.AlertManager, tickInterval time.Duration, c *client.Client) {
	// Disconnected buildslaves.
	go func() {
		seriesTmpl := "buildbot.buildslaves.%s.connected"
		re := regexp.MustCompile("[^A-Za-z0-9]+")
		emailAction, err := alerting.ParseAction("Email(infra-alerts@skia.org)")
		if err != nil {
			glog.Fatal(err)
		}
		actions := []alerting.Action{emailAction}
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
}
