// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package version

import (
	"fmt"
	"os"

	"github.com/maruel/subcommands"
)

// SubcommandVersion implement subcommand that prints version of CIPD package
// that contains the executable.
var SubcommandVersion = &subcommands.Command{
	UsageLine:  "version",
	ShortDesc:  "prints version of CIPD package this exe was installed from",
	LongDesc:   "Prints version of CIPD package this exe was installed from.",
	CommandRun: func() subcommands.CommandRun { return &versionRun{} },
}

type versionRun struct {
	subcommands.CommandRunBase
}

func (c *versionRun) Run(a subcommands.Application, args []string) int {
	ver, err := GetStartupVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}
	if ver.InstanceID == "" {
		fmt.Fprintf(os.Stderr, "Not installed via CIPD package\n")
		return 1
	}
	fmt.Printf("Package name: %s\n", ver.PackageName)
	fmt.Printf("Instance ID:  %s\n", ver.InstanceID)
	return 0
}
