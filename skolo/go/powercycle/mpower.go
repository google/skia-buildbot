package powercycle

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// mPowerConfig contains the necessary parameters to connect and control an mPower Pro power strip.
// Authentication is handled via the mPower switch recognizing the host's SSH key.
// See go/skolo-powercycle-setup for more.
type mPowerConfig struct {
	// IP address of the device, i.e. 192.168.1.33
	Address string `json:"address"`

	// User of the ssh connection
	User string `json:"user"`

	// Mapping between device name and port on the power strip.
	DevPortMap map[DeviceID]int `json:"ports"`
}

// Validate returns an error if the configuration is not complete.
func (c *mPowerConfig) Validate() error {
	if c.User == "" || c.Address == "" {
		return skerr.Fmt("You must specify a user and ip address.")
	}
	return nil
}

// Constants used to access the Ubiquiti mPower Pro.
const (
	// String template to address a relay.
	relayTemplateMPower = "/proc/power/relay%d"

	// Default amount of time to wait between turn off and on.
	powerOffDelayMPower = 10 * time.Second

	// Values to write to the relay file to disable/enable ports.
	mpowerOff = "0"
	mpowerOn  = "1"
)

// mPowerClient implements the Controller interface.
type mPowerClient struct {
	runner       CommandRunner
	deviceIDs    []DeviceID
	mPowerConfig *mPowerConfig
}

// newMPowerController returns a new instance of Controller for the mPowerPro
// power strip.
//
// The *mPowerClient is always returned not nil as long as the config is valid,
// so even on error it can be interrogated for the list of machines.
func newMPowerController(ctx context.Context, conf *mPowerConfig, connect bool) (*mPowerClient, error) {
	if err := conf.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	target := fmt.Sprintf("%s@%s", conf.User, conf.Address)
	// The mPower switch is running old firmware that only supports older SSH algorithms. Golang no
	// longer supports it, so we shell out to a native ssh binary and tell it to include the older
	// diffie-hellman-group1-sha1 algorithm. The -T removes a warning SSH gives because we are not
	// invoking it over TTY.
	runner := PublicKeySSHCommandRunner("-oKexAlgorithms=+diffie-hellman-group1-sha1", "-T", target)

	devIDs := make([]DeviceID, 0, len(conf.DevPortMap))
	sklog.Infof("conf %v", *conf)
	for id, port := range conf.DevPortMap {
		if port < 1 || port > 8 {
			return nil, skerr.Fmt("invalid port for %s (%d)", id, port)
		}
		devIDs = append(devIDs, id)
	}
	sortIDs(devIDs)

	ret := &mPowerClient{
		runner:       runner,
		deviceIDs:    devIDs,
		mPowerConfig: conf,
	}

	if connect {
		out, err := runner.ExecCmds(ctx, "cat /proc/power/active_pwr1")
		if err != nil {
			return ret, skerr.Wrapf(err, "performing smoke test on mpower %s; output: %s", target, out)
		}
		sklog.Infof("connected successfully to mpower %s", target)
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
		delay = delayOverride
	}

	if !DeviceIn(id, m.deviceIDs) {
		return skerr.Fmt("Unknown device ID: %s", id)
	}

	port := m.mPowerConfig.DevPortMap[id]
	if err := m.setPortValue(ctx, port, mpowerOff); err != nil {
		return skerr.Wrapf(err, "turning port %d off", port)
	}

	sklog.Infof("Switched port %d off. Waiting for %s.", port, delay)
	time.Sleep(delay)
	if err := m.setPortValue(ctx, port, mpowerOn); err != nil {
		return skerr.Wrapf(err, "turning port %d back on", port)
	}

	sklog.Infof("Switched port %d on.", port)
	return nil
}

func (m *mPowerClient) setPortValue(ctx context.Context, port int, value string) error {
	if out, err := m.runner.ExecCmds(ctx, fmt.Sprintf("echo %s > %s", value, getRelayFile(port))); err != nil {
		return skerr.Wrapf(err, "while setting port value - got output %s", out)
	}
	// echo doesn't return any output, so we ignore the out in a non error case.
	return nil
}

// getRelayFile returns name of the relay file for the given port.
func getRelayFile(port int) string {
	return fmt.Sprintf(relayTemplateMPower, port)
}
