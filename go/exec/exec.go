/*
	A wrapper around the os/exec package that supports timeouts and testing.

	Example usage:

	Simple command with argument:
	err := Run(&Command{
		Name: "touch",
		Args: []string{file},
	})

	More complicated example:
	output := bytes.Buffer{}
	err := Run(&Command{
		Name: "make",
		Args: []string{"all"},
		// Set environment:
		Env: []string{fmt.Sprintf("GOPATH=%s", projectGoPath)},
		// Set working directory:
		Dir: projectDir,
		// Capture output:
		CombinedOutput: &output,
		// Set a timeout:
		Timeout: 10*time.Minute,
	})

	Inject a Run function for testing:
	var actualCommand *Command
	SetRunForTesting(func(command *Command) error {
		actualCommand = command
		return nil
	})
	defer SetRunForTesting(DefaultRun)
	TestCodeCallingRun()
	expect.Equal(t, "touch", actualCommand.Name)
	expect.Equal(t, 1, len(actualCommand.Args))
	expect.Equal(t, file, actualCommand.Args[0])
*/
package exec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"strings"
	"time"

	"github.com/skia-dev/glog"
)

// WriteLog implements the io.Writer interface and writes to the given log function.
type WriteLog struct {
	LogFunc func(format string, args ...interface{})
}

func (wl WriteLog) Write(p []byte) (n int, err error) {
	wl.LogFunc("%s", string(p))
	return len(p), nil
}

var (
	WriteInfoLog  = WriteLog{LogFunc: glog.Infof}
	WriteErrorLog = WriteLog{LogFunc: glog.Errorf}
)

type Command struct {
	// Name of the command, as passed to osexec.Command. Can be the path to a binary or the
	// name of a command that osexec.Lookpath can find.
	Name string
	// Arguments of the command, not including Name.
	Args []string
	// The environment of the process. If nil, the current process's environment is used.
	Env []string
	// If Env is non-nil, adds the current process's PATH to Env.
	InheritPath bool
	// The working directory of the command. If nil, runs in the current process's current
	// directory.
	Dir string
	// See docs for osexec.Cmd.Stdin.
	Stdin io.Reader
	// If true, duplicates stdout of the command to WriteInfoLog.
	LogStdout bool
	// Sends the stdout of the command to this Writer, e.g. os.File or bytes.Buffer.
	Stdout io.Writer
	// If true, duplicates stderr of the command to WriteErrorLog.
	LogStderr bool
	// Sends the stderr of the command to this Writer, e.g. os.File or bytes.Buffer.
	Stderr io.Writer
	// Sends the combined stdout and stderr of the command to this Writer, in addition to
	// Stdout and Stderr. Only one goroutine will write at a time. Note: the Go runtime seems to
	// combine stdout and stderr into one stream as long as LogStdout and LogStderr are false
	// and Stdout and Stderr are nil. Otherwise, the stdout and stderr of the command could be
	// arbitrarily reordered when written to CombinedOutput.
	CombinedOutput io.Writer
	// Time limit to wait for the command to finish. (Starts when Wait is called.) No limit if
	// not specified.
	Timeout time.Duration
}

// Divides commandLine at spaces; treats the first token as the program name and the other tokens
// as arguments. Note: don't expect this function to do anything smart with quotes or escaped
// spaces.
func ParseCommand(commandLine string) Command {
	programAndArgs := strings.Split(commandLine, " ")
	return Command{Name: programAndArgs[0], Args: programAndArgs[1:]}
}

// Given io.Writers or nils, return a single writer that writes to all, or nil if no non-nil
// writers. Does not handle non-nil interface containing a nil value.
// http://devs.cloudimmunity.com/gotchas-and-common-mistakes-in-go-golang/index.html#nil_in_nil_in_vals
func squashWriters(writers ...io.Writer) io.Writer {
	nonNil := []io.Writer{}
	for _, writer := range writers {
		if writer != nil {
			nonNil = append(nonNil, writer)
		}
	}
	switch len(nonNil) {
	case 0:
		return nil
	case 1:
		return nonNil[0]
	default:
		return io.MultiWriter(nonNil...)
	}
}

func createCmd(command *Command) *osexec.Cmd {
	cmd := osexec.Command(command.Name, command.Args...)
	if len(command.Env) != 0 {
		cmd.Env = command.Env
		if command.InheritPath {
			cmd.Env = append(cmd.Env, "PATH="+os.Getenv("PATH"))
		}
	}
	cmd.Dir = command.Dir
	cmd.Stdin = command.Stdin
	var stdoutLog io.Writer
	if command.LogStdout {
		stdoutLog = WriteInfoLog
	}
	cmd.Stdout = squashWriters(stdoutLog, command.Stdout, command.CombinedOutput)
	var stderrLog io.Writer
	if command.LogStderr {
		stderrLog = WriteErrorLog
	}
	cmd.Stderr = squashWriters(stderrLog, command.Stderr, command.CombinedOutput)
	return cmd
}

func start(cmd *osexec.Cmd) error {
	if len(cmd.Env) == 0 {
		glog.Infof("Executing %s", strings.Join(cmd.Args, " "))
	} else {
		glog.Infof("Executing %s with env %s",
			strings.Join(cmd.Args, " "), strings.Join(cmd.Env, " "))
	}
	err := cmd.Start()
	if err != nil {
		glog.Errorf("Unable to start command %s: %s", strings.Join(cmd.Args, " "), err)
	}
	return err
}

func waitSimple(cmd *osexec.Cmd) error {
	err := cmd.Wait()
	if err != nil {
		glog.Errorf("Command exited with %s: %s", err, strings.Join(cmd.Args, " "))
	}
	return err
}

func wait(command *Command, cmd *osexec.Cmd) error {
	if command.Timeout == 0 {
		return waitSimple(cmd)
	}
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case <-time.After(command.Timeout):
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("Failed to kill timed out process: %s", err)
		}
		<-done // allow goroutine to exit
		glog.Errorf("Command killed since it took longer than %f secs", command.Timeout.Seconds())
		return fmt.Errorf("Command killed since it took longer than %f secs", command.Timeout.Seconds())
	case err := <-done:
		if err != nil {
			glog.Errorf("Command exited with %s: %s", err, strings.Join(cmd.Args, " "))
		}
		return err
	}
}

// Default value of Run.
func DefaultRun(command *Command) error {
	cmd := createCmd(command)
	if err := start(cmd); err != nil {
		return err
	}
	return wait(command, cmd)
}

// Run runs command and waits for it to finish. If any failure, returns non-nil. If a timeout was
// specified, returns an error once the command has exceeded that timeout.
var Run func(command *Command) error = DefaultRun

// SetRunForTesting replaces the Run function with a test version so that commands don't actually
// run.
func SetRunForTesting(testRun func(command *Command) error) {
	Run = testRun
}

// Run method is convenience for Run(command).
func (command *Command) Run() error {
	return Run(command)
}

// RunSimple executes the given command line string; the command being run is expected to not care
// what its current working directory is. Returns the combined stdout and stderr. May also return
// an error if the command exited with a non-zero status or there is any other error.
func RunSimple(commandLine string) (string, error) {
	command := ParseCommand(commandLine)
	output := bytes.Buffer{}
	command.CombinedOutput = &output
	err := Run(&command)
	result := string(output.Bytes())
	glog.Infof("StdOut + StdErr: %s\n", result)
	return result, err
}
