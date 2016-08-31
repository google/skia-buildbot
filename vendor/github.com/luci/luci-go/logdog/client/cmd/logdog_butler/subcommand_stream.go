// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"os"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/flag/nestedflagset"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/client/butler"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"
	"github.com/luci/luci-go/logdog/common/types"

	"github.com/maruel/subcommands"
	"golang.org/x/net/context"
)

var subcommandStream = &subcommands.Command{
	UsageLine: "stream",
	ShortDesc: "Serves a single input stream.",
	LongDesc:  "Configures and transmits a single stream from the command-line.",
	CommandRun: func() subcommands.CommandRun {
		cmd := &streamCommandRun{}

		cmd.Flags.StringVar(&cmd.path, "source", "-",
			"The path of the file to stream. If omitted, STDIN will be used")

		streamFlag := &nestedflagset.FlagSet{}
		cmd.stream.addFlags(&streamFlag.F)
		cmd.Flags.Var(streamFlag, "stream", "Output stream parameters.")
		return cmd
	},
}

type streamCommandRun struct {
	subcommands.CommandRunBase

	path   string       // The stream input path.
	stream streamConfig // Stream configuration parameters.
}

// subcommands.Run
func (cmd *streamCommandRun) Run(app subcommands.Application, args []string) int {
	a := app.(*application)

	var (
		streamFile        *os.File
		defaultStreamName streamproto.StreamNameFlag
	)
	if cmd.path == "-" {
		defaultStreamName = "stdin"
		streamFile = os.Stdin
	} else {
		streamName, err := types.MakeStreamName("file:", cmd.path)
		if err != nil {
			log.Fields{
				log.ErrorKey: err,
			}.Errorf(a, "Failed to generate stream name.")
			return runtimeErrorReturnCode
		}
		defaultStreamName = streamproto.StreamNameFlag(streamName)

		file, err := os.Open(cmd.path)
		if err != nil {
			log.Fields{
				log.ErrorKey: err,
				"path":       cmd.path,
			}.Errorf(a, "Failed to open input stream file.")
			return runtimeErrorReturnCode
		}
		streamFile = file
	}
	if cmd.stream.Name == "" {
		cmd.stream.Name = defaultStreamName
	}

	// We think everything should work. Configure our Output instance.
	output, err := a.configOutput()
	if err != nil {
		log.WithError(err).Errorf(a, "Failed to create output instance.")
		return runtimeErrorReturnCode
	}
	defer output.Close()

	// Instantiate our Processor.
	err = a.runWithButler(a, output, func(ctx context.Context, b *butler.Butler) error {
		if err := b.AddStream(streamFile, cmd.stream.properties()); err != nil {
			return errors.Annotate(err).Reason("failed to add stream").Err()
		}

		b.Activate()
		return b.Wait()
	})
	if err != nil {
		logAnnotatedErr(a, err, "Failed to stream file.")
		return runtimeErrorReturnCode
	}

	return 0
}
