// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/maruel/subcommands"
	"golang.org/x/net/context"
	"google.golang.org/genproto/protobuf"

	"github.com/luci/luci-go/common/cli"
	"github.com/luci/luci-go/common/proto/google/descutil"
	"github.com/luci/luci-go/grpc/prpc"
)

var cmdShow = &subcommands.Command{
	UsageLine: `show [flags] <server> [name]

  server: host ("example.com") or port for localhost (":8080").
  name: a full name of service, method or type. If not specified prints names
  	of all available services.`,
	ShortDesc: "lists services and prints a definition of a referenced object.",
	LongDesc:  "Lists services and prints a definition of a referenced object.",
	CommandRun: func() subcommands.CommandRun {
		c := &showRun{}
		c.registerBaseFlags()
		return c
	},
}

type showRun struct {
	cmdRun
}

func (r *showRun) Run(a subcommands.Application, args []string) int {
	if r.cmd == nil {
		r.cmd = cmdShow
	}

	var host, name string
	switch len(args) {
	case 2:
		name = args[1]
		fallthrough
	case 1:
		host = args[0]

	default:
		return r.argErr("")
	}

	ctx := cli.GetContext(a, r)
	client, err := r.authenticatedClient(ctx, host)
	if err != nil {
		return ecAuthenticatedClientError
	}
	return r.done(show(ctx, client, name))
}

// show prints a definition of an object referenced by name in proto3 style.
func show(c context.Context, client *prpc.Client, name string) error {
	desc, err := loadDescription(c, client)
	if err != nil {
		return fmt.Errorf("could not load server description: %s", err)
	}

	if name == "" {
		for _, s := range desc.Services {
			fmt.Println(s)
		}
		return nil
	}

	file, obj, path := descutil.Resolve(desc.Description, name)
	if obj == nil {
		return fmt.Errorf("name %q could not resolved", name)
	}

	print := newPrinter(os.Stdout)
	print.File = file

	switch obj := obj.(type) {

	case *descriptor.ServiceDescriptorProto:
		print.Service(obj, path[1], -1)

	case *descriptor.MethodDescriptorProto:
		serviceIndex, methodIndex := path[1], path[3]
		print.Service(file.Service[serviceIndex], serviceIndex, methodIndex)

		printMsg := func(name string) {
			name = strings.TrimPrefix(name, ".")
			file, msg, path := descutil.Resolve(desc.Description, name)
			if msg == nil {
				print.Printf("// Message %q is not found\n", name)
				return
			}
			print.File = file
			print.Message(msg.(*descriptor.DescriptorProto), path)
		}

		print.Printf("\n")
		printMsg(obj.GetInputType())
		print.Printf("\n")
		printMsg(obj.GetOutputType())

	case *descriptor.DescriptorProto:
		print.Message(obj, path)

	case *descriptor.FieldDescriptorProto:
		print.Field(obj, path)

	case *descriptor.EnumDescriptorProto:
		print.Enum(obj, path)

	case *descriptor.EnumValueDescriptorProto:
		print.EnumValue(obj, path)

	default:
		return fmt.Errorf("object of type %T is not supported", obj)
	}

	return print.Err
}
