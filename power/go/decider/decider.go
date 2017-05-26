package decider

import (
	"encoding/json"
	"fmt"

	swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/skolo/go/powercycle"
)

// The BotDecider interface abstracts away a configuration file or similar which indicates which bots have powercyclable devices attached and which are golo machines, etc
type Decider interface {
	ShouldPowercycleBot(*swarming.SwarmingRpcsBotInfo) bool
	ShouldPowercycleDevice(*swarming.SwarmingRpcsBotInfo) bool
}

type decider struct {
	enabledBots util.StringSet
}

func New(powercycleConfigFile string) (Decider, error) {
	dg, err := powercycle.DeviceGroupFromYamlFile(powercycleConfigFile, false)
	if err != nil {
		return nil, err
	}
	sklog.Infof("Found ids %q", dg.DeviceIDs())
	return &decider{enabledBots: util.NewStringSet(dg.DeviceIDs())}, nil
}

func (d *decider) ShouldPowercycleBot(bot *swarming.SwarmingRpcsBotInfo) bool {
	return bot.IsDead && !bot.Deleted && d.checkEnabled(bot.BotId)
}

type state struct {
	// We don't have to enumerate all of the components of state, since we only care
	// about the state.devices[foo].state field
	Devices map[string]map[string]interface{} `json:"devices"`
}

func (d *decider) ShouldPowercycleDevice(bot *swarming.SwarmingRpcsBotInfo) bool {
	if !bot.Quarantined || bot.IsDead || bot.Deleted || !d.checkEnabled(TransformBotIDToDevice(bot.BotId)) {
		return false
	}
	// bot.State is a JSON string.  Among other things, it has a map of devices.
	// This map maps device id -> a device object, which contains a state/status.
	// We check the status to double check if we should try powercycling the device.
	// For example, we don't powercycle devices that are too hot, but do powercycle
	// devices that have a "usb_failure".
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
		} else if ok && status == "usb_failure" {
			return true
		}
		break
	}
	// Possibly has more than one device attached or a state we can't handle
	return false
}

func (d *decider) checkEnabled(id string) bool {
	return d.enabledBots[id]
}

func TransformBotIDToDevice(id string) string {
	return id + "-device"
}
