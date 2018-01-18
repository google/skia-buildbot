package ssh

/*
   High-level utility for running over SSH.
*/

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/ssh"
)

type Config struct {
	IpAddr   string
	Hostname string
	Port     string
	Username string
	Password string
}

type Shell struct {
	client  *ssh.Client
	config  *Config
	session *ssh.Session
	stdout  io.Reader
	stderr  io.Reader
	stdin   io.Writer
}

// NewShell creates an SSH connection to the given host and opens a login shell.
func NewShell(cfg *Config) (*Shell, error) {
	sshConfig := &ssh.ClientConfig{
		User: cfg.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(cfg.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO(borenet): NO!
	}

	// Open the connection.
	client, err := ssh.Dial("tcp", cfg.IpAddr+":"+cfg.Port, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to dial: %s", err)
	}

	shell := &Shell{
		client: client,
		config: cfg,
	}
	return shell, nil
}

// Close closes the Shell.
func (s *Shell) Close() error {
	err3 := s.client.Close()
	return err3
}

// WaitFor waits for the given rune to appear in the stream and then returns.
func (s *Shell) waitFor(signal string) error {
	matchIdx := 0
	for {
		buf := make([]byte, 128)
		n, err := s.stdout.Read(buf)
		if n > 0 {
			for _, b := range buf {
				if b == signal[matchIdx] {
					matchIdx++
				} else {
					matchIdx = 0
				}
				if matchIdx >= len(signal) {
					return nil
				}
			}
		}
		if err != nil {
			return fmt.Errorf("Encountered error while waiting for %q: %s", signal, err)
		}
	}
	return nil
}

// WaitForShellPrompt waits for the shell prompt to appear.
func (s *Shell) waitForShellPrompt() error {
	// TODO(borenet): This is not very robust...
	return s.waitFor(fmt.Sprintf("%s@%s:~$ ", s.config.Username, s.config.Hostname))
}

// run the given command without waiting.
func (s *Shell) run(cmd string) error {
	if !strings.HasSuffix(cmd, "\n") {
		cmd += "\n"
	}
	if _, err := s.stdin.Write([]byte(cmd)); err != nil {
		return fmt.Errorf("Failed to write command to connection: %s", err)
	}
	return nil
}

// Run runs the given command and waits for the shell prompt to appear again.
func (s *Shell) Run(cmd string) error {
	session, err := s.client.NewSession()
	if err != nil {
		return err
	}
	_, err := session.CombinedOutput(cmd)
	return err
}

// RunWithPassword runs the given command, which is expected to prompt for
// password, then enters the password and waits for the shell prompt to appear
// again.
func (s *Shell) RunWithPassword(cmd string) error {
	if err := s.run(cmd); err != nil {
		return err
	}
	// TODO(borenet): This is not very robust...
	if err := s.waitFor(fmt.Sprintf("[sudo] password for %s:", s.config.Username)); err != nil {
		return err
	}
	if _, err := s.stdin.Write([]byte(s.config.Password + "\n")); err != nil {
		return err
	}
	return s.waitForShellPrompt()
}
