package util

import (
	"os/exec"
	"strings"

	"github.com/skia-dev/glog"
)

// DoCmd executes the given command line string; the command being
// run is expected to not care what its current working directory is.
// Returns the stdout and stderr.  If there is an error, the
// returned error will be of type ExitError, which the caller
// can use to find out more about what happened.
func DoCmd(commandLine string) (string, error) {
	glog.Infof("Command: %q\n", commandLine)
	programAndArgs := strings.SplitN(commandLine, " ", 2)
	program := programAndArgs[0]
	args := []string{}
	if len(programAndArgs) > 1 {
		args = strings.Split(programAndArgs[1], " ")
	}
	cmd := exec.Command(program, args...)
	message, err := cmd.CombinedOutput()
	glog.Infof("StdOut + StdErr: %s\n", string(message))
	if err != nil {
		glog.Errorf("Exit status: %s\n", err)
		return string(message), err
	}
	return string(message), nil
}
