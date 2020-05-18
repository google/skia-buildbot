package powercycle

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	// Amount of time to wait between turning a port on and off again.
	powerOffDelayEdgeSwitch = 5 * time.Second

	// values for the poe opmode
	edgeSwitchOff = "shutdown"
	edgeSwitchOn  = "auto"

	powerCyclePasswordEnvVar = "POWERCYCLE_PASSWORD"
)

// EdgeSwitchConfig contains configuration options for a single EdgeSwitch. Authentication is
// handled via a provided password. See go/skolo-powercycle-setup for more.
type EdgeSwitchConfig struct {
	// IP address of the device, i.e. 192.168.1.33
	Address string `json:"address"`

	// User of the ssh connection.
	User string `json:"user"`

	// Password for User. This can also be set by the environment variable "POWERCYCLE_PASSWORD".
	Password string `json:"password"`

	// Mapping between device id and port on the power strip.
	DevPortMap map[DeviceID]int `json:"ports"`
}

// Validate returns an error if the configuration is not complete.
func (c *EdgeSwitchConfig) Validate() error {
	if c.User == "" || c.Address == "" {
		return skerr.Fmt("You must specify a user and ip address.")
	}
	if c.getPassword() == "" {
		return skerr.Fmt("You must specify the password.")
	}
	return nil
}

// getPassword returns the password.
func (c *EdgeSwitchConfig) getPassword() string {
	if c.Password != "" {
		return c.Password
	}
	return strings.TrimSpace(os.Getenv(powerCyclePasswordEnvVar))
}

// edgeSwitchClient implements the Client interface.
type edgeSwitchClient struct {
	conf       *EdgeSwitchConfig
	portDevMap map[int]DeviceID
	devIDs     []DeviceID
	runner     CommandRunner
}

// newEdgeSwitchController connects to the EdgeSwitch identified by the given configuration and
// returns a new instance of edgeSwitchClient.
func newEdgeSwitchController(ctx context.Context, conf *EdgeSwitchConfig, connect bool) (*edgeSwitchClient, error) {
	if err := conf.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	target := fmt.Sprintf("%s@%s", conf.User, conf.Address)
	// The -T removes a warning SSH gives because we are not invoking it over TTY.
	runner := PasswordSSHCommandRunner(conf.getPassword(), "-T", target, "-o", "StrictHostKeyChecking=no")
	if connect {
		out, _ := runner.ExecCmds(ctx, "help")
		// When using sshpass, we always seem to get exit code 255 (from ssh) and any actual errors are
		// in stderr. So, we check the returned output for evidence that things actually worked
		if !strings.Contains(out, "HELP") {
			return nil, skerr.Fmt("smoke test on edge switch %s failed; output: %s", target, out)
		}
		sklog.Infof("connected successfully to edge switch %s", target)
	}

	ret := &edgeSwitchClient{
		conf:   conf,
		runner: runner,
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
func (e *edgeSwitchClient) PowerCycle(ctx context.Context, id DeviceID, delayOverride time.Duration) error {
	delay := powerOffDelayEdgeSwitch
	if delayOverride > 0 {
		delay = delayOverride
	}

	port, ok := e.conf.DevPortMap[id]
	if !ok {
		return skerr.Fmt("Invalid id: %s", id)
	}

	if ok := softPowerCycle(ctx, id); ok {
		sklog.Infof("Was able to powercycle %s via SSH", id)
		return nil
	}
	sklog.Infof("soft powercycle of %s failed, going to turn off POE port %d", id, port)

	if err := e.setPortValue(ctx, port, edgeSwitchOff); err != nil {
		return skerr.Wrapf(err, "turning port %d off", port)
	}

	sklog.Infof("Switched port %d off. Waiting for %s.", port, delay)
	time.Sleep(delay)
	if err := e.setPortValue(ctx, port, edgeSwitchOn); err != nil {
		return skerr.Wrapf(err, "turning port %d back on", port)
	}

	sklog.Infof("Switched port %d on.", port)

	return nil
}

// softPowerCycle attempts to SSH into the machine using the jumphost's private/public key and
// reboot it. This should help the jarring behavior seen when a bot is hard-rebooted frequently.
func softPowerCycle(ctx context.Context, machineName DeviceID) bool {
	// We rely on a dns lookup for the bot id ("e.g. skia-rpi-001") for this to work.
	// The router or the host can have it in /etc/host.
	machineRunner := PublicKeySSHCommandRunner("-T", string(machineName))

	tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// First try to run a trivial command to see if we can access the machine via SSH.
	if _, err := machineRunner.ExecCmds(tCtx, "time"); err != nil {
		return false
	}

	tCtx, cancel2 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel2()
	// Do not bother checking error - this always fails because the command doesn't return after
	// reboot.
	out, _ := machineRunner.ExecCmds(tCtx, "sudo /sbin/reboot -f")
	sklog.Infof("Soft reboot should have succeeded.  See logs: %s", out)
	return true
}

func (e *edgeSwitchClient) setPortValue(ctx context.Context, port int, value string) error {
	out, _ := e.runner.ExecCmds(ctx,
		"enable",
		"configure",
		fmt.Sprintf("interface 0/%d", port),
		fmt.Sprintf("poe opmode %s", value),
	)
	// When using sshpass, we always seem to get exit code 255 (from ssh) and any actual errors are
	// in stderr. So, we check the returned output for evidence that things actually worked
	if !strings.Contains(out, value) {
		return skerr.Fmt("Error while setting port value - got output %s", out)
	}
	sklog.Debugf("output while setting port %d to %s:\n%s\n", port, value, out)
	return nil
}
