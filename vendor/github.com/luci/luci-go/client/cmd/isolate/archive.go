// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/luci/luci-go/client/archiver"
	"github.com/luci/luci-go/client/internal/common"
	"github.com/luci/luci-go/client/isolate"
	"github.com/luci/luci-go/common/data/text/units"
	"github.com/luci/luci-go/common/isolatedclient"
	"github.com/maruel/subcommands"
)

var cmdArchive = &subcommands.Command{
	UsageLine: "archive <options>",
	ShortDesc: "creates a .isolated file and uploads the tree to an isolate server.",
	LongDesc:  "All the files listed in the .isolated file are put in the isolate server cache",
	CommandRun: func() subcommands.CommandRun {
		c := archiveRun{}
		c.commonServerFlags.Init()
		c.isolateFlags.Init(&c.Flags)
		return &c
	},
}

type archiveRun struct {
	commonServerFlags
	isolateFlags
}

func (c *archiveRun) Parse(a subcommands.Application, args []string) error {
	if err := c.commonServerFlags.Parse(); err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := c.isolateFlags.Parse(cwd, RequireIsolatedFile); err != nil {
		return err
	}
	if len(args) != 0 {
		return errors.New("position arguments not expected")
	}
	return nil
}

func (c *archiveRun) main(a subcommands.Application, args []string) error {
	out := os.Stdout
	prefix := "\n"
	if c.defaultFlags.Quiet {
		out = nil
		prefix = ""
	}
	start := time.Now()
	client, err := c.createAuthClient()
	if err != nil {
		return err
	}
	arch := archiver.New(isolatedclient.New(nil, client, c.isolatedFlags.ServerURL, c.isolatedFlags.Namespace, nil), out)
	common.CancelOnCtrlC(arch)
	item := isolate.Archive(arch, &c.ArchiveOptions)
	item.WaitForHashed()
	if err = item.Error(); err != nil {
		fmt.Printf("%s%s  %s\n", prefix, filepath.Base(c.Isolate), err)
	} else {
		fmt.Printf("%s%s  %s\n", prefix, item.Digest(), filepath.Base(c.Isolate))
	}
	if err2 := arch.Close(); err == nil {
		err = err2
	}
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
