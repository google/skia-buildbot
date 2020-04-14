package powercycle

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/crypto/ssh"
)

// MPowerConfig contains the necessary parameters to connect
// and control an mPowerPro power strip.
type MPowerConfig struct {
	Address    string         `json:"address"` // IP address and port of the device, i.e. 192.168.1.33:22
	User       string         `json:"user"`    // User of the ssh connection
	DevPortMap map[string]int `json:"ports"`   // Mapping between device name and port on the power strip.
}

// Constants used to access the Ubiquiti mPower Pro.
const (
	// Location of the directory where files are that control the device.
	MPOWER_ROOT = "/proc/power"

	// String template to address a relay.
	MPOWER_RELAY = "relay%d"

	// Number of seconds to wait between turn off and on.
	MPOWER_DELAY = 10

	// Values to write to the relay file to disable/enable ports.
	PORT_OFF = 0
	PORT_ON  = 1
)

// Mapping between strings and port states.
var POWER_VALUES = map[string]int{
	"0": PORT_OFF,
	"1": PORT_ON,
}

// MPowerClient implements the DeviceGroup interface.
type MPowerClient struct {
	client       *ssh.Client
	deviceIDs    []string
	mPowerConfig *MPowerConfig
}

// NewMPowerClient returns a new instance of DeviceGroup for the
// mPowerPro power strip.
func NewMPowerClient(mPowerConfig *MPowerConfig, connect bool) (DeviceGroup, error) {
	var client *ssh.Client = nil
	if connect {
		key, err := ioutil.ReadFile(os.ExpandEnv("${HOME}/.ssh/id_rsa"))
		if err != nil {
			return nil, fmt.Errorf("Unable to read private key: %v", err)
		}
		sklog.Infof("Retrieved private key")

		// Create the Signer for this private key.
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("Unable to parse private key: %v", err)
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
			return nil, err
		}

		sklog.Infof("Dial successful")
	}

	devIDs := make([]string, 0, len(mPowerConfig.DevPortMap))
	for id := range mPowerConfig.DevPortMap {
		devIDs = append(devIDs, id)
	}
	sort.Strings(devIDs)

	ret := &MPowerClient{
		client:       client,
		deviceIDs:    devIDs,
		mPowerConfig: mPowerConfig,
	}

	if connect {
		if err := ret.ping(); err != nil {
			return nil, err
		}
	}

	return ret, nil
}

// DeviceIDs, see DeviceGroup interface.
func (m *MPowerClient) DeviceIDs() []string {
	return m.deviceIDs
}

// PowerCycle, see PowerCycle interface.
func (m *MPowerClient) PowerCycle(devID string, delayOverride time.Duration) error {
	delay := MPOWER_DELAY * time.Second
	if delayOverride > 0 {
		delayOverride = delay
	}

	if !util.In(devID, m.deviceIDs) {
		return fmt.Errorf("Unknown device ID: %s", devID)
	}

	port := m.mPowerConfig.DevPortMap[devID]
	if err := m.setPortValue(port, PORT_OFF); err != nil {
		return err
	}

	sklog.Infof("Switched port %d off. Waiting for %d seconds.", port, MPOWER_DELAY)
	time.Sleep(delay)
	if err := m.setPortValue(port, PORT_ON); err != nil {
		return err
	}

	sklog.Infof("Switched port %d on.", port)
	return nil
}

// ping issues a command to the device to verify that the
// connection works.
func (m *MPowerClient) ping() error {
	sklog.Infof("Executing ping.")

	session, err := m.client.NewSession()
	if err != nil {
		return err
	}
	defer util.Close(session)

	out, err := session.CombinedOutput("pwd")
	if err != nil {
		return err
	}
	sklog.Infof("PWD: %s", string(out))
	return nil
}

// getPortValue returns the status of given port.
func (m *MPowerClient) getPortValue(port int) (int, error) {
	if !m.validPort(port) {
		return PORT_OFF, fmt.Errorf("Invalid port. Expected 1-8 got %d", port)
	}

	session, err := m.client.NewSession()
	if err != nil {
		return PORT_OFF, err
	}
	defer util.Close(session)

	cmd := fmt.Sprintf("cat %s", m.getRelayFile(port))
	outBytes, err := session.CombinedOutput(cmd)
	if err != nil {
		return PORT_OFF, err
	}
	out := strings.TrimSpace(string(outBytes))
	current, ok := POWER_VALUES[out]
	if !ok {
		return PORT_OFF, fmt.Errorf("Got unexpected relay value: %s", out)
	}
	return current, nil
}

// getRelayFile returns name of the relay file for the given port.
func (m *MPowerClient) getRelayFile(port int) string {
	return path.Join(MPOWER_ROOT, fmt.Sprintf(MPOWER_RELAY, port))
}

// validPort returns true if the given port is valid.
func (m *MPowerClient) validPort(port int) bool {
	return (port >= 1) && (port <= 8)
}

// setPortValue sets the value of the given port to the given value.
func (m *MPowerClient) setPortValue(port int, newVal int) error {
	if !m.validPort(port) {
		return fmt.Errorf("Invalid port. Expected 1-8 got %d", port)
	}

	session, err := m.client.NewSession()
	if err != nil {
		return err
	}
	defer util.Close(session)

	// Check if the value is already the target value.
	if current, err := m.getPortValue(port); err != nil {
		return err
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
		return err
	} else if current != newVal {
		return fmt.Errorf("Could not read back new value. Got %d", current)
	}

	return nil
}
