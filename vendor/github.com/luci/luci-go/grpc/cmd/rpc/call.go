// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/maruel/subcommands"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/luci/luci-go/client/flagpb"
	"github.com/luci/luci-go/common/cli"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/grpc/prpc"
)

var cmdCall = &subcommands.Command{
	UsageLine: `call [flags] <server> <service>.<method> [input message flags]

  server: host ("example.com") or port for localhost (":8080").
  service: full name of a service, e.g. "pkg.service"
  method: name of the method.
  input message flags: the input message in flagpb format.
    Ignored if format is json/binary/text, in which case input message is read
    from stdin.
    See also fmt subcommand.`,
	ShortDesc: "calls a service method.",
	LongDesc: `Calls a service method.
Unless format is "flag" (default), the input message is read from stdin`,
	CommandRun: func() subcommands.CommandRun {
		c := &callRun{
			format: formatFlagPB,
		}
		c.registerBaseFlags()
		c.Flags.Var(&c.format, "format", fmt.Sprintf(
			`Message format. Valid values: %s. `+
				`If format is "flag" (default), the output format is text; otherwise output format is same.`,
			formatFlagMap.Choices()))
		return c
	},
}

// callRun implements "call" subcommand.
type callRun struct {
	cmdRun
	format  formatFlag
	message string
}

func (r *callRun) Run(a subcommands.Application, args []string) int {
	if r.cmd == nil {
		r.cmd = cmdCall
	}

	if len(args) < 2 {
		return r.argErr("")
	}
	host, target := args[0], args[1]
	args = args[2:]

	req := request{
		format:       r.format,
		message:      os.Stdin,
		messageFlags: args,
	}

	var err error
	req.service, req.method, err = splitServiceAndMethod(target)
	if err != nil {
		return r.argErr("%s", err)
	}

	ctx := cli.GetContext(a, r)
	client, err := r.authenticatedClient(ctx, host)
	if err != nil {
		return ecAuthenticatedClientError
	}
	return r.done(call(ctx, client, &req, os.Stdout))
}

func splitServiceAndMethod(fullName string) (service string, method string, err error) {
	lastDot := strings.LastIndex(fullName, ".")
	if lastDot < 0 {
		return "", "", fmt.Errorf("invalid full method name %q. It must contain a '.'", fullName)
	}
	service = fullName[:lastDot]
	method = fullName[lastDot+1:]
	return
}

// request is an RPC request.
type request struct {
	service      string
	method       string
	message      io.Reader
	messageFlags []string
	format       formatFlag
}

// call makes an RPC and writes response to out.
func call(c context.Context, client *prpc.Client, req *request, out io.Writer) error {
	var inf, outf prpc.Format
	var message []byte
	switch req.format {

	case formatFlagPB:
		serverDesc, err := loadDescription(c, client)
		if err != nil {
			logging.WithError(err).Errorf(c, "Failed to load description.")
			return err
		}

		desc, err := serverDesc.resolveInputMessage(req.service, req.method)
		if err != nil {
			return err
		}

		resolver := flagpb.NewResolver(serverDesc.Description)
		flagMsg, err := flagpb.UnmarshalUntyped(req.messageFlags, desc, resolver)
		if err != nil {
			return err
		}

		message, err = json.Marshal(flagMsg)
		if err != nil {
			return err
		}
		inf = prpc.FormatJSONPB
		outf = prpc.FormatText

	default:
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(req.message); err != nil {
			return err
		}
		message = buf.Bytes()
		inf = req.format.Format()
		outf = inf
	}

	// Send the request.
	var hmd, tmd metadata.MD
	res, err := client.CallRaw(c, req.service, req.method, message, inf, outf, prpc.Header(&hmd), prpc.Trailer(&tmd))
	if err != nil {
		return &exitCode{err, int(grpc.Code(err))}
	}

	// Read response.
	if _, err := out.Write(res); err != nil {
		return fmt.Errorf("failed to write response: %s", err)
	}
	return err
}
