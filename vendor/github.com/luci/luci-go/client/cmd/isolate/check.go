// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/maruel/subcommands"
)

var cmdCheck = &subcommands.Command{
	UsageLine: "check <options>",
	ShortDesc: "checks that all the inputs are present and generates .isolated",
	LongDesc:  "",
	CommandRun: func() subcommands.CommandRun {
		c := checkRun{}
		c.commonFlags.Init()
		c.isolateFlags.Init(&c.Flags)
		return &c
	},
}

type checkRun struct {
	commonFlags
	isolateFlags
}

func (c *checkRun) Parse(a subcommands.Application, args []string) error {
	if err := c.commonFlags.Parse(); err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := c.isolateFlags.Parse(cwd, RequireIsolatedFile|RequireIsolateFile); err != nil {
		return err
	}
	if len(args) != 0 {
		return errors.New("position arguments not expected")
	}
	return nil
}

func (c *checkRun) main(a subcommands.Application, args []string) error {
	if !c.defaultFlags.Quiet {
		fmt.Printf("Isolate:   %s\n", c.Isolate)
		fmt.Printf("Isolated:  %s\n", c.Isolated)
		fmt.Printf("Blacklist: %s\n", c.Blacklist)
		fmt.Printf("Config:    %s\n", c.ConfigVariables)
		fmt.Printf("Path:      %s\n", c.PathVariables)
		fmt.Printf("Extra:     %s\n", c.ExtraVariables)
	}
	return errors.New("TODO")
}

func (c *checkRun) Run(a subcommands.Application, args []string) int {
	if err := c.Parse(a, args); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	cl, err := c.defaultFlags.StartTracing()
	if err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	defer cl.Close()
	if err := c.main(a, args); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}
