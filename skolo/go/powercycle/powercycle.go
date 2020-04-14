package powercycle

import (
	"io/ioutil"
	"sort"
	"time"

	"github.com/flynn/json5"
	"go.skia.org/infra/go/skerr"
)

// DeviceID is a unique identifier for a given machine or attached device.
type DeviceID string

// DeviceIn returns true if the given id is in the slice of DeviceID.
func DeviceIn(id DeviceID, ids []DeviceID) bool {
	for _, other := range ids {
		if other == id {
			return true
		}
	}
	return false
}

func sortIDs(ids []DeviceID) {
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
}

// Client abstracts a set of devices that can all be controlled together.
type Client interface {
	// DeviceIDs returns a list of strings that uniquely identify the devices that can be controlled
	// through this group.
	DeviceIDs() []DeviceID

	// PowerCycle turns the device off for a reasonable amount of time (i.e. 10 seconds) and then
	// turns it back on. If delayOverride is larger than zero it overrides the default delay between
	// turning the port off and on again.
	PowerCycle(id DeviceID, delayOverride time.Duration) error
}

// config is the overall structure to aggregate configuration options for different device types.
type config struct {
	// MPower aggregates all mPower configurations.
	MPower map[string]*mPowerConfig `json:"mpower"`

	// EdgeSwitch aggregates all EdgeSwitch configurations.
	EdgeSwitch map[string]*edgeSwitchConfig `json:"edgeswitch"`
}

// aggregatedClients allows us to combine multiple Client implementations into one.
type aggregatedClients struct {
	clientForID map[DeviceID]Client // Maps from a deviceID to a device group.
}

// add adds a new device group.
func (a *aggregatedClients) add(client Client) error {
	for _, id := range client.DeviceIDs() {
		if _, ok := a.clientForID[id]; ok {
			return skerr.Fmt("Device '%s' already exists.", id)
		}
		a.clientForID[id] = client
	}
	return nil
}

// DeviceIDs implements the Client interface.
func (a *aggregatedClients) DeviceIDs() []DeviceID {
	ret := make([]DeviceID, 0, len(a.clientForID))
	for id := range a.clientForID {
		ret = append(ret, id)
	}
	sortIDs(ret)
	return ret
}

// PowerCycle implements the Client interface.
func (a *aggregatedClients) PowerCycle(id DeviceID, delayOverride time.Duration) error {
	client, ok := a.clientForID[id]
	if !ok {
		return skerr.Fmt("Unknown device id: %s", id)
	}
	return client.PowerCycle(id, delayOverride)
}

// ParseJSON5 parses a JSON5 file and instantiates the defined devices. If connect is true, an
// attempt will be made to connect to the subclients an errors will be returned if they are not
// accessible.
func ParseJSON5(path string, connect bool) (Client, error) {
	conf, err := readConfig(path)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	ret := &aggregatedClients{
		clientForID: map[DeviceID]Client{},
	}

	// Add the mpower devices.
	for _, c := range conf.MPower {
		mp, err := newMPowerClient(c, connect)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		if err := ret.add(mp); err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	// Add the EdgeSwitch devices.
	for _, c := range conf.EdgeSwitch {
		es, err := newEdgeSwitchClient(c, connect)
		if err != nil {
			return nil, skerr.Wrap(err)
		}

		if err := ret.add(es); err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	return ret, nil
}

func readConfig(path string) (config, error) {
	conf := config{}
	jsonBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return conf, skerr.Wrapf(err, "reading %s", path)
	}

	if err := json5.Unmarshal(jsonBytes, &conf); err != nil {
		return conf, skerr.Wrapf(err, "reading JSON5 from %s", path)
	}
	return conf, nil
}
