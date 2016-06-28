package main

import (
	"os"
	"time"

	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

// rebootMonitoringInit checks once a minute if the machine needs to be
// rebooted and stuffs that information into influx.
func rebootMonitoringInit() {
	name, err := os.Hostname()
	if err != nil {
		sklog.Errorf("Failed to get hostname: %s", err)
		return
	}
	primary, err := metadata.Get("owner_primary")
	if err != nil {
		sklog.Errorf("Problem getting primary instance owner: %s", err)
		primary = "UNKNOWN_OWNER"
	}
	secondary, err := metadata.Get("owner_secondary")
	if err != nil {
		sklog.Warningf("Problem getting secondary instance owner.  There might not actually be one: %s", err)
		secondary = ""
	}
	owners := primary
	if secondary != "" {
		owners += ", " + secondary
	}
	reboot := metrics2.GetInt64Metric("reboot-required-i", map[string]string{
		"host":   name,
		"owners": owners,
	})
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
