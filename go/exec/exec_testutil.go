package exec

// This file contains helpers for working with exec in tests.

import (
	"context"
	"sync"
)

// CommandCollector collects arguments to the Run method for later inspection. Safe for use in
// multiple goroutines as long as the function passed to SetDelegateRun is.
//
// Example usage:
//
//    mock := exec.CommandCollector{}
//    ctx := exec.NewContext(context.Background(), mock.Run)
//    err := exec.Run(ctx, &exec.Command{
//      Name: "touch",
//      Args: []string{"/tmp/file"},
//    })
//    assert.Equal(t, "touch /tmp/file"", exec.DebugString(mock.Commands()[0]))
type CommandCollector struct {
	mutex       sync.RWMutex
	commands    []*Command
	delegateRun func(context.Context, *Command) error
}

// Commands returns a copy of the commands that have been run up to this point.
func (c *CommandCollector) Commands() []*Command {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	result := make([]*Command, len(c.commands))
	copy(result, c.commands)
	return result
}

// ClearCommands resets the commands seen thus far.
func (c *CommandCollector) ClearCommands() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.commands = nil
}

// SetDelegateRun allows some custom function to be executed when Run is called on this object.
// By default, nothing will happen apart from storing the command.
func (c *CommandCollector) SetDelegateRun(delegateRun func(context.Context, *Command) error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.delegateRun = delegateRun
}

// Run collects command into c and delegates to the function specified by SetDelegateRun.
// Returns nil if SetDelegateRun has not been called. The command will be visible in Commands()
// before the SetDelegateRun function is called.
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
