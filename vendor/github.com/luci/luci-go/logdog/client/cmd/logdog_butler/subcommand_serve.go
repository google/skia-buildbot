// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/client/butler"

	"github.com/maruel/subcommands"
	"golang.org/x/net/context"
)

var subcommandServe = &subcommands.Command{
	UsageLine: "serve",
	ShortDesc: "Instantiates a stream server.",
	LongDesc:  "Instantiates a stream server, accepting connections and forwarding them to output.",
	CommandRun: func() subcommands.CommandRun {
		cmd := &serveCommandRun{}

		cmd.Flags.Var(&cmd.uri, "streamserver-uri",
			"The stream server URI to bind to (e.g., "+string(exampleStreamServerURI)+").")
		return cmd
	},
}

type serveCommandRun struct {
	subcommands.CommandRunBase

	uri streamServerURI
}

func (cmd *serveCommandRun) Run(app subcommands.Application, args []string) int {
	a := app.(*application)

	if err := cmd.uri.Validate(); err != nil {
		log.Fields{
			"flag":  "-streamserver_uri",
			"value": cmd.uri,
		}.Errorf(a, "Invalid stream server URI.")
		return configErrorReturnCode
	}
	streamServer := createStreamServer(a, cmd.uri)

	if err := streamServer.Listen(); err != nil {
		log.Errorf(log.SetError(a, err), "Failed to connect to stream server.")
		return runtimeErrorReturnCode
	}

	// We think everything will work. Configure our Output instance.
	output, err := a.configOutput()
	if err != nil {
		log.WithError(err).Errorf(a, "Failed to create output instance.")
		return runtimeErrorReturnCode
	}
	defer output.Close()

	err = a.runWithButler(a, output, func(ctx context.Context, b *butler.Butler) error {
		b.AddStreamServer(streamServer)
		return b.Wait()
	})
	if err != nil {
		logAnnotatedErr(a, err, "Failed to serve.")
		return runtimeErrorReturnCode
	}

	return 0
}
