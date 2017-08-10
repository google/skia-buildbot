package powercycle

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
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
	Address    string         `json:"address"` // IP address and port of the device, i.e. 192.168.1.33:22
	DevPortMap map[string]int `json:"ports"`   // Mapping between device name and port on the power strip.
}

// EdgeSwitchClient allows to control a single EdgeSwitch and
// implements the DeviceGroup interface.
type EdgeSwitchClient struct {
	conf       *EdgeSwitchConfig
	portDevMap map[int]string
	devIDs     []string
}

// NewEdgeSwitchClient connects to the EdgeSwitch identified by the given
// configuration and returns a new instane of EdgeSwitchClient.
func NewEdgeSwitchClient(conf *EdgeSwitchConfig, connect bool) (DeviceGroup, error) {
	ret := &EdgeSwitchClient{
		conf: conf,
	}

	if connect {
		if err := ret.ping(); err != nil {
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
func (e *EdgeSwitchClient) DeviceIDs() []string {
	return e.devIDs
}

// PowerCycle, see the DeviceGroup interface.
func (e *EdgeSwitchClient) PowerCycle(devID string, delayOverride time.Duration) error {
	delay := EDGE_SWITCH_DELAY * time.Second
	if delayOverride > 0 {
		delay = delayOverride
	}

	port, ok := e.conf.DevPortMap[devID]
	if !ok {
		return fmt.Errorf("Invalid port: %d", port)
	}

	// Turn the given port off, wait and then on again.
	if err := e.turnOffPort(port); err != nil {
		return err
	}

	time.Sleep(delay)

	if err := e.turnOnPort(port); err != nil {
		return err
	}
	return nil
}

// PowerUsage, see the DeviceGroup interface.
func (e *EdgeSwitchClient) PowerUsage() (*GroupPowerUsage, error) {
	outputLines, err := e.execCmds([]string{
		"show poe status all",
	})
	if err != nil {
		return nil, err
	}

	ret := &GroupPowerUsage{
		TS: time.Now(),
	}
	ret.Stats = make(map[string]*PowerStat, len(outputLines))
	// only consider lines like:
	// Intf      Detection      Class   Consumed(W) Voltage(V) Current(mA) Temperature(C)
	// 0/6       Good           Class3         1.93      52.82       36.62             45
	for _, oneLine := range outputLines {
		fields := strings.Fields(oneLine)
		if (len(fields) < 7) || (len(fields[0]) < 3) || (fields[0][1] != '/') {
			continue
		}

		stat := &PowerStat{}
		var err error = nil
		last := len(fields)
		stat.Ampere = parseFloat(&err, fields[last-2])
		stat.Volt = parseFloat(&err, fields[last-3])
		stat.Watt = parseFloat(&err, fields[last-4])
		port := parseInt(&err, fields[0][2:])

		if err != nil {
			sklog.Errorf("Error: %s", err)
			continue
		}

		devID, ok := e.portDevMap[port]

		if !ok {
			continue
		}

		sklog.Infof("Found port %d and dev '%s'", port, devID)
		ret.Stats[devID] = stat
	}

	return ret, nil
}

func parseFloat(err *error, strVal string) float32 {
	if *err != nil {
		return 0
	}
	var ret float64
	ret, *err = strconv.ParseFloat(strVal, 32)
	return float32(ret)
}

func parseInt(err *error, strVal string) int {
	if *err != nil {
		return 0
	}
	var ret int64
	ret, *err = strconv.ParseInt(strVal, 10, 32)
	return int(ret)
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
		User:            DEFAULT_USER,
		Auth:            []ssh.AuthMethod{ssh.Password(DEFAULT_USER)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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
	defer util.Close(client)

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer util.Close(session)

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
