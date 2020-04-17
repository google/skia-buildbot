package decider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	swarming "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skolo/go/powercycle"
)

// The Decider interface abstracts away the logic to decide if a bot/device
// 1) is powercycleable and 2) should be powercycled
type Decider interface {
	// ShouldPowercycleBot returns true if the bot/host supports powercycling and is in a state that
	// would be fixed by powercycling.
	ShouldPowercycleBot(*swarming.SwarmingRpcsBotInfo) bool
	// ShouldPowercycleDevice returns true if the device supports powercycling and is in a state that
	// would be fixed by powercycling.
	ShouldPowercycleDevice(*swarming.SwarmingRpcsBotInfo) bool
}

// decider implements the Decider interface
type decider struct {
	enabledBots map[powercycle.DeviceID]bool
	hostMap     map[powercycle.DeviceID]string // maps id -> host
}

var json5FileMatcher = regexp.MustCompile(".+powercycle-(.+).json5")

// New creates a new Decider based off the powercycle configs. It will assume that
// only the bots listed in that config file are powercycleable.
// Additionally, it returns a map of deviceID -> jumphost it is on, which is
// derived from which config file declares the given device.
func New(powercycleConfigFiles []string) (Decider, map[powercycle.DeviceID]string, error) {
	hm := map[powercycle.DeviceID]string{}
	enabled := map[powercycle.DeviceID]bool{}
	for _, file := range powercycleConfigFiles {
		// We can pass context.Background() safely here because we set connect
		// to false, which means context won't be used.
		dg, err := powercycle.ControllerFromJSON5(context.Background(), file, false)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		for _, id := range dg.DeviceIDs() {
			enabled[id] = true
		}
		// Extract the jumphost name from the file name of the powercycle config file that
		// was passed in.
		if matches := json5FileMatcher.FindStringSubmatch(file); matches == nil {
			sklog.Warningf("The powercycle config files are expected to start with a powercycle- prefix and end in .json5.  %s does not", file)
		} else {
			hostname := "jumphost-" + matches[1]
			for _, d := range dg.DeviceIDs() {
				hm[d] = hostname
			}
		}

	}

	sklog.Infof("Derived hostmap: %#v", hm)

	return &decider{enabledBots: enabled}, hm, nil
}

// ShouldPowercycleBot implements the Decider interface.
func (d *decider) ShouldPowercycleBot(bot *swarming.SwarmingRpcsBotInfo) bool {
	// Deleted bots typically won't show up in a call to the BotList API, but
	// it doesn't hurt to be defensive.
	return bot.IsDead && !bot.Deleted && d.isEnabled(powercycle.DeviceID(bot.BotId))
}

// state represents a portion of the bot.State.  bot.State is given to us as a
// JSON string.  Among other things, it has a map of devices. This device
// map maps device id -> a device object, which contains a state/status.
// We check the status to double check if we should try powercycling the
// device. For example, we don't powercycle devices that are too hot, but do
// powercycle devices that have a "usb_failure".
type state struct {
	// We don't have to enumerate all of the components of state, since we only care
	// about the state.devices[foo].state field
	Devices map[string]map[string]interface{} `json:"devices"`
}

// ShouldPowercycleDevice implements the Decider interface.
func (d *decider) ShouldPowercycleDevice(bot *swarming.SwarmingRpcsBotInfo) bool {
	if !bot.Quarantined || bot.IsDead || bot.Deleted || !d.isEnabled(transformBotIDToDevice(bot.BotId)) {
		return false
	}

	s := state{}
	if err := json.Unmarshal([]byte(bot.State), &s); err != nil {
		fmt.Printf("Could not read bot state %s", err)
		return false
	}
	if len(s.Devices) == 0 {
		return true // no device attached and there should be, powercycle may fix it.
	}
	for _, dev := range s.Devices {
		if status, ok := dev["state"].(string); ok && status == "too_hot" {
			return false
		} else if ok && (status == "usb_failure" || strings.Contains(status, "booting")) {
			return true
		}
		break
	}
	// Possibly has more than one device attached or a state we can't handle
	return false
}

// isEnabled returns true if the bot or device id is supported for powercycling.
func (d *decider) isEnabled(id powercycle.DeviceID) bool {
	return d.enabledBots[id]
}

// transformBotIDToDevice transforms the bot id to a bot id with device, e.g.
// skia-foo -> skia-foo-device
func transformBotIDToDevice(id string) powercycle.DeviceID {
	return powercycle.DeviceID(id + "-device")
}
