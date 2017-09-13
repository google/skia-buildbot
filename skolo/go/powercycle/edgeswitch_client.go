package powercycle

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/crypto/ssh"
)

// For details about the Ubiquiti EdgeSwitch see
// https://dl.ubnt.com/guides/edgemax/EdgeSwitch_CLI_Command_Reference_UG.pdf

const (
	// EdgeSwitch default user and password.
	DEFAULT_USER = "ubnt"
)

// EdgeSwitchClient implements the EdgeSwitchCommandRunner interface.
type EdgeSwitchClient struct {
	ipaddress string
}

type EdgeSwitchCommandRunner interface {
	// ExecCmds executes a series of commands and returns the accumulated
	// output of all commands.
	ExecCmds(cmds []string) ([]string, error)
}

// NewEdgeSwitchClient connects to the EdgeSwitch identified by the given
// configuration and returns a new instance of EdgeSwitchClient.
func NewEdgeSwitchClient(ipaddress string) *EdgeSwitchClient {
	return &EdgeSwitchClient{ipaddress: ipaddress}
}

// newClient returns a new ssh client.
func (e *EdgeSwitchClient) newClient() (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User:            DEFAULT_USER,
		Auth:            []ssh.AuthMethod{ssh.Password(DEFAULT_USER)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", e.ipaddress, sshConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// ExecCmds, see EdgeSwitchCommandRunner
func (e *EdgeSwitchClient) ExecCmds(cmds []string) ([]string, error) {
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
