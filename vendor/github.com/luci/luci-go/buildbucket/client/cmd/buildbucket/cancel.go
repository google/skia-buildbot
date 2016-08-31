// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/maruel/subcommands"

	"github.com/luci/luci-go/common/cli"
)

var cmdCancel = &subcommands.Command{
	UsageLine: `cancel [flags] <build id>`,
	ShortDesc: "Cancel a build",
	LongDesc:  "Attempt to cancel an existing build.",
	CommandRun: func() subcommands.CommandRun {
		c := &cancelRun{}
		c.SetDefaultFlags()
		return c
	},
}

type cancelRun struct {
	baseCommandRun
	buildIDArg
}

func (r *cancelRun) Run(a subcommands.Application, args []string) int {
	ctx := cli.GetContext(a, r)

	if err := r.parseArgs(args); err != nil {
		return r.done(ctx, err)
	}

	return r.callAndDone(ctx, "POST", fmt.Sprintf("builds/%d/cancel", r.buildID), nil)
}
