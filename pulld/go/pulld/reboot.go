package main

import (
	"os"
	"time"

	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

const (
	// REBOOT_TARGET_TIME_OF_DAY is duration after midnight UTC.
	REBOOT_TARGET_TIME_OF_DAY = 5 * time.Hour
	// REBOOT_TARGET_DURATION indicates a reboot may occur anytime between
	// REBOOT_TARGET_TIME_OF_DAY and REBOOT_TARGET_TIME_OF_DAY + REBOOT_TARGET_DURATION.
	REBOOT_TARGET_DURATION = 1 * time.Hour
)

// rebootMonitoringInit checks once a minute if the machine needs to be
// rebooted and stuffs that information into influx.
func rebootMonitoringInit() {
	name, err := os.Hostname()
	if err != nil {
		sklog.Errorf("Failed to get hostname: %s", err)
		return
	}
	primary := metadata.GetWithDefault("owner_primary", "UNKNOWN_OWNER")
	secondary := metadata.GetWithDefault("owner_secondary", "")
	auto := metadata.GetWithDefault("auto_reboot", "false")
	owners := primary
	if secondary != "" {
		owners += ", " + secondary
	}
	reboot := metrics2.GetInt64Metric("reboot-required-i", map[string]string{
		"host":   name,
		"owners": owners,
		"auto":   auto,
	})
	go func() {
		for _ = range time.Tick(time.Minute) {
			_, err := os.Stat("/var/run/reboot-required")
			if err == nil {
				reboot.Update(1)
				if auto == "true" {
					maybeReboot()
				}
			} else {
				reboot.Update(0)
			}
		}
	}()
}

// timeForReboot returns true if now is a good time to trigger an automatic reboot. Otherwise,
// returns false and the next good time for a reboot.
func timeForReboot(now time.Time) (bool, time.Time) {
	year, month, day := now.UTC().Date()
	targetRebootTime := time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Add(REBOOT_TARGET_TIME_OF_DAY)
	if now.Before(targetRebootTime) {
		return false, targetRebootTime
	} else if now.Before(targetRebootTime.Add(REBOOT_TARGET_DURATION)) {
		return true, targetRebootTime
	}
	return false, targetRebootTime.Add(24 * time.Hour)
}

// maybeReboot triggers an automatic reboot if now is a good time.
func maybeReboot() {
	rebootNow, schedTime := timeForReboot(time.Now())
	if rebootNow {
		sklog.Warningf("Reboot required. Triggering reboot now.")
		res, err := doChange("reboot.target", "start")
		if err != nil {
			sklog.Errorf("Could not auto-reboot: %v", err)
		} else {
			sklog.Warningf("Reboot %s", res.Result)
		}
	} else {
		sklog.Infof("Reboot required. Automatic reboot scheduled for %s", schedTime)
	}
}
