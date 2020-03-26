package powercycle

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/flynn/json5"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// DeviceGroup describes a set of devices that can all be
// controlled together. Any switch or power strip needs to
// implement this interface.
type DeviceGroup interface {
	// DeviceIDs returns a list of strings that uniquely identify
	// the devices that can be controlled through this group.
	DeviceIDs() []string

	// PowerCycle turns the device off for a reasonable amount of time
	// (i.e. 10 seconds) and then turns it back on. If delayOverride
	// is larger than zero it overrides the default delay between
	// turning the port off and on again.
	PowerCycle(devID string, delayOverride time.Duration) error

	// PowerUsage returns the power usage of all devices in the group
	// at a specific time.
	PowerUsage() (*GroupPowerUsage, error)
}

// GroupPowerUsage captures power usage of a set of devices at
// a specific time stamp.
type GroupPowerUsage struct {
	// Time stamp when the usage sample was taken.
	TS time.Time
	// Map[deviceID]PowerStat capturs power usage for the devices in the group.
	Stats map[string]*PowerStat
}

// PowerStat captures the current, voltage and wattage.
type PowerStat struct {
	Ampere float32 // current in mA
	Volt   float32 // voltage in V
	Watt   float32 // wattage in W (redundant ~Ampere * Volt / 1000)
}

// Config is the overall structure to aggregate configuration options
// for different device types.
type Config struct {
	// MPower aggregates all mPower configurations.
	MPower map[string]*MPowerConfig `json:"mpower"`

	// EdgeSwitch aggregates all EdgeSwitch configurations.
	EdgeSwitch map[string]*EdgeSwitchConfig `json:"edgeswitch"`

	// Seeeduino aggregates all Seeeduino configurations.
	Seeeduino map[string]*SeeeduinoConfig `json:"seeeduino"`
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

// DeviceGroupFromJson5File parses a Json5 file and instantiates the
// defined devices.
func DeviceGroupFromJson5File(path string, connect bool) (DeviceGroup, error) {
	conf, err := readConfig(path)
	if err != nil {
		return nil, err
	}

	ret := &aggregatedDevGroup{
		idDevGroupMap: map[string]DeviceGroup{},
	}

	// Add the mpower devices.
	for _, c := range conf.MPower {
		mp, err := NewMPowerClient(c, connect)
		if err != nil {
			return nil, err
		}

		if err := ret.add(mp); err != nil {
			return nil, err
		}
	}

	// Add the EdgeSwitch devices.
	for _, c := range conf.EdgeSwitch {
		es, err := NewEdgeSwitchDevGroup(c, connect)
		if err != nil {
			return nil, err
		}

		if err := ret.add(es); err != nil {
			return nil, err
		}
	}

	// Add the Seeeduino boards.
	for _, c := range conf.Seeeduino {
		ar, err := NewSeeeduinoClient(c, connect)
		if err != nil {
			return nil, err
		}
		if err := ret.add(ar); err != nil {
			return nil, err
		}
	}

	return ret, nil
}

func readConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer util.Close(f)

	jsonBytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	conf := &Config{}
	if err := json5.Unmarshal(jsonBytes, conf); err != nil {
		return nil, err
	}

	return conf, nil
}
