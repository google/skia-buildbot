package testutils

import (
	"fmt"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils"
)

// Run runs the given command in the given dir and asserts that it succeeds.
func Run(t assert.TestingT, dir string, cmd ...string) string {
	out, err := exec.RunCwd(dir, cmd...)
	assert.NoError(t, err, fmt.Sprintf("Command %q failed:\n%s", strings.Join(cmd, " "), out))
	return out
}

// MockCmd is a struct used for mocking out commands.
type MockCmd struct {
	expect  []*exec.Command
	mockErr []error
	mockOut []string
	t       *testing.T
}

// NewMockCmd returns a MockCmd instance.
func NewMockCmd(t *testing.T) *MockCmd {
	c := &MockCmd{
		expect:  []*exec.Command{},
		mockErr: []error{},
		mockOut: []string{},
		t:       t,
	}
	exec.SetRunForTesting(func(cmd *exec.Command) error {
		return c.run(cmd)
	})
	return c
}

// Mock adds the given command to the mock list.
func (c *MockCmd) Mock(cmd *exec.Command, out string, err error) {
	c.expect = append(c.expect, cmd)
	c.mockErr = append(c.mockErr, err)
	c.mockOut = append(c.mockOut, out)
}

// run pretends to run the given command.
func (c *MockCmd) run(cmd *exec.Command) error {
	assert.NotEmpty(c.t, c.expect, fmt.Sprintf("cmd: %v", cmd))
	assert.NotEmpty(c.t, c.mockErr, fmt.Sprintf("cmd: %v", cmd))
	assert.NotEmpty(c.t, c.mockOut, fmt.Sprintf("cmd: %v", cmd))
	expect := c.expect[0]
	mockErr := c.mockErr[0]
	mockOut := c.mockOut[0]
	testutils.AssertDeepEqual(c.t, expect, cmd)
	c.expect = c.expect[1:]
	c.mockErr = c.mockErr[1:]
	c.mockOut = c.mockOut[1:]
	if cmd.CombinedOutput != nil {
		n, err := cmd.CombinedOutput.Write([]byte(mockOut))
		assert.NoError(c.t, err)
		assert.Equal(c.t, n, len(mockOut))
	} else if mockOut != "" {
		assert.FailNow(c.t, "Output string provided but cmd.CombinedOutput is nil!")
	}
	return mockErr
}

// AssertEmpty asserts that all of the mocked commands have run.
func (c *MockCmd) AssertEmpty() {
	assert.Empty(c.t, c.expect)
	assert.Empty(c.t, c.mockErr)
	assert.Empty(c.t, c.mockOut)
}

// Cleanup cleans up the MockCmd.
func (c *MockCmd) Cleanup() {
	c.AssertEmpty()
	exec.SetRunForTesting(exec.DefaultRun)
}
