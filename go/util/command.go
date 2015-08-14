package util

import (
	"go.skia.org/infra/go/exec"
)

// DoCmd executes the given command line string; the command being
// run is expected to not care what its current working directory is.
// Returns the stdout and stderr.
func DoCmd(commandLine string) (string, error) {
	return exec.RunSimple(commandLine)
}
