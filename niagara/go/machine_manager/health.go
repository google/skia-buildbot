package machine_manager

import (
	"math/rand"
	"time"

	"go.skia.org/infra/niagara/go/machine"
)

const (
	beginRebootingUptime = 12 * time.Hour
	forSureRebootUptime  = 14 * time.Hour
)

func checkHealth(desc machine.Description) (machine.Status, machine.StatusReason) {
	if !uptimeOk(desc.Uptime) {
		return machine.Quarantined, machine.ExcessiveUptimeReason
	}
	return machine.Ready, machine.NoReason
}

func uptimeOk(duration time.Duration) bool {
	if duration < beginRebootingUptime {
		return true
	}
	if duration > forSureRebootUptime {
		return false
	}
	// Do a probabilistic reboot to avoid too many machines rebooting at once. A simple linear
	// equation is good enough, unless evidence to the contrary is seen in production.
	p := float32(duration - beginRebootingUptime)
	p /= float32(forSureRebootUptime - beginRebootingUptime)
	return p < rand.Float32()
}
