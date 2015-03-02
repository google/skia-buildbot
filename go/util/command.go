package util

import (
	"fmt"
	"github.com/skia-dev/glog"
	"os/exec"
	"strings"
)

// DoCmd executes the given command line string; the command being
// run is expected to not care what its current working directory is.
// Returns the stdout and stderr.
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
		return string(message), fmt.Errorf("Failed to run command.")
	}
	return string(message), nil
}
