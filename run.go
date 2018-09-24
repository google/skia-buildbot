package main

import (
	"errors"

	"go.skia.org/infra/go/sklog"
	t "go.skia.org/infra/go/test_automation"
)

var (
	SCOPES = []string{}
)

func pwd(c *t.Context) error {
	out, err := c.Exec("pwd")
	sklog.Infof("pwd: %s", out)
	return err
}

func main() {
	t.Main(SCOPES, func(c *t.Context) error {
		// Run a command.
		if _, err := c.Exec("echo", "hello world"); err != nil {
			return err
		}

		// Ignore a failed step.
		if err := c.Run("failed", func() error {
			return errors.New("bad bad bad")
		}); err != nil {
			sklog.Warningf("Ignoring error: %s", err)
		}

		// Run a function as a step in a sub-context.
		if err := c.Infra().Cwd("go/test_automation").Run("say hi", func() error {
			sklog.Infof("hi")
			return nil
		}); err != nil {
			sklog.Warningf("Ignoring error: %s", err)
		}

		// Run an arbitrary step.
		if err := c.RunStep(&t.Step{
			Name:  "start",
			Infra: false,
			Fn: func() error {
				sklog.Infof("blah blah")
				return nil
			},
		}); err != nil {
			return err
		}

		// We can pass a context to a function.
		if err := pwd(c); err != nil {
			return err
		}

		// Enter a new context, and run some steps.
		if err := c.Cwd("go").Do(func(c2 *t.Context) error {
			if err := pwd(c2); err != nil {
				return err
			}
			output, err := c2.Exec("pwd")
			if err != nil {
				return err
			}
			sklog.Infof("pwd: %s", output)
			return nil
		}); err != nil {
			return err
		}
		return nil
	})
}
