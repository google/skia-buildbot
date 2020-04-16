package powercycle

import (
	"context"
	"io"
	"os/exec"
	"strings"

	"go.skia.org/infra/go/skerr"
)

// The CommandRunner interface adds a layer of abstraction around a device
// that can have commands executed on it.
type CommandRunner interface {
	// ExecCmds executes a series of commands and returns the accumulated output of all commands.
	ExecCmds(ctx context.Context, cmds ...string) (string, error)
}

type standardInRunner struct {
	cmd  string
	args []string
}

// PublicKeySSHCommandRunner returns a CommandRunner that will operate over a native ssh binary
// with the following arguments. One of the provided arguments should be the user/ip address.
// It presumes that the target is configured to authenticate via a shared public key (e.g. in
// .ssh/authorized_keys), as it does not expect or support ssh prompting for a password.
func PublicKeySSHCommandRunner(sshArgs ...string) *standardInRunner {
	return &standardInRunner{
		cmd:  "ssh",
		args: sshArgs,
	}
}

// PasswordSSHCommandRunner returns a CommandRunner that will operate over a native ssh binary
// with the following arguments. One of the provided arguments should be the user/ip address.
// It passes the password into ssh via sshpass. See
// http://manpages.ubuntu.com/manpages/trusty/man1/sshpass.1.html for more details on why sshpass
// is needed to give the password to ssh.
func PasswordSSHCommandRunner(password string, sshArgs ...string) *standardInRunner {
	args := append([]string{"-p", password, "ssh"}, sshArgs...)
	return &standardInRunner{
		cmd:  "sshpass",
		args: args,
	}
}

// ExecCmds implements the CommandRunner interface. It makes a connection to the target and then
// feeds the commands into standard in joined by newlines. It returns any output it receives and
// any errors.
func (s *standardInRunner) ExecCmds(ctx context.Context, cmds ...string) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.cmd, s.args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", skerr.Wrapf(err, "getting stdin pipe")
	}

	// Commands sent via standard in are typically executed after a newline is seen.
	cmdStr := strings.Join(cmds, "\n") + "\n"
	if _, err := io.WriteString(stdin, cmdStr); err != nil {
		return "", skerr.Wrapf(err, "sending command %q to stdin", cmdStr)
	}

	// SSH will keep running until stdin is closed, so we need to close it before we start, otherwise
	// CombinedOutput will block forever.
	if err := stdin.Close(); err != nil {
		return "", skerr.Wrapf(err, "closing stdin pipe")
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		// out could have valid input if err is non-nil, e.g. why it crashed.
		return string(out), skerr.Wrapf(err, "running %q", cmds)
	}
	return string(out), nil
}

var _ CommandRunner = (*standardInRunner)(nil)
