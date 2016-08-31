// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package common

import (
	"fmt"

	"github.com/maruel/subcommands"
)

// CmdVersion returns a generic "version" subcommand printing the version given.
func CmdVersion(version string) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "version <options>",
		ShortDesc: "prints version number",
		LongDesc:  "Prints the tool version number.",
		CommandRun: func() subcommands.CommandRun {
			return &versionRun{version: version}
		},
	}
}

type versionRun struct {
	subcommands.CommandRunBase
	version string
}

func (c *versionRun) Run(a subcommands.Application, args []string) int {
	if len(args) != 0 {
		fmt.Fprintf(a.GetErr(), "%s: position arguments not expected\n", a.GetName())
		return 1
	}
	fmt.Println(c.version)
	return 0
}
