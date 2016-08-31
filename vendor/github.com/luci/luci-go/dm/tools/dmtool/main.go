// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/maruel/subcommands"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/cli"
	"github.com/luci/luci-go/common/logging/gologger"
)

// cmdRun is a base of all rpc subcommands.
type cmdRun struct {
	subcommands.CommandRunBase
	cmd *subcommands.Command
}

var logCfg = gologger.LoggerConfig{
	Format: `%{message}`,
	Out:    os.Stderr,
}

// argErr prints an err and usage to stderr and returns an exit code.
func (r *cmdRun) argErr(format string, a ...interface{}) int {
	if format != "" {
		fmt.Fprintf(os.Stderr, format+"\n", a...)
	}
	fmt.Fprintln(os.Stderr, r.cmd.ShortDesc)
	fmt.Fprintln(os.Stderr, r.cmd.UsageLine)
	fmt.Fprintln(os.Stderr, "\nFlags:")
	r.Flags.PrintDefaults()
	return -1
}

var application = &cli.Application{
	Name:  "dmtool",
	Title: "Dungeon Master CLI tool",
	Context: func(ctx context.Context) context.Context {
		return logCfg.Use(ctx)
	},
	Commands: []*subcommands.Command{
		cmdHashQuest,
		cmdVisQuery,
		subcommands.CmdHelp,
	},
}

func main() {
	os.Exit(subcommands.Run(application, os.Args[1:]))
}
