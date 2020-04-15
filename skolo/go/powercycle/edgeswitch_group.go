package powercycle

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"sort"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/crypto/ssh"
)

const (
	// Number of seconds to wait between turning a port on and off again.
	EDGE_SWITCH_DELAY = 5
)

// EdgeSwitchConfig contains configuration options for a single EdgeSwitch.
// Note: We assume the device is on a trusted network.
type EdgeSwitchConfig struct {
	Address    string         `json:"address"` // IP address and port of the device, i.e. 192.168.1.33:22
	DevPortMap map[string]int `json:"ports"`   // Mapping between device name and port on the power strip.
}

// EdgeSwitchDevGroup implements the DeviceGroup interface.
type EdgeSwitchDevGroup struct {
	conf       *EdgeSwitchConfig
	portDevMap map[int]string
	devIDs     []string
	client     *EdgeSwitchClient
}

// NewEdgeSwitchDevGroup connects to the EdgeSwitch identified by the given
// configuration and returns a new instance of EdgeSwitchDevGroup.
func NewEdgeSwitchDevGroup(conf *EdgeSwitchConfig, connect bool) (*EdgeSwitchDevGroup, error) {
	ret := &EdgeSwitchDevGroup{
		conf:   conf,
		client: NewEdgeSwitchClient(conf.Address),
	}

	if connect {
		if err := Ping(ret.client); err != nil {
			return nil, err
		}
	}

	// Build the dev-port mappings. Ensure each device and port occur only once.
	devIDSet := make(util.StringSet, len(conf.DevPortMap))
	ret.portDevMap = make(map[int]string, len(conf.DevPortMap))
	for id, port := range conf.DevPortMap {
		if devIDSet[id] {
			return nil, fmt.Errorf("Device '%s' occurs more than once.", id)
		}
		if _, ok := ret.portDevMap[port]; ok {
			return nil, fmt.Errorf("Port '%d' specified more than once.", port)
		}
		devIDSet[id] = true
		ret.portDevMap[port] = id
	}
	ret.devIDs = devIDSet.Keys()
	sort.Strings(ret.devIDs)

	return ret, nil
}

// DeviceIDs, see the DeviceGroup interface.
func (e *EdgeSwitchDevGroup) DeviceIDs() []string {
	return e.devIDs
}

// PowerCycle, see the DeviceGroup interface.
func (e *EdgeSwitchDevGroup) PowerCycle(devID string, delayOverride time.Duration) error {
	delay := EDGE_SWITCH_DELAY * time.Second
	if delayOverride > 0 {
		delay = delayOverride
	}

	port, ok := e.conf.DevPortMap[devID]
	if !ok {
		return fmt.Errorf("Invalid devID: %s", devID)
	}

	// We rely on a dns lookup for the bot id ("e.g. skia-rpi-001") for this to work.
	// The router or the host can have it in /etc/host.
	if ok := SoftPowerCycle(devID); ok {
		sklog.Infof("Was able to powercycle %s via SSH", devID)
		return nil
	}

	// Turn the given port off, wait and then on again.
	if err := TurnOffPort(e.client, port); err != nil {
		return err
	}

	time.Sleep(delay)

	if err := TurnOnPort(e.client, port); err != nil {
		return err
	}
	return nil
}

// SoftPowerCycle attempts to SSH into the machine using the
// jumphost's private/public key and reboot it. This should
// help the jarring behavior seen when a bot is hard-rebooted
// frequently.
func SoftPowerCycle(address string) bool {
	key, err := ioutil.ReadFile("/home/chrome-bot/.ssh/id_rsa")
	if err != nil {
		sklog.Errorf("unable to read private key: %s", err)
		return false
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		sklog.Errorf("unable to parse private key: %s", err)
		return false
	}
	c := &ssh.ClientConfig{
		User: "chrome-bot",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	client, err := ssh.Dial("tcp", address+":22", c)
	if err != nil {
		sklog.Errorf("Failed to dial: %s", err)
		return false
	}
	session, err := client.NewSession()
	if err != nil {
		sklog.Errorf("Failed to create session: %s", err)
		return false
	}
	defer util.Close(session)

	var b bytes.Buffer
	session.Stdout = &b
	// This always fails because the command doesn't return after reboot.
	_ = session.Run("sudo /sbin/reboot")
	sklog.Infof("Soft reboot should have succeeded.  See logs: %s", b.String())
	return true
}
