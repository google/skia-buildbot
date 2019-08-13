/*
	Types and functions to help with testing code that uses exec.Run.
*/
package exec

import (
	"context"
	"regexp"
	"sync"
)

// CommandCollector collects arguments to the Run method for later inspection. Safe for use in
// multiple goroutines as long as the function passed to SetDelegateRun is.
// Example usage:
// 	mock := CommandCollector{}
//	SetRunForTesting(mock.Run)
//	defer SetRunForTesting(DefaultRun)
//	err := Run(&Command{
//		Name: "touch",
//		Args: []string{"/tmp/file"},
//	})
//	assert.Equal(t, "touch /tmp/file"", DebugString(mock.Commands()[0]))
type CommandCollector struct {
	mutex       sync.RWMutex
	commands    []*Command
	delegateRun func(context.Context, *Command) error
}

func (c *CommandCollector) Commands() []*Command {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	// TODO(benjaminwagner): Can I just return c.commands?
	result := make([]*Command, len(c.commands))
	copy(result, c.commands)
	return result
}

func (c *CommandCollector) ClearCommands() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.commands = nil
}

func (c *CommandCollector) SetDelegateRun(delegateRun func(context.Context, *Command) error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.delegateRun = delegateRun
}

// Collects command into c and delegates to the function specified by SetDelegateRun. Returns nil
// if SetDelegateRun has not been called. The command will be visible in Commands() before the
// SetDelegateRun function is called.
func (c *CommandCollector) Run(ctx context.Context, command *Command) error {
	c.mutex.Lock()
	c.commands = append(c.commands, command)
	delegateRun := c.delegateRun
	c.mutex.Unlock()
	if delegateRun == nil {
		return nil
	} else {
		return delegateRun(ctx, command)
	}
}

// Provides a Run method that returns based on regexp matches of DebugString(command). Safe for use
// in multiple goroutines.
// Example usage:
// 	mock := MockRun{}
//	SetRunForTesting(mock.Run)
//	defer SetRunForTesting(DefaultRun)
//	mock.AddRule("touch /tmp/bar", fmt.Errorf("baz"))
//	assert.NoError(t, Run(&Command{
//		Name: "touch",
//		Args: []string{"/tmp/foo"},
//	}))
//	err := Run(&Command{
//		Name: "touch",
//		Args: []string{"/tmp/bar"},
//	})
//	assert.Error(t, err)
//	assert.Contains(t, err.Error(), "baz")
type MockRun struct {
	mutex    sync.RWMutex
	matchers []*regexp.Regexp
	results  []error
}

// Panics if expr is not a valid regexp.
func (m *MockRun) AddRule(expr string, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.matchers = append(m.matchers, regexp.MustCompile(expr))
	m.results = append(m.results, err)
}

// Tries to match DebugString(command) against the regexps in the order of the calls to AddRule,
// with the first matched giving the return value. Returns nil if no regexps match.
func (m *MockRun) Run(ctx context.Context, command *Command) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	commandStr := DebugString(command)
	for i, matcher := range m.matchers {
		if matcher.FindStringIndex(commandStr) != nil {
			return m.results[i]
		}
	}
	return nil
}
