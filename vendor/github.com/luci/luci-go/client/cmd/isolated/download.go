// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"

	"github.com/maruel/subcommands"
)

var cmdDownload = &subcommands.Command{
	UsageLine: "download <options>...",
	ShortDesc: "downloads a file or a .isolated tree from an isolate server.",
	LongDesc: `Downloads one or multiple files, or a isolated tree from the isolate server.

Files are referenced by their hash`,
	CommandRun: func() subcommands.CommandRun {
		c := downloadRun{}
		c.commonFlags.Init()
		return &c
	},
}

type downloadRun struct {
	commonFlags
}

func (c *downloadRun) Parse(a subcommands.Application, args []string) error {
	if err := c.commonFlags.Parse(); err != nil {
		return err
	}
	if len(args) != 0 {
		return errors.New("position arguments not expected")
	}
	return nil
}

func (c *downloadRun) main(a subcommands.Application, args []string) error {
	return errors.New("TODO")
}

func (c *downloadRun) Run(a subcommands.Application, args []string) int {
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
