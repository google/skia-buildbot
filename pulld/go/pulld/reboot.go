package main

import (
	"os"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/metrics2"
)

// rebootMonitoringInit checks once a minute if the machine needs to be
// rebooted and stuffs that information into influx.
func rebootMonitoringInit() {
	name, err := os.Hostname()
	if err != nil {
		glog.Errorf("Failed to get hostname: %s", err)
		return
	}
	reboot := metrics2.GetInt64Metric("reboot-required-i", map[string]string{"host": name})
	go func() {
		for _ = range time.Tick(time.Minute) {
			_, err := os.Stat("/var/run/reboot-required")
			if err == nil {
				reboot.Update(1)
			} else {
				reboot.Update(0)
			}
		}
	}()
}
