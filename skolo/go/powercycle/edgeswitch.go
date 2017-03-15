package main

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/go/sklog"
	"golang.org/x/crypto/ssh"
)

// For details about the Ubiquiti EdgeSwitch see
// https://dl.ubnt.com/guides/edgemax/EdgeSwitch_CLI_Command_Reference_UG.pdf

const (
	// EdgeSwitch default user and password.
	DEFAULT_USER = "ubnt"

	// Number of seconds to wait between turning a port on and off again.
	EDGE_SWITCH_DELAY = 5
)

// EdgeSwitchConfig contains configuration options for a single EdgeSwitch.
// Note: We assume the device is on a trusted network.
type EdgeSwitchConfig struct {
	Address    string         `toml:"address"` // IP address and port of the device, i.e. 192.168.1.33:22
	DevPortMap map[string]int `toml:"ports"`   // Mapping between device name and port on the power strip.
}

// EdgeSwitchClient allows to control a single EdgeSwitch and
// implements the DeviceGroup interface.
type EdgeSwitchClient struct {
	conf *EdgeSwitchConfig
}

// NewEdgeSwitchClient connects to the EdgeSwitch identified by the given
// configuration and returns a new instane of EdgeSwitchClient.
func NewEdgeSwitchClient(conf *EdgeSwitchConfig) (*EdgeSwitchClient, error) {
	ret := &EdgeSwitchClient{
		conf: conf,
	}

	if err := ret.ping(); err != nil {
		return nil, err
	}
	return ret, nil
}

// DeviceIDs, see the DeviceGroup interface.
func (e *EdgeSwitchClient) DeviceIDs() []string {
	ret := make([]string, 0, len(e.conf.DevPortMap))
	for id := range e.conf.DevPortMap {
		ret = append(ret, id)
	}
	sort.Strings(ret)
	return ret
}

// PowerCycle, see the DeviceGroup interface.
func (e *EdgeSwitchClient) PowerCycle(devID string) error {
	port, ok := e.conf.DevPortMap[devID]
	if !ok {
		return fmt.Errorf("Invalid port: %d", port)
	}

	// Turn the given port off, wait and then on again.
	if err := e.turnOffPort(port); err != nil {
		return err
	}

	time.Sleep(EDGE_SWITCH_DELAY * time.Second)

	if err := e.turnOnPort(port); err != nil {
		return err
	}
	return nil
}

// turnOffPort disables PoE at the given port.
func (e *EdgeSwitchClient) turnOffPort(port int) error {
	_, err := e.execCmds([]string{
		"configure",
		"interface " + fmt.Sprintf("0/%d", port),
		"poe opmode shutdown",
		"exit", // leave the interface config mode (entered via 'interface ...')
		"exit", // leave the global configuration mode (entered via 'configure')
	})
	return err
}

// turnOffPort enables PoE at the given port.
func (e *EdgeSwitchClient) turnOnPort(port int) error {
	_, err := e.execCmds([]string{
		"configure",
		"interface " + fmt.Sprintf("0/%d", port),
		"poe opmode auto",
		"exit",
		"exit",
	})
	return err
}

// newClient returns a new ssh client.
func (e *EdgeSwitchClient) newClient() (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User: DEFAULT_USER,
		Auth: []ssh.AuthMethod{ssh.Password(DEFAULT_USER)},
		Config: ssh.Config{
			Ciphers: []string{"aes128-cbc", "3des-cbc", "aes256-cbc",
				"twofish256-cbc", "twofish-cbc", "twofish128-cbc", "blowfish-cbc"},
		},
	}

	client, err := ssh.Dial("tcp", e.conf.Address, sshConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// execCmds executes a series of commands and returns the accumulated
// output of all commands.
func (e *EdgeSwitchClient) execCmds(cmds []string) ([]string, error) {
	// The EdgeSwitch server doesn't like to re-use a client. So we create
	// a new connection for every series of commands.
	client, err := e.newClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	// Set a terminal with many lines so we are not paginated.
	if err := session.RequestPty("xterm", 80, 5000, nil); err != nil {
		return nil, fmt.Errorf("Error: Could not retrieve pseudo terminal: %s", err)
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := session.Shell(); err != nil {
		return nil, err
	}

	// Switch to exec mode.
	if _, err := stdinPipe.Write([]byte("enable\n")); err != nil {
		return nil, err
	}

	// Execute the commands.
	for _, cmd := range cmds {
		sklog.Infof("Executing: %s", cmd)
		if _, err := stdinPipe.Write([]byte(cmd + "\n")); err != nil {
			return nil, err
		}
	}

	// Switch out of exec mode and leave the shell.
	if _, err := stdinPipe.Write([]byte("exit\nexit\n")); err != nil {
		return nil, err
	}

	// Get the output and return it.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, stdoutPipe); err != nil {
		return nil, err
	}

	// Strip out empty lines and all lines with the prompt.
	lines := strings.Split(buf.String(), "\n")
	ret := make([]string, 0, len(lines))
	for _, line := range lines {
		oneLine := strings.TrimSpace(line)
		if (oneLine == "") || (strings.HasPrefix(oneLine, "(UBNT EdgeSwitch)")) {
			continue
		}
		ret = append(ret, oneLine)
	}
	return ret, nil
}

// ping runs a simple command to make sure the connection works.
func (c *EdgeSwitchClient) ping() error {
	sklog.Infof("Executing ping.")
	output, err := c.execCmds([]string{
		"show clock",
	})
	sklog.Infof("OUT:%s", strings.Join(output, "\n"))
	return err
}
