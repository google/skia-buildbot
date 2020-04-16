package powercycle

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/crypto/ssh"
)

// mPowerConfig contains the necessary parameters to connect and control an mPower Pro power strip.
// Authentication is handled via the mPower switch recognizing the host's SSH key.
type mPowerConfig struct {
	// IP address and port of the device, i.e. 192.168.1.33:22
	Address string `json:"address"`

	// User of the ssh connection
	User string `json:"user"`

	// Mapping between device name and port on the power strip.
	DevPortMap map[DeviceID]int `json:"ports"`
}

// Constants used to access the Ubiquiti mPower Pro.
const (
	// Location of the directory where files are that control the device.
	rootDirMPower = "/proc/power"

	// String template to address a relay.
	relayTemplateMPower = "relay%d"

	// Amount of time to wait between turn off and on.
	powerOffDelayMPower = 10 * time.Second

	// Values to write to the relay file to disable/enable ports.
	off = 0
	on  = 1
)

// Mapping between strings and port states.
var powerValues = map[string]int{
	"0": off,
	"1": on,
}

// mPowerClient implements the Controller interface.
type mPowerClient struct {
	client       *ssh.Client
	deviceIDs    []DeviceID
	mPowerConfig *mPowerConfig
}

// newMPowerController returns a new instance of Controller for the mPowerPro power strip.
func newMPowerController(mPowerConfig *mPowerConfig, connect bool) (*mPowerClient, error) {
	var client *ssh.Client = nil
	if connect {
		pKey := os.ExpandEnv("${HOME}/.ssh/id_rsa")
		key, err := ioutil.ReadFile(pKey)
		if err != nil {
			return nil, skerr.Wrapf(err, "reading private key at %s", pKey)
		}
		sklog.Infof("Retrieved private key")

		// Create the Signer for this private key.
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, skerr.Wrapf(err, "parsing private key at %s", pKey)
		}
		sklog.Infof("Parsed private key")

		sshConfig := &ssh.ClientConfig{
			User: mPowerConfig.User,
			Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
			Config: ssh.Config{
				Ciphers: []string{"aes128-cbc", "3des-cbc", "aes256-cbc",
					"twofish256-cbc", "twofish-cbc", "twofish128-cbc", "blowfish-cbc"},
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}

		sklog.Infof("Signed private key")

		client, err = ssh.Dial("tcp", mPowerConfig.Address, sshConfig)
		if err != nil {
			return nil, skerr.Wrapf(err, "dialing %s", mPowerConfig.Address)
		}

		sklog.Infof("Dial successful")
	}

	devIDs := make([]DeviceID, 0, len(mPowerConfig.DevPortMap))
	for id := range mPowerConfig.DevPortMap {
		devIDs = append(devIDs, id)
	}
	sortIDs(devIDs)

	ret := &mPowerClient{
		client:       client,
		deviceIDs:    devIDs,
		mPowerConfig: mPowerConfig,
	}

	if connect {
		if err := ret.ping(); err != nil {
			return nil, skerr.Wrapf(err, "pinging the mPower")
		}
	}

	return ret, nil
}

// DeviceIDs implements the Controller interface.
func (m *mPowerClient) DeviceIDs() []DeviceID {
	return m.deviceIDs
}

// PowerCycle implements the Controller interface.
func (m *mPowerClient) PowerCycle(ctx context.Context, id DeviceID, delayOverride time.Duration) error {
	delay := powerOffDelayMPower
	if delayOverride > 0 {
		delayOverride = delay
	}

	if !DeviceIn(id, m.deviceIDs) {
		return skerr.Fmt("Unknown device ID: %s", id)
	}

	port := m.mPowerConfig.DevPortMap[id]
	if err := m.setPortValue(port, off); err != nil {
		return err
	}

	sklog.Infof("Switched port %d off. Waiting for %s.", port, powerOffDelayMPower)
	time.Sleep(delay)
	if err := m.setPortValue(port, on); err != nil {
		return err
	}

	sklog.Infof("Switched port %d on.", port)
	return nil
}

// ping issues a command to the device to verify that the connection works.
func (m *mPowerClient) ping() error {
	sklog.Infof("Executing ping.")

	session, err := m.client.NewSession()
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(session)

	out, err := session.CombinedOutput("pwd")
	if err != nil {
		return skerr.Wrap(err)
	}
	sklog.Infof("PWD: %s", string(out))
	return nil
}

// getPortValue returns the status of given port.
func (m *mPowerClient) getPortValue(port int) (int, error) {
	if !m.validPort(port) {
		return off, skerr.Fmt("Invalid port. Expected 1-8 got %d", port)
	}

	session, err := m.client.NewSession()
	if err != nil {
		return off, skerr.Wrap(err)
	}
	defer util.Close(session)

	cmd := fmt.Sprintf("cat %s", m.getRelayFile(port))
	outBytes, err := session.CombinedOutput(cmd)
	if err != nil {
		return off, skerr.Wrap(err)
	}
	out := strings.TrimSpace(string(outBytes))
	current, ok := powerValues[out]
	if !ok {
		return off, skerr.Fmt("unexpected relay value: %s", out)
	}
	return current, nil
}

// getRelayFile returns name of the relay file for the given port.
func (m *mPowerClient) getRelayFile(port int) string {
	return path.Join(rootDirMPower, fmt.Sprintf(relayTemplateMPower, port))
}

// validPort returns true if the given port is valid.
func (m *mPowerClient) validPort(port int) bool {
	return (port >= 1) && (port <= 8)
}

// setPortValue sets the value of the given port to the given value.
func (m *mPowerClient) setPortValue(port int, newVal int) error {
	if !m.validPort(port) {
		return skerr.Fmt("Invalid port. Expected 1-8 got %d", port)
	}

	session, err := m.client.NewSession()
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.Close(session)

	// Check if the value is already the target value.
	if current, err := m.getPortValue(port); err != nil {
		return skerr.Wrap(err)
	} else if current == newVal {
		return nil
	}

	cmd := fmt.Sprintf("echo '%d' > %s", newVal, m.getRelayFile(port))
	sklog.Infof("Executing: %s", cmd)
	_, err = session.CombinedOutput(cmd)
	if err != nil {
		return err
	}

	// Check if the value was set correctly.
	if current, err := m.getPortValue(port); err != nil {
		return skerr.Wrap(err)
	} else if current != newVal {
		return skerr.Fmt("Could not read back new value. Got %d", current)
	}

	return nil
}
