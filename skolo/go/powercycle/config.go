package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"time"

	yaml "gopkg.in/yaml.v2"

	"go.skia.org/infra/go/sklog"
)

// Config is the overall structure to aggregate configuration options
// for different device types.
type Config struct {
	// MPower aggregates all mPower configurations.
	MPower map[string]*MPowerConfig `yaml:"mpower"`

	// EdgeSwitch aggregates all EdgeSwitch configurations.
	EdgeSwitch map[string]*EdgeSwitchConfig `yaml:"edgeswitch"`
}

// aggregatedDevGroup implements the DeviceGroup interface and allows
// to combine multiple device groups into one.
type aggregatedDevGroup struct {
	idDevGroupMap map[string]DeviceGroup // Maps from a deviceID to a device group.
}

// add adds a new device group.
func (a *aggregatedDevGroup) add(devGroup DeviceGroup) error {
	for _, id := range devGroup.DeviceIDs() {
		if _, ok := a.idDevGroupMap[id]; ok {
			return fmt.Errorf("Device '%s' already exists.", id)
		}
		a.idDevGroupMap[id] = devGroup
	}
	return nil
}

// DeviceIDs, see the DeviceGroup interface.
func (a *aggregatedDevGroup) DeviceIDs() []string {
	ret := make([]string, 0, len(a.idDevGroupMap))
	for id := range a.idDevGroupMap {
		ret = append(ret, id)
	}
	sort.Strings(ret)
	return ret
}

// PowerCycle, see the DeviceGroup interface.
func (a *aggregatedDevGroup) PowerCycle(devID string, delayOverride time.Duration) error {
	dev, ok := a.idDevGroupMap[devID]
	if !ok {
		return fmt.Errorf("Unknown device id: %s", devID)
	}
	return dev.PowerCycle(devID, delayOverride)
}

func (a *aggregatedDevGroup) PowerUsage() (*GroupPowerUsage, error) {
	ret := &GroupPowerUsage{
		TS: time.Now(),
	}
	ret.Stats = map[string]*PowerStat{}
	for _, dev := range a.idDevGroupMap {
		devStats, err := dev.PowerUsage()
		if err != nil {
			sklog.Errorf("Error getting power stats: %s", err)
			continue
		}
		for id, stat := range devStats.Stats {
			ret.Stats[id] = stat
		}
	}
	return ret, nil
}

// DeviceGroupFromYamlFile parses a TOML file and instantiates the
// defined devices.
func DeviceGroupFromYamlFile(path string) (DeviceGroup, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	yamlBytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	conf := &Config{}
	if err := yaml.Unmarshal(yamlBytes, conf); err != nil {
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
