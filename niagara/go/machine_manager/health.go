package machine_manager

import (
	"math/rand"
	"time"

	"go.skia.org/infra/niagara/go/fs_entries"
	"go.skia.org/infra/niagara/go/machine"
)

const (
	beginRebootingUptime = 12 * time.Hour
	forSureRebootUptime  = 14 * time.Hour
)

func checkHealth(desc machine.Description, prevMachine *fs_entries.Machine) (machine.Description, machine.Status, machine.StatusReason) {
	if prevMachine != nil {
		oldDevice := prevMachine.Description.DeviceAttached
		if oldDevice != "" && desc.DeviceAttached == "" {
			desc.DeviceAttached = oldDevice
			return desc, machine.Quarantined, machine.DeviceMissing
		}
		// TODO(kjlubick) check that the current device isn't hot and battery levels are good.
	}

	if !uptimeOk(desc.Uptime) {
		return desc, machine.Quarantined, machine.ExcessiveUptimeReason
	}
	return desc, machine.Ready, machine.NoReason
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
