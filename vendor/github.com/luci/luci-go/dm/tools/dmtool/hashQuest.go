// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/golang/protobuf/jsonpb"
	dm "github.com/luci/luci-go/dm/api/service/v1"
	"github.com/maruel/subcommands"
)

var cmdHashQuest = &subcommands.Command{
	UsageLine: `hash [options]`,
	ShortDesc: "Produces a DM-compliant QuestID",
	LongDesc: `This command generates a DM QuestID from all the components of a
	DM Quest Description. The description must be supplied via STDIN
	in the form of JSONPB.`,
	CommandRun: func() subcommands.CommandRun {
		r := &hashQuestRun{}
		r.registerOptions()
		return r
	},
}

type hashQuestRun struct {
	cmdRun
}

func (r *hashQuestRun) registerOptions() {
}

func (r *hashQuestRun) Run(a subcommands.Application, args []string) int {
	r.cmd = cmdHashQuest

	if len(args) > 0 {
		return r.argErr("found %d extra arguments", len(args))
	}

	desc := &dm.Quest_Desc{}
	err := jsonpb.Unmarshal(os.Stdin, desc)
	if err != nil {
		return r.argErr("failed to unmarshal dm.Quest.Desc: %s", err)
	}

	if err := desc.Normalize(); err != nil {
		return r.argErr("failed to normalize dm.Quest.Desc: %s", err)
	}

	fmt.Println(desc.QuestID())
	return 0
}
