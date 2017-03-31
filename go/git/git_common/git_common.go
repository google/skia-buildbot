package git_common

/*
   Common elements used by go/git and go/git/testutils.
*/

import (
	"fmt"
	"regexp"
	"strconv"

	"go.skia.org/infra/go/exec"
)

var (
	gitVersionRegex = regexp.MustCompile("git version (\\d+)\\.(\\d+)\\..*")
)

// Version returns the installed Git version, in the form:
// (major, minor), or an error if it could not be determined.
func Version() (int, int, error) {
	out, err := exec.RunCwd(".", "git", "--version")
	if err != nil {
		return -1, -1, err
	}
	m := gitVersionRegex.FindStringSubmatch(out)
	if m == nil {
		return -1, -1, fmt.Errorf("Failed to parse the git version from output: %q", out)
	}
	if len(m) != 3 {
		return -1, -1, fmt.Errorf("Failed to parse the git version from output: %q", out)
	}
	major, err := strconv.Atoi(m[1])
	if err != nil {
		return -1, -1, fmt.Errorf("Failed to parse the git version from output: %q", out)
	}
	minor, err := strconv.Atoi(m[2])
	if err != nil {
		return -1, -1, fmt.Errorf("Failed to parse the git version from output: %q", out)
	}
	return major, minor, nil
}
