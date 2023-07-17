package testutils

import (
	"context"
	"fmt"
	"strings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
)

// Run runs the given command in the given dir and asserts that it succeeds.
func Run(t require.TestingT, ctx context.Context, dir string, cmd ...string) string {
	out, err := exec.RunCwd(ctx, dir, cmd...)
	require.NoError(t, err, fmt.Sprintf("Command %q failed:\n%s", strings.Join(cmd, " "), out))
	return out
}

// AssertCommandsMatch does a quick comparison of the commands returned by exec.CommandCollector
// with the name and arguments listed as a space seperated list. Other details of a command (e.g.
// if they had standard out or set environment variables) should be handled in follow on, specific
// assertions. If the size of the command lists does not match, as many as possible will be compared
// before ending in a fatal manner.
func AssertCommandsMatch(t require.TestingT, expectedCommands [][]string, actualCommands []*exec.Command) {
	for i, cmd := range expectedCommands {
		if i >= len(actualCommands) {
			require.Fail(t, fmt.Sprintf("Ran out of actual commands to compare against after %s",
				expectedCommands[len(expectedCommands)-1]))
		}
		actualCmd := actualCommands[i]
		cli := make([]string, 1, 1+len(actualCmd.Args))
		cli[0] = actualCmd.Name
		cli = append(cli, actualCmd.Args...)
		assert.Equal(t, cmd, cli)
	}
	require.Equal(t, len(expectedCommands), len(actualCommands))
}
