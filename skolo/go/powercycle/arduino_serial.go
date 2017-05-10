package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"sort"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/crypto/ssh"
)

// ArduinoConfig contains the necessary parameters to connect
// and control an Arduino with servos via the serial_relay program, that
// is assumed to run on the remote host.
type ArduinoConfig struct {
	Address    string         `yaml:"address"` // IP address and port of the device, i.e. 192.168.1.33:22
	DevPortMap map[string]int `yaml:"ports"`   // Mapping between device name and port on the power strip.
}

// ArduinoClient implements the DeviceGroup interface.
type ArduinoClient struct {
	client        *ssh.Client
	deviceIDs     []string
	arduinoConfig *ArduinoConfig
}

// NewArduinoClient returns a new instance of DeviceGroup for the
// Arduino driven servos.
func NewArduinoClient(arduinoConfig *ArduinoConfig) (DeviceGroup, error) {
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

	currUser, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve current user: %s", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            currUser.Username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sklog.Infof("Signed private key")

	client, err := ssh.Dial("tcp", arduinoConfig.Address, sshConfig)
	if err != nil {
		return nil, err
	}

	sklog.Infof("Dial successful")

	devIDs := make([]string, 0, len(arduinoConfig.DevPortMap))
	for id := range arduinoConfig.DevPortMap {
		devIDs = append(devIDs, id)
	}
	sort.Strings(devIDs)

	ret := &ArduinoClient{
		client:        client,
		deviceIDs:     devIDs,
		arduinoConfig: arduinoConfig,
	}

	if err := ret.ping(); err != nil {
		return nil, err
	}

	return ret, nil
}

// DeviceIDs, see DeviceGroup interface.
func (a *ArduinoClient) DeviceIDs() []string {
	return a.deviceIDs
}

// PowerCycle, see PowerCycle interface.
func (a *ArduinoClient) PowerCycle(devID string, delayOverride time.Duration) error {
	if !util.In(devID, a.deviceIDs) {
		return fmt.Errorf("Unknown device ID: %s", devID)
	}

	port := a.arduinoConfig.DevPortMap[devID]
	cmd := fmt.Sprintf("./serial_relay %d", port)
	session, err := a.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	sklog.Infof("Executing: %s", cmd)
	outBytes, err := session.CombinedOutput(cmd)
	if err != nil {
		return fmt.Errorf("Error executing %s: %s\nGot output:\n%s", cmd, err, string(outBytes))
	}

	sklog.Infof("Powercycled port %s on port %d.", devID, port)
	return nil
}

// PowerUsage, see the DeviceGroup interface.
func (a *ArduinoClient) PowerUsage() (*GroupPowerUsage, error) {
	return &GroupPowerUsage{}, nil // N/A for arduino board.
}

// ping issues a command to the device to verify that the
// connection works.
func (a *ArduinoClient) ping() error {
	sklog.Infof("Executing ping.")

	session, err := a.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	out, err := session.CombinedOutput("pwd")
	if err != nil {
		return err
	}
	sklog.Infof("PWD: %s", string(out))
	return nil
}
