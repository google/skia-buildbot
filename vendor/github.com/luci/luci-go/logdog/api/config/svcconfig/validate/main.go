// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package main implements the LogDog Coordinator validation binary. This simply
// loads configuration from a text protobuf and verifies that it works.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/maruel/subcommands"

	"github.com/golang/protobuf/proto"
	"github.com/luci/go-render/render"
	"github.com/luci/luci-go/common/cli"
	"github.com/luci/luci-go/logdog/api/config/svcconfig"
)

var (
	subcommandValidateServices = subcommands.Command{
		UsageLine: "services",
		ShortDesc: fmt.Sprintf("Validate %q services config file.", svcconfig.ServiceConfigFilename),
		CommandRun: func() subcommands.CommandRun {
			return &validateCommandRun{msg: &svcconfig.Config{}}
		},
	}

	subcommandValidateProject = subcommands.Command{
		UsageLine: "project",
		ShortDesc: "Validate project config file.",
		CommandRun: func() subcommands.CommandRun {
			return &validateCommandRun{msg: &svcconfig.ProjectConfig{}}
		},
	}
)

type validateCommandRun struct {
	subcommands.CommandRunBase

	msg proto.Message
}

func (cmd *validateCommandRun) Run(_ subcommands.Application, args []string) int {
	if len(args) != 1 {
		log.Fatalln("Must specify exactly one argument: the path to the config.")
	}

	d, err := ioutil.ReadFile(args[0])
	if err != nil {
		log.Fatalln("Failed to read input file: %v", err)
	}

	if err := proto.UnmarshalText(string(d), cmd.msg); err != nil {
		log.Fatalf("Failed to unmarshal input file: %v", err)
	}
	fmt.Println("Successfully unmarshalled configuration:\n", render.Render(cmd.msg))
	return 0
}

func main() {
	app := cli.Application{
		Name:  "Configuration Validator",
		Title: "LogDog Configuration validator",

		Commands: []*subcommands.Command{
			subcommands.CmdHelp,
			&subcommandValidateServices,
			&subcommandValidateProject,
		},
	}

	flag.Parse()
	os.Exit(subcommands.Run(&app, flag.Args()))
}
