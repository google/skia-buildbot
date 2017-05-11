/*
	A wrapper around the os/exec package that supports testing.

	Example usage:

	Simple command with argument:
	err := skexec.NewExec().Run(&skexec.Command{
		Name: "touch",
		Args: []string{file},
	})

	Retrieving output:
	out, err := skexec.NewExec().GetOutput(&skexec.Command{
		Name: "uname",
		Args: []string{"-s"},
	})

	Simplified invocations:
	out, err := skexec.NewExec().RunSimple("grep flags /proc/cpuinfo")
	out, err := skexec.NewExec().RunCwd("/tmp", "ls", "-a" "-t")

	More complicated example:
	exec := skexec.NewExec()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output := bytes.Buffer{}
	err := exec.Run(&skexec.Command{
		Name: "make",
		Args: []string{"all"},
		// Set environment:
		Env: []string{fmt.Sprintf("GOPATH=%s", projectGoPath)},
		// Set working directory:
		Dir: projectDir,
		// Capture output:
		Stdout: &output,
		Stderr: &output,
		Context: ctx,
	})

	It is recommended to create a package-global variable initalized with NewExec to allow
	injecting a mock Run function for testing without interfering with other packages.
	Code:
	var exec = skexec.NewExec()
	func TouchIt() error {
		return exec.Run(skexec.Command{
			Name: "touch",
			Args: []string{"/tmp/file"},
		})
	}

	Test:
	mock := skexec_testutils.Mock{}
	exec.SetRun(mock.Run)
	defer exec.Reset()
	assert.NoError(TouchIt())
	assert.Equal(t, 1, len(mock.Commands()))
	actualCommand := mock.Commands()[0]
	assert.Equal(t, "touch", actualCommand.Name)
	assert.Equal(t, 1, len(actualCommand.Args))
	assert.Equal(t, file, actualCommand.Args[0])
*/
package skexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/util/limitwriter"
)

type Verbosity int

const (
	Debug Verbosity = iota
	Info
	Silent
)

const (
	// Command output will be limited to this many bytes. Does not apply to Command.Stdout and
	// Command.Stderr io.Writers.
	OUTPUT_LIMIT = 64 * 1024
)

var (
	// OutputLimitExceeded is returned by DefaultRun if the output exceeds OUTPUT_LIMIT.
	OutputLimitExceeded = errors.New("Command output exceeded limit.")

	WriteDebugLog = WriteLog{LogFunc: sklog.Debugf}
	WriteInfoLog  = WriteLog{LogFunc: sklog.Infof}
	WriteErrorLog = WriteLog{LogFunc: sklog.Errorf}
)

// Run runs command and waits for it to finish. Returns a non-nil error if any failure occurred.
type Run func(command *Command) error

// WriteLog implements the io.Writer interface and writes to the given log function.
type WriteLog struct {
	LogFunc func(format string, args ...interface{})
}

func (wl WriteLog) Write(p []byte) (n int, err error) {
	wl.LogFunc("%s", string(p))
	return len(p), nil
}

type Command struct {
	// Name of the command, as passed to exec.Command. Can be the path to a binary or the name
	// of a command that exec.Lookpath can find.
	Name string
	// Arguments of the command, not including Name.
	Args []string
	// The environment of the process. If nil, the current process's environment is used.
	Env []string
	// If Env is non-nil, adds the current process's entire environment to Env, excluding
	// variables that are set in Env.
	InheritEnv bool
	// If Env is non-nil, adds the current process's PATH to Env. Do not include PATH in Env.
	InheritPath bool
	// The working directory of the command. If nil, runs in the current process's current
	// directory.
	Dir string
	// See docs for exec.Cmd.Stdin.
	Stdin io.Reader
	// If true, duplicates stdout of the command to WriteInfoLog/WriteDebugLog, based on
	// Verbose.
	LogStdout bool
	// Sends the stdout of the command to this Writer, e.g. os.File or bytes.Buffer.
	// Note: The Go runtime seems to combine Stdout and Stderr into one stream if they are the
	// same, as long as LogStdout and LogStderr are false. Otherwise, the stdout and stderr of
	// the command could be arbitrarily reordered.
	Stdout io.Writer
	// If true, duplicates stderr of the command to WriteErrorLog.
	LogStderr bool
	// Sends the stderr of the command to this Writer, e.g. os.File or bytes.Buffer.
	// Note: The Go runtime seems to combine Stdout and Stderr into one stream if they are the
	// same, as long as LogStdout and LogStderr are false. Otherwise, the stdout and stderr of
	// the command could be arbitrarily reordered.
	Stderr io.Writer
	// If Context.Done() is closed before the command starts, returns Context.Err(). If
	// Context.Done() is closed before the command finishes, kills the command.
	Context context.Context
	// Whether to log when the command starts.
	Verbose Verbosity
}

