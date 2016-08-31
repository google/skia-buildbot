// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/luci/luci-go/client/archiver"
	"github.com/luci/luci-go/client/internal/common"
	"github.com/luci/luci-go/client/isolate"
	"github.com/luci/luci-go/common/data/text/units"
	"github.com/luci/luci-go/common/isolated"
	"github.com/luci/luci-go/common/isolatedclient"
	"github.com/maruel/subcommands"
)

var cmdBatchArchive = &subcommands.Command{
	UsageLine: "batcharchive <options> file1 file2 ...",
	ShortDesc: "archives multiple isolated trees at once.",
	LongDesc: `Archives multiple isolated trees at once.

Using single command instead of multiple sequential invocations allows to cut
redundant work when isolated trees share common files (e.g. file hashes are
checked only once, their presence on the server is checked only once, and
so on).

Takes a list of paths to *.isolated.gen.json files that describe what trees to
isolate. Format of files is:
{
  "version": 1,
  "dir": <absolute path to a directory all other paths are relative to>,
  "args": [list of command line arguments for single 'archive' command]
}`,
	CommandRun: func() subcommands.CommandRun {
		c := batchArchiveRun{}
		c.commonServerFlags.Init()
		c.Flags.StringVar(&c.dumpJSON, "dump-json", "",
			"Write isolated digests of archived trees to this file as JSON")
		return &c
	},
}

type batchArchiveRun struct {
	commonServerFlags
	dumpJSON string
}

func (c *batchArchiveRun) Parse(a subcommands.Application, args []string) error {
	if err := c.commonServerFlags.Parse(); err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("at least one isolate file required")
	}
	return nil
}

func parseArchiveCMD(args []string, cwd string) (*isolate.ArchiveOptions, error) {
	// Python isolate allows form "--XXXX-variable key value".
	// Golang flag pkg doesn't consider value to be part of --XXXX-variable flag.
	// Therefore, we convert all such "--XXXX-variable key value" to
	// "--XXXX-variable key --XXXX-variable value" form.
	// Note, that key doesn't have "=" in it in either case, but value might.
	// TODO(tandrii): eventually, we want to retire this hack.
	args = convertPyToGoArchiveCMDArgs(args)
	base := subcommands.CommandRunBase{}
	i := isolateFlags{}
	i.Init(&base.Flags)
	if err := base.GetFlags().Parse(args); err != nil {
		return nil, err
	}
	if err := i.Parse(cwd, RequireIsolatedFile); err != nil {
		return nil, err
	}
	if base.GetFlags().NArg() > 0 {
		return nil, fmt.Errorf("no positional arguments expected")
	}
	i.PostProcess(cwd)
	return &i.ArchiveOptions, nil
}

// convertPyToGoArchiveCMDArgs converts kv-args from old python isolate into go variants.
// Essentially converts "--X key value" into "--X key=value".
func convertPyToGoArchiveCMDArgs(args []string) []string {
	kvars := map[string]bool{
		"--path-variable": true, "--config-variable": true, "--extra-variable": true}
	newArgs := []string{}
	for i := 0; i < len(args); {
		newArgs = append(newArgs, args[i])
		kvar := args[i]
		i++
		if !kvars[kvar] {
			continue
		}
		if i >= len(args) {
			// Ignore unexpected behaviour, it'll be caught by flags.Parse() .
			break
		}
		appendArg := args[i]
		i++
		if !strings.Contains(appendArg, "=") && i < len(args) {
			// appendArg is key, and args[i] is value .
			appendArg = fmt.Sprintf("%s=%s", appendArg, args[i])
			i++
		}
		newArgs = append(newArgs, appendArg)
	}
	return newArgs
}

func (c *batchArchiveRun) main(a subcommands.Application, args []string) error {
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
	type tmp struct {
		*archiver.Item
		name string
	}
	items := make(chan *tmp, len(args))
	var wg sync.WaitGroup
	for _, arg := range args {
		wg.Add(1)
		go func(genJsonPath string) {
			defer wg.Done()
			data := &struct {
				Args    []string
				Dir     string
				Version int
			}{}
			if err := common.ReadJSONFile(genJsonPath, data); err != nil {
				arch.Cancel(err)
				return
			}
			if data.Version != isolate.IsolatedGenJSONVersion {
				arch.Cancel(fmt.Errorf("invalid version %d in %s", data.Version, genJsonPath))
				return
			}
			if !common.IsDirectory(data.Dir) {
				arch.Cancel(fmt.Errorf("invalid dir %s in %s", data.Dir, genJsonPath))
				return
			}
			opts, err := parseArchiveCMD(data.Args, data.Dir)
			if err != nil {
				arch.Cancel(fmt.Errorf("invalid archive command in %s: %s", genJsonPath, err))
				return
			}
			name := filepath.Base(opts.Isolated)
			// Strip the extension if there is one.
			if dotIndex := strings.LastIndex(name, "."); dotIndex != -1 {
				name = name[0:dotIndex]
			}
			items <- &tmp{isolate.Archive(arch, opts), name}
		}(arg)
	}
	go func() {
		wg.Wait()
		close(items)
	}()

	data := map[string]isolated.HexDigest{}
	for item := range items {
		item.WaitForHashed()
		if item.Error() == nil {
			data[item.name] = item.Digest()
			fmt.Printf("%s%s  %s\n", prefix, item.Digest(), item.name)
		} else {
			fmt.Fprintf(os.Stderr, "%s%s  %s\n", prefix, item.name, item.Error())
		}
	}
	err = arch.Close()
	duration := time.Since(start)
	// Only write the file once upload is confirmed.
	if err == nil && c.dumpJSON != "" {
		err = common.WriteJSONFile(c.dumpJSON, data)
	}
	if !c.defaultFlags.Quiet {
		stats := arch.Stats()
		fmt.Fprintf(os.Stderr, "Hits    : %5d (%s)\n", stats.TotalHits(), stats.TotalBytesHits())
		fmt.Fprintf(os.Stderr, "Misses  : %5d (%s)\n", stats.TotalMisses(), stats.TotalBytesPushed())
		fmt.Fprintf(os.Stderr, "Duration: %s\n", units.Round(duration, time.Millisecond))
	}
	return err
}

func (c *batchArchiveRun) Run(a subcommands.Application, args []string) int {
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
