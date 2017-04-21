/*
	Types and functions to help with testing code that runs commands.
*/
package skexec_testutils

import (
	"fmt"
	"strings"

	"regexp"
	"sync"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skexec"
)

// RunCwd runs the given command in the given dir and asserts that it succeeds.
func RunCwd(t assert.TestingT, dir string, cmd ...string) string {
	out, err := skexec.NewExec().RunCwd(dir, cmd...)
	assert.NoError(t, err, fmt.Sprintf("Command %q failed:\n%s", strings.Join(cmd, " "), out))
	return out
}

// Mock collects arguments to the Run method for later inspection. The stdout and/or error return
// value of the command can be set based on regexp match of skexec.DebugString(command). Safe for
// use in multiple goroutines.
// Example usage:
//      exec := skexec.NewExec()
// 	mock := skexec_testutils.Mock{}
//	exec.SetRun(mock.Run)
//	defer exec.Reset()
//
//	assert.NoError(t, exec.Run(&skexec.Command{
//		Name: "touch",
//		Args: []string{"/tmp/file"},
//	}))
//	assert.Equal(t, "touch /tmp/file"", skexec.DebugString(mock.Commands()[0]))
//
//	mock.AddRule("echo Hello", "Hello\n", nil)
//	mock.AddRule("^touch /tmp/bar$", "", fmt.Errorf("baz"))
//
//	out, err := exec.GetOutput(&skexec.Command{
//		Name: "echo",
//		Args: []string{"Hello"},
//	})
//	assert.NoError(t, err)
//	assert.Equal(t, "Hello\n", out)
//
//	assert.NoError(t, exec.Run(&skexec.Command{
//		Name: "touch",
//		Args: []string{"/tmp/foo"},
//	}))
//	err = exec.Run(&skexec.Command{
//		Name: "touch",
//		Args: []string{"/tmp/bar"},
//	})
//	assert.Error(t, err)
//	assert.Contains(t, err.Error(), "baz")
type Mock struct {
	mutex      sync.RWMutex
	commands   []*skexec.Command
	defaultRun skexec.Run
	rules      []mockRule
}

type mockRule struct {
	matcher *regexp.Regexp
	out     string
	err     error
}

func (m *Mock) Commands() []*skexec.Command {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make([]*skexec.Command, len(m.commands))
	copy(result, m.commands)
	return result
}

func (m *Mock) ClearCommands() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.commands = nil
}

// Panics if expr is not a valid regexp.
func (m *Mock) AddRule(expr string, out string, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.rules = append(m.rules, mockRule{
		matcher: regexp.MustCompile(expr),
		out:     out,
		err:     err,
	})
}

// SetDefaultRun causes the given function to be called if no rule matches. When this function is
// called, the argument command will be visible in Commands().
func (m *Mock) SetDefaultRun(defaultRun skexec.Run) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.defaultRun = defaultRun
}

// Saves command for later inspection via Commands(). Tries to match skexec.DebugString(command)
// against the regexps in the order of the calls to AddRule, with the first matched giving the value
// to write to stdout and the return value. Calls the function provided to SetDefaultRun if no
// regexps match, or returns nil if no default Run has been set.
// If the rule specifies stdout and there is an error writing to command.Stdout, returns the error.
func (m *Mock) Run(command *skexec.Command) error {
	commandStr := skexec.DebugString(command)
	var defaultRun skexec.Run
	var matched bool
	var out string
	var err error
	func() {
		m.mutex.Lock()
		defer m.mutex.Unlock()
		m.commands = append(m.commands, command)
		defaultRun = m.defaultRun
		for _, rule := range m.rules {
			if rule.matcher.FindStringIndex(commandStr) != nil {
				matched, out, err = true, rule.out, rule.err
			}
		}
	}()
	if out != "" {
		if command.Stdout == nil {
			return fmt.Errorf("Command matched rule, but Stdout is nil! Command: %q; out: %q", commandStr, out)
		}
		_, err := command.Stdout.Write([]byte(out))
		if err != nil {
			return err
		}
	}
	if matched {
		return err
	} else if defaultRun == nil {
		return nil
	} else {
		return defaultRun(command)
	}
}