func (c *Command) Append(args ...string) {
	c.Args = append(c.Args, args...)
}

// Divides commandLine at spaces; treats the first token as the program name and the other tokens
// as arguments. Note: don't expect this function to do anything smart with quotes or escaped
// spaces.
func ParseCommand(commandLine string) Command {
	programAndArgs := strings.Split(commandLine, " ")
	return Command{Name: programAndArgs[0], Args: programAndArgs[1:]}
}

// Returns the Env, Name, and Args of command joined with spaces. Does not perform any quoting.
func DebugString(command *Command) string {
	result := ""
	result += strings.Join(command.Env, " ")
	if len(command.Env) != 0 {
		result += " "
	}
	result += command.Name
	if len(command.Args) != 0 {
		result += " "
	}
	result += strings.Join(command.Args, " ")
	return result
}

// createCmd creates an os/exec.Cmd from command.
func createCmd(command *Command) *exec.Cmd {
	var cmd *exec.Cmd
	if command.Context == nil {
		cmd = exec.Command(command.Name, command.Args...)
	} else {
		cmd = exec.CommandContext(command.Context, command.Name, command.Args...)
	}
	if len(command.Env) != 0 {
		cmd.Env = command.Env
		if command.InheritEnv {
			existing := make(map[string]bool, len(command.Env))
			for _, s := range command.Env {
				existing[strings.SplitN(s, "=", 2)[0]] = true
			}
			for _, s := range os.Environ() {
				if !existing[strings.SplitN(s, "=", 2)[0]] {
					cmd.Env = append(cmd.Env, s)
				}
			}
		} else if command.InheritPath {
			cmd.Env = append(cmd.Env, "PATH="+os.Getenv("PATH"))
		}
	}
	cmd.Dir = command.Dir
	cmd.Stdin = command.Stdin

	joinOutput := func(logWriter io.Writer, stdWriter io.Writer) io.Writer {
		if stdWriter != nil && logWriter != nil {
			return io.MultiWriter(stdWriter, limitwriter.New(logWriter, OUTPUT_LIMIT))
		} else if stdWriter != nil {
			return stdWriter
		} else if logWriter != nil {
			return limitwriter.New(logWriter, OUTPUT_LIMIT)
		} else {
			return nil
		}
	}

	var outLogWriter io.Writer
	if command.LogStdout {
		if command.Verbose == Info {
			outLogWriter = WriteInfoLog
		} else {
			outLogWriter = WriteDebugLog
		}
	}
	cmd.Stdout = joinOutput(outLogWriter, command.Stdout)

	var errLogWriter io.Writer
	if command.LogStderr {
		errLogWriter = WriteErrorLog
	}
	cmd.Stderr = joinOutput(errLogWriter, command.Stderr)

	return cmd
}

func start(command *Command, cmd *exec.Cmd) error {
	if command.Verbose != Silent {
		dirMsg := ""
		if cmd.Dir != "" {
			dirMsg = " with CWD " + cmd.Dir
		}
		if command.Verbose == Info {
			sklog.Infof("Executing '%s' (where %s is %s)%s", DebugString(command), command.Name, cmd.Path, dirMsg)
		} else if command.Verbose == Debug {
			sklog.Debugf("Executing '%s' (where %s is %s)%s", DebugString(command), command.Name, cmd.Path, dirMsg)
		}

	}
	err := cmd.Start()
	if err != nil {
		return err
	}
	return nil
}

