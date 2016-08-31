// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"

	"github.com/maruel/subcommands"
	"golang.org/x/net/context"

	"github.com/luci/luci-go/client/flagpb"
	"github.com/luci/luci-go/common/cli"
	"github.com/luci/luci-go/grpc/prpc"
)

var cmdF2J = &subcommands.Command{
	UsageLine: `f2j [flags] <server> <message type> <message flags>

  server: host ("example.com") or port for localhost (":8080").
  message type: full name of the message type.
  message flags: a message in flagpb format.`,
	ShortDesc: "converts a message from flagpb format to JSON format.",
	LongDesc: `Converts a message from flagpb format to JSON format.
It is convenient for switching from flag format in "rpc call" to json format
once a command line becomes unreadable.

Example:

	$ rpc fmt f2j :8080 helloworld.HelloRequest -name Lucy
	{
		"name: "Lucy"
	}

See also j2f subcommand.
`,
	CommandRun: func() subcommands.CommandRun {
		c := &f2jRun{}
		c.registerBaseFlags()
		return c
	},
}

type f2jRun struct {
	cmdRun
}

func (r *f2jRun) Run(a subcommands.Application, args []string) int {
	if r.cmd == nil {
		r.cmd = cmdF2J
	}

	if len(args) < 2 {
		return r.argErr("")
	}
	host, msgType := args[0], args[1]
	args = args[2:]

	ctx := cli.GetContext(a, r)
	client, err := r.authenticatedClient(ctx, host)
	if err != nil {
		return ecAuthenticatedClientError
	}
	return r.done(flagsToJSON(ctx, client, msgType, args))
}

// flagsToJSON loads server description, resolves msgType, parses
// args to a message according to msgType and prints the message in JSON
// format.
func flagsToJSON(c context.Context, client *prpc.Client, msgType string, args []string) error {
	// Load server description.
	serverDesc, err := loadDescription(c, client)
	if err != nil {
		return err
	}

	// Resolve message type.
	desc, err := serverDesc.resolveMessage(msgType)
	if err != nil {
		return err
	}

	// Parse flags.
	msg, err := flagpb.UnmarshalUntyped(args, desc, flagpb.NewResolver(serverDesc.Description))
	if err != nil {
		return err
	}

	// Print JSON.
	jsonBytes, err := json.MarshalIndent(msg, "", " ")
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", jsonBytes)
	return nil
}
