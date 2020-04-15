package powercycle

import (
	"bytes"
	"io/ioutil"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/crypto/ssh"
)

const (
	// Amount of time to wait between turning a port on and off again.
	powerOffDelayEdgeSwitch = 5 * time.Second
)

// edgeSwitchConfig contains configuration options for a single EdgeSwitch. Authentication is
// handled via *sigh* a hard-coded default password.
type edgeSwitchConfig struct {
	// IP address and port of the device, i.e. 192.168.1.33:22
	Address string `json:"address"`
	// Mapping between device id and port on the power strip.
	DevPortMap map[DeviceID]int `json:"ports"`
}

// edgeSwitchClient implements the Client interface.
type edgeSwitchClient struct {
	conf       *edgeSwitchConfig
	portDevMap map[int]DeviceID
	devIDs     []DeviceID
	ssh        *edgeSwitchSSHClient
}

// newEdgeSwitchController connects to the EdgeSwitch identified by the given
// configuration and returns a new instance of edgeSwitchClient.
func newEdgeSwitchController(conf *edgeSwitchConfig, connect bool) (*edgeSwitchClient, error) {
	ret := &edgeSwitchClient{
		conf: conf,
		ssh:  NewEdgeSwitchClient(conf.Address),
	}

	if connect {
		if err := Ping(ret.ssh); err != nil {
			return nil, skerr.Wrapf(err, "pinging switch")
		}
	}

	// Build the dev-port mappings. Ensure each device and port occur only once.
	ret.portDevMap = make(map[int]DeviceID, len(conf.DevPortMap))
	for id, port := range conf.DevPortMap {
		if _, ok := ret.portDevMap[port]; ok {
			return nil, skerr.Fmt("Port '%d' specified more than once.", port)
		}
		ret.portDevMap[port] = id
		ret.devIDs = append(ret.devIDs, id)
	}
	sortIDs(ret.devIDs)
	return ret, nil
}

// DeviceIDs implements the Client interface.
func (e *edgeSwitchClient) DeviceIDs() []DeviceID {
	return e.devIDs
}

// PowerCycle implements the Client interface.
func (e *edgeSwitchClient) PowerCycle(id DeviceID, delayOverride time.Duration) error {
	delay := powerOffDelayEdgeSwitch
	if delayOverride > 0 {
		delay = delayOverride
	}

	port, ok := e.conf.DevPortMap[id]
	if !ok {
		return skerr.Fmt("Invalid id: %s", id)
	}

	if ok := softPowerCycle(id); ok {
		sklog.Infof("Was able to powercycle %s via SSH", id)
		return nil
	}

	// Turn the given port off, wait and then on again.
	if err := TurnOffPort(e.ssh, port); err != nil {
		return skerr.Wrap(err)
	}

	time.Sleep(delay)

	if err := TurnOnPort(e.ssh, port); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// softPowerCycle attempts to SSH into the machine using the jumphost's private/public key and
// reboot it. This should help the jarring behavior seen when a bot is hard-rebooted frequently.
func softPowerCycle(machineName DeviceID) bool {
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
	// We rely on a dns lookup for the bot id ("e.g. skia-rpi-001") for this to work.
	// The router or the host can have it in /etc/host.
	client, err := ssh.Dial("tcp", string(machineName)+":22", c)
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
