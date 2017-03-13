package main

import (
	"fmt"
	"sort"

	"github.com/BurntSushi/toml"
)

type Config struct {
	MPower     map[string]*MPowerConfig     `toml:"mpower"`
	EdgeSwitch map[string]*EdgeSwitchConfig `toml:"edgeswitch"`
}

type aggregatedDevGroup struct {
	idDevGroupMap map[string]DeviceGroup // Maps from a deviceID to a device group.
}

//
func (a *aggregatedDevGroup) add(devGroup DeviceGroup) error {
	for _, id := range devGroup.DeviceIDs() {
		if _, ok := a.idDevGroupMap[id]; ok {
			return fmt.Errorf("Device '%s' already exists.", id)
		}
		a.idDevGroupMap[id] = devGroup
	}
	return nil
}

func (a *aggregatedDevGroup) DeviceIDs() []string {
	ret := make([]string, 0, len(a.idDevGroupMap))
	for id := range a.idDevGroupMap {
		ret = append(ret, id)
	}
	sort.Strings(ret)
	return ret
}

func (a *aggregatedDevGroup) PowerCycle(devID string) error {
	dev, ok := a.idDevGroupMap[devID]
	if !ok {
		return fmt.Errorf("Unknown device id: %s", devID)
	}
	return dev.PowerCycle(devID)
}

// DeviceGroupFromTomlFile parses a TOML file and instantiates the
// defined devices.
func DeviceGroupFromTomlFile(path string) (DeviceGroup, error) {
	conf := &Config{}
	if _, err := toml.DecodeFile(path, conf); err != nil {
		return nil, err
	}

	ret := &aggregatedDevGroup{
		idDevGroupMap: map[string]DeviceGroup{},
	}
	// Add the mpower devices.
	for _, c := range conf.MPower {
		mp, err := NewMPowerClient(c)
		if err != nil {
			return nil, err
		}

		if err := ret.add(mp); err != nil {
			return nil, err
		}
	}

	// Add the EdgeSwitch devices.
	for _, c := range conf.EdgeSwitch {
		es, err := NewEdgeSwitchClient(c)
		if err != nil {
			return nil, err
		}

		if err := ret.add(es); err != nil {
			return nil, err
		}
	}

	return ret, nil
}
