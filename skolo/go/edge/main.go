package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
)

const (
	ADDRESS  = "192.168.1.41:22"
	USER     = "ubnt"
	PASSWORD = "ubnt"
)

type CmdClient struct{}

func NewCmdClient() (*CmdClient, error) {
	ret := &CmdClient{}

	if err := ret.ping(); err != nil {
		return nil, err
	}
	if err := ret.ping(); err != nil {
		return nil, err
	}
	sklog.Infof("ALL done XXX.")
	return ret, nil
}

func (c *CmdClient) newClient() (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User: USER,
		Auth: []ssh.AuthMethod{ssh.Password(PASSWORD)},
		// Config: ssh.Config{
		// 	Ciphers: []string{"aes128-cbc", "3des-cbc", "aes256-cbc",
		// 		"twofish256-cbc", "twofish-cbc", "twofish128-cbc", "blowfish-cbc"},
		// },
	}

	client, err := ssh.Dial("tcp", ADDRESS, sshConfig)
	if err != nil {
		return nil, err
	}

	sklog.Infof("Dial successful")
	return client, nil
}

func (c *CmdClient) ExecCmds(cmds []string) ([]string, error) {
	client, err := c.newClient()
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
	// modes := ssh.TerminalModes{
	// // ssh.ECHO:          0,     // disable echoing
	// // ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
	// // ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	// }

	if err := session.RequestPty("xterm", 80, 5000, nil); err != nil {
		return nil, fmt.Errorf("request for pseudo terminal failed: %s", err)
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

func (c *CmdClient) ping() error {
	sklog.Infof("Executing ping.")
	output, err := c.ExecCmds([]string{
		"show clock",
	})
	sklog.Infof("OUT:%s", strings.Join(output, "\n"))
	return err
}

var (
	port = flag.Int("port", -1, "Port to power cycle")
)

func main() {
	common.Init()

	if *port == -1 {
		sklog.Fatalln("No port specified.")
	}

	c, err := NewCmdClient()
	if err != nil {
		sklog.Fatalf("Unable to create client. Got error: %s", err)
	}

	outputLines, err := c.ExecCmds([]string{"show poe status all"})
	if err != nil {
		sklog.Fatalf("Error: %s", err)
	}
	sklog.Infof("OUTPUT status: %s", strings.Join(outputLines, "\n"))

	offCmds := []string{
		"configure",
		"interface " + fmt.Sprintf("0/%d", *port),
		"poe opmode shutdown",
		"exit",
		"exit",
	}

	outputLines, err = c.ExecCmds(offCmds)
	if err != nil {
		sklog.Fatalf("Error: %s", err)
	}
	sklog.Infof("OUTPUT shutdown: %s", strings.Join(outputLines, "\n"))

	time.Sleep(time.Second * 10)

	outputLines, err = c.ExecCmds([]string{"show poe status all"})
	if err != nil {
		sklog.Fatalf("Error: %s", err)
	}
	sklog.Infof("OUTPUT status: %s", strings.Join(outputLines, "\n"))

	sklog.Infof("Waiting for 30 seconds")
	time.Sleep(time.Second * 10)

	onCmds := []string{
		"configure",
		"interface " + fmt.Sprintf("0/%d", *port),
		"poe opmode auto",
		"exit",
		"exit",
	}

	outputLines, err = c.ExecCmds(onCmds)
	if err != nil {
		sklog.Fatalf("Error: %s", err)
	}
	sklog.Infof("OUTPUT on: %s", strings.Join(outputLines, "\n"))

	time.Sleep(time.Second * 10)

	outputLines, err = c.ExecCmds([]string{"show poe status all"})
	if err != nil {
		sklog.Fatalf("Error: %s", err)
	}
	sklog.Infof("OUTPUT status: %s", strings.Join(outputLines, "\n"))

	// sklog.Infof("Got output:\n")
	// for idx, output := range outputs {
	// 	sklog.Infof("%3d: %s", idx, output)
	// }
	sklog.Infof("Success.")
}
