package powercycle

import (
	"bytes"
	"context"
	"io"
	"strings"
	"time"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/skerr"
)

// execTimeout is the timeout when we exec a command over SSH.
const execTimeout = 10 * time.Second

// The CommandRunner interface adds a layer of abstraction around sending commands to powercycle
// Controllers. It is not meant to be a general purpose interface or a robust implementation beyond
// exactly that use.
type CommandRunner interface {
	// ExecCmds executes a series of commands and returns the accumulated output of all commands.
	// If one command fails, an error is returned, but no other guarantees are made.
	ExecCmds(ctx context.Context, cmds ...string) (string, error)
}

// stdinRunner implements the CommandRunner interface by sending commands through standard input
// to the given executable running with the given args.
type stdinRunner struct {
	executable string
	args       []string
}

// PublicKeySSHCommandRunner returns a CommandRunner that will operate over a native ssh binary
// with the following arguments. One of the provided arguments should be the user/ip address.
// It presumes that the target is configured to authenticate via a shared public key (e.g. in
// .ssh/authorized_keys), as it does not expect or support ssh prompting for a password.
func PublicKeySSHCommandRunner(sshArgs ...string) *stdinRunner {
	return &stdinRunner{
		executable: "ssh",
		args:       sshArgs,
	}
}

// PasswordSSHCommandRunner returns a CommandRunner that will operate over a native ssh binary
// with the following arguments. One of the provided arguments should be the user/ip address.
// It passes the password into ssh via sshpass. See
// http://manpages.ubuntu.com/manpages/trusty/man1/sshpass.1.html for more details on why sshpass
// is needed to give the password to ssh.
// Note: ssh is known to return errors even when the command executed normally. To work around
// this, ignore the error returned by ExecCmds and look at the standard out.
func PasswordSSHCommandRunner(password string, sshArgs ...string) *stdinRunner {
	args := append([]string{"-p", password, "ssh"}, sshArgs...)
	return &stdinRunner{
		executable: "sshpass",
		args:       args,
	}
}

// ExecCmds implements the CommandRunner interface. It makes a connection to the
// target and then feeds the commands into standard in joined by newlines. It
// returns any output it receives and any errors.
func (s *stdinRunner) ExecCmds(ctx context.Context, cmds ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()
	cmd := executil.CommandContext(ctx, s.executable, s.args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", skerr.Wrapf(err, "getting stdin pipe")
	}

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	// Start the command before sending to stdin just in case we try to send
	// more data to standard input than it can take (~4k).
	if err := cmd.Start(); err != nil {
		return "", skerr.Wrapf(err, "starting executable %s %s", s.executable, s.args)
	}

	// Commands sent via standard in are executed after a newline is seen.
	cmdStr := strings.Join(cmds, "\n") + "\n"
	if _, err := io.WriteString(stdin, cmdStr); err != nil {
		return "", skerr.Wrapf(err, "sending command %q to stdin", cmdStr)
	}

	// SSH will keep running until stdin is closed, so we need to close it before we Wait, otherwise
	// Wait will block forever.
	if err := stdin.Close(); err != nil {
		return "", skerr.Wrapf(err, "closing stdin pipe")
	}

	if err := cmd.Wait(); err != nil {
		// combined could have valid input if err is non-nil, e.g. why it crashed.
		return combined.String(), skerr.Wrapf(err, "running %q", cmds)
	}
	return combined.String(), nil
}

var _ CommandRunner = (*stdinRunner)(nil)
