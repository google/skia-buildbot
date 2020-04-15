package powercycle

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/crypto/ssh"
)

// For details about the Ubiquiti EdgeSwitch see
// https://dl.ubnt.com/guides/edgemax/EdgeSwitch_CLI_Command_Reference_UG.pdf

const (
	// EdgeSwitch default user and password.
	edgeswitchDefaultUser     = "ubnt"
	edgeswitchDefaultPassword = "ubnt"
)

// The CommandRunner interface adds a layer of abstraction around a device
// that can have commands executed on it.
type CommandRunner interface {
	// ExecCmds executes a series of commands and returns the accumulated
	// output of all commands.
	ExecCmds(cmds []string) ([]string, error)
}

// edgeSwitchSSHClient implements the CommandRunner interface.
type edgeSwitchSSHClient struct {
	ipaddress string
}

// NewEdgeSwitchClient connects to the EdgeSwitch identified by the given
// configuration and returns a new instance of edgeSwitchSSHClient.
func NewEdgeSwitchClient(ipaddress string) *edgeSwitchSSHClient {
	return &edgeSwitchSSHClient{ipaddress: ipaddress}
}

// newSSHClient returns a new ssh client.
func (e *edgeSwitchSSHClient) newSSHClient() (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User:            edgeswitchDefaultUser,
		Auth:            []ssh.AuthMethod{ssh.Password(edgeswitchDefaultPassword)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", e.ipaddress, sshConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// ExecCmds, see CommandRunner
func (e *edgeSwitchSSHClient) ExecCmds(cmds []string) ([]string, error) {
	// The EdgeSwitch server doesn't like to re-use an ssh client. So we create
	// a new connection for every series of commands.
	client, err := e.newSSHClient()
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

// The following are convenience methods for common functionality and
// act as example usage.

// TurnOffPort disables PoE at the given port.
func TurnOffPort(client CommandRunner, port int) error {
	_, err := client.ExecCmds([]string{
		"configure",
		"interface " + fmt.Sprintf("0/%d", port),
		"poe opmode shutdown",
		"exit", // leave the interface config mode (entered via 'interface ...')
		"exit", // leave the global configuration mode (entered via 'configure')
	})
	return skerr.Wrap(err)
}

// TurnOnPort enables PoE at the given port.
func TurnOnPort(client CommandRunner, port int) error {
	_, err := client.ExecCmds([]string{
		"configure",
		"interface " + fmt.Sprintf("0/%d", port),
		"poe opmode auto",
		"exit", // leave the interface config mode (entered via 'interface ...')
		"exit", // leave the global configuration mode (entered via 'configure')
	})
	return skerr.Wrap(err)
}

// Ping runs a simple command to make sure the connection works.
func Ping(client CommandRunner) error {
	sklog.Infof("Executing ping.")
	output, err := client.ExecCmds([]string{
		"show clock",
	})
	sklog.Infof("OUT:%s", strings.Join(output, "\n"))
	return skerr.Wrap(err)
}
