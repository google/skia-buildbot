package main

import (
	"fmt"
	"log"

	"golang.org/x/crypto/ssh"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	ADDRESS  = "192.168.1.41:22"
	USER     = "ubnt"
	PASSWORD = "ubnt"
)

type CmdClient struct {
	client *ssh.Client
}

func NewCmdClient() (*CmdClient, error) {
	sshConfig := &ssh.ClientConfig{
		User: USER,
		Auth: []ssh.AuthMethod{ssh.Password(PASSWORD)},
		Config: ssh.Config{
			Ciphers: []string{"aes128-cbc", "3des-cbc", "aes256-cbc",
				"twofish256-cbc", "twofish-cbc", "twofish128-cbc", "blowfish-cbc"},
		},
	}

	client, err := ssh.Dial("tcp", ADDRESS, sshConfig)
	if err != nil {
		return nil, err
	}

	sklog.Infof("Dial successful")

	ret := &CmdClient{
		client: client,
	}

	if err := ret.ping(); err != nil {
		return nil, err
	}
	return ret, nil
}

func (c *CmdClient) RunExecMode(cmds []string) ([]string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	if err := session.Shell(); err != nil {
		return nil, err
	}

	inWriter, err := session.StdinPipe()
	if err != nil {
		return nil, err
	}
	defer util.Close(inWriter)
	outReader, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}

	sklog.Infof("Executing: enable")

	if err := session.Shell(); err != nil {
		return nil, fmt.Errorf("Error running enable: %s", err)
	}
	inWriter.Write([]byte("enable\n"))
	inWriter.Write([]byte(PASSWORD + "\n"))

	go func() {
		io.Read
		outReader.Rea
	}

	ret := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		sklog.Infof("Executing: %s", cmd)
		inWriter.Write([]byte(cmd + "\n"))

		outputBytes, err := session.CombinedOutput(cmd)
		if err != nil {
			return nil, err
		}
		if err != nil {
			return nil, fmt.Errorf("CMD: '%s'.  Got error: %s", cmd, err)
		}
		ret = append(ret, string(outputBytes))
	}
	return ret, nil
}

func (c *CmdClient) ping() error {
	sklog.Infof("Executing ping.")

	session, err := c.client.NewSession()
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

func main() {
	common.Init()

	c, err := NewCmdClient()
	if err != nil {
		log.Fatalf("Unable to create client. Got error: %s", err)
	}

	shutDownCmds := []string{
		"configure",
		"interface 0/20",
		"poe opmode shutdown",
		"exit",
		"exit",
		"show poe status all",
	}

	outputs, err := c.RunExecMode(shutDownCmds)
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	sklog.Infof("Got output:\n")
	for idx, output := range outputs {
		sklog.Infof("%3d: %s", idx, output)
	}
	sklog.Infof("Success.")
}
