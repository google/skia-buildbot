// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/luci/luci-go/client/archiver"
	"github.com/luci/luci-go/client/internal/common"
	"github.com/luci/luci-go/common/data/text/units"
	"github.com/luci/luci-go/common/isolatedclient"
	"github.com/maruel/subcommands"
)

var cmdArchive = &subcommands.Command{
	UsageLine: "archive <options>...",
	ShortDesc: "creates a .isolated file and uploads the tree to an isolate server.",
	LongDesc:  "All the files listed in the .isolated file are put in the isolate server.",
	CommandRun: func() subcommands.CommandRun {
		c := archiveRun{}
		c.commonFlags.Init()
		c.Flags.Var(&c.dirs, "dirs", "Directory(ies) to archive")
		c.Flags.Var(&c.files, "files", "Individual file(s) to archive")
		c.Flags.Var(&c.blacklist, "blacklist",
			"List of regexp to use as blacklist filter when uploading directories")
		return &c
	},
}

type archiveRun struct {
	commonFlags
	dirs      common.Strings
	files     common.Strings
	blacklist common.Strings
}

func (c *archiveRun) Parse(a subcommands.Application, args []string) error {
	if err := c.commonFlags.Parse(); err != nil {
		return err
	}
	if len(args) != 0 {
		return errors.New("position arguments not expected")
	}
	return nil
}

func (c *archiveRun) main(a subcommands.Application, args []string) error {
	start := time.Now()
	out := os.Stdout
	prefix := "\n"
	if c.defaultFlags.Quiet {
		out = nil
		prefix = ""
	}
	arch := archiver.New(isolatedclient.New(nil, c.createClient(), c.isolatedFlags.ServerURL, c.isolatedFlags.Namespace, nil), out)
	common.CancelOnCtrlC(arch)
	items := make([]*archiver.Item, 0, len(c.files)+len(c.dirs))
	names := make([]string, 0, cap(items))
	for _, file := range c.files {
		items = append(items, arch.PushFile(file, file, 0))
		names = append(names, file)
	}

	for _, d := range c.dirs {
		items = append(items, archiver.PushDirectory(arch, d, "", nil))
		names = append(names, d)
	}

	for i, item := range items {
		item.WaitForHashed()
		if err := item.Error(); err == nil {
			fmt.Printf("%s%s  %s\n", prefix, item.Digest(), names[i])
		} else {
			fmt.Printf("%s%s failed: %s\n", prefix, names[i], err)
		}
	}
	// This waits for all uploads.
	err := arch.Close()
	if !c.defaultFlags.Quiet {
		duration := time.Since(start)
		stats := arch.Stats()
		fmt.Fprintf(os.Stderr, "Hits    : %5d (%s)\n", stats.TotalHits(), stats.TotalBytesHits())
		fmt.Fprintf(os.Stderr, "Misses  : %5d (%s)\n", stats.TotalMisses(), stats.TotalBytesPushed())
		fmt.Fprintf(os.Stderr, "Duration: %s\n", units.Round(duration, time.Millisecond))
	}
	return err
}

func (c *archiveRun) Run(a subcommands.Application, args []string) int {
	if err := c.Parse(a, args); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	cl, err := c.defaultFlags.StartTracing()
	if err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	defer cl.Close()
	if err := c.main(a, args); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}