func wait(command *Command, cmd *exec.Cmd) error {
	err := cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

// DefaultRun is the standard implementation for Run.
func DefaultRun(command *Command) error {
	cmd := createCmd(command)
	if err := start(command, cmd); err != nil {
		if command.Verbose != Silent {
			sklog.Errorf("Unable to start command %s: %s", DebugString(command), err)
		}
		return err
	}
	return cmd.Wait()
}

// Exec provides various methods for running Commands and allows using a mock command runner.
type Exec struct {
	run Run
}

// NewExec returns an Exec that will run commands using DefaultRun.
func NewExec() *Exec {
	return &Exec{
		run: DefaultRun,
	}
}

// SetRun replaces the Exec's current Run with another implementation to allow using a mock command
// runner. See example in the comment at the beginning of this file.
func (e *Exec) SetRun(r Run) {
	e.run = r
}

// Reset is equivalent to SetRun(DefaultRun). It is useful to defer this method immediately after
// calling SetRun. See example in the comment at the beginning of this file.
func (e *Exec) Reset() {
	e.run = DefaultRun
}

// Run runs command and waits for it to finish. Returns a non-nil error if any failure occurred.
// This method either calls the Run passed to SetRun or DefaultRun after cleaning up command.
func (e Exec) Run(command *Command) error {
	if command.Context != nil && util.IsNil(command.Context) {
		command.Context = nil
	}
	if command.Stdin != nil && util.IsNil(command.Stdin) {
		command.Stdin = nil
	}
	if command.Stdout != nil && util.IsNil(command.Stdout) {
		command.Stdout = nil
	}
	if command.Stderr != nil && util.IsNil(command.Stderr) {
		command.Stderr = nil
	}
	return e.run(command)
}

// GetOutput runs command and waits for it to finish, returning the stdout and/or stderr if
// command.Stdout and/or command.Stderr are/is nil. Also returns a non-nil error if any failure
// occurred.
func (e Exec) GetOutput(command *Command) (string, error) {
	resultOutput := bytes.Buffer{}
	limitOutput := limitwriter.New(&resultOutput, OUTPUT_LIMIT)
	resultHasStdout := false
	if util.IsNil(command.Stdout) {
		command.Stdout = limitOutput
		resultHasStdout = true
	}
	resultHasStderr := false
	if util.IsNil(command.Stderr) {
		command.Stderr = limitOutput
		resultHasStderr = true
	}
	err := e.Run(command)
	result := resultOutput.String()
	overrun := limitOutput.Overrun()
	if err != nil {
		if command.Verbose != Silent {
			resultForLog := bytes.Buffer{}
			if result != "" && !(command.LogStdout && command.LogStderr) {
				resultForLog.WriteString("; ")
				if !resultHasStdout {
					resultForLog.WriteString("Stderr")
				} else if !resultHasStderr {
					resultForLog.WriteString("Stdout")
				} else {
					resultForLog.WriteString("Stdout+Stderr")
				}
				if overrun > 0 {
					resultForLog.WriteString(" (truncated)")
				}
				resultForLog.WriteString(":\n")
				resultForLog.WriteString(result)
			}
			sklog.Errorf("Command exited with %s: %s%s", err, DebugString(command), resultForLog.String())
		}
		if err == context.Canceled || err == context.DeadlineExceeded || result == "" {
			// Preserve original error.
			return result, err
		}
		return result, fmt.Errorf("%s; Stdout+Stderr:\n%s", err.Error(), result)
	}
	if overrun > 0 {
		sklog.Errorf("Stdout+Stderr exceeded limit of %d by %d bytes. If this is expected, specify an io.Writer for Command.Stdout/Command.Stderr. Command: %s\nTruncated Stdout+Stderr:\n%s", OUTPUT_LIMIT, overrun, DebugString(command), result)
		return result, OutputLimitExceeded
	}
	return result, nil
}

// RunCommand is a synonym for GetOutput.
func (e Exec) RunCommand(command *Command) (string, error) {
	return e.GetOutput(command)
}

// RunSimple executes the given command line string; the command being run is expected to not care
// what its current working directory is. Returns the combined stdout and stderr. May also return
// an error if the command exited with a non-zero status or there is any other error. Note: don't
// expect this function to do anything smart with quotes or escaped spaces.
func (e Exec) RunSimple(commandLine string) (string, error) {
	cmd := ParseCommand(commandLine)
	return e.GetOutput(&cmd)
}

// RunCwd executes the given command in the given directory. Returns the combined stdout and
// stderr. May also return an error if the command exited with a non-zero status or there is any
// other error.
func (e Exec) RunCwd(cwd string, args ...string) (string, error) {
	command := &Command{
		Name: args[0],
		Args: args[1:],
		Dir:  cwd,
	}
	return e.GetOutput(command)
}

// RunAsync starts the command and then returns. Clients can listen for the command to end on the
// returned channel. Use command.Context with context.WithCancel to kill the process.
func (e Exec) RunAsync(command *Command) <-chan error {
	done := make(chan error, 1)
	go func() {
		err := e.Run(command)
		done <- err
		close(done)
	}()
	return done
}

// AsyncResult can be called on the return value of RunAsync. It returns the result and true if the
// Command has finished, otherwise nil and false.
func AsyncResult(done <-chan error) (error, bool) {
	select {
	case err := <-done:
		return err, true
	default:
		return nil, false
	}
}
