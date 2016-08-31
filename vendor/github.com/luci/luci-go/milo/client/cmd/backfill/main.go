// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/luci/gae/impl/prod"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/gologger"
	"github.com/luci/luci-go/milo/appengine/backend/git"
	"github.com/luci/luci-go/milo/appengine/model"
	"github.com/maruel/subcommands"
	"golang.org/x/net/context"
	// Imported so you can upload this as a "module" to app engine, and access
	// the remote API.
	_ "google.golang.org/appengine/remote_api"
)

var application = &subcommands.DefaultApplication{
	Name:  "backfill",
	Title: "Backfill Build and Revision data into the milo backend from various data sources.",
	Commands: []*subcommands.Command{
		gitCmd,
		subcommands.CmdHelp,
	},
}

func main() {
	os.Exit(subcommands.Run(application, nil))
}

// commandRunBase is the base of all backfill subcommands. It defines the common
// flags dryRun and remoteURL
type commandRunBase struct {
	subcommands.CommandRunBase
	dryRun    bool
	remoteURL string
}

func (c *commandRunBase) Init() {
	c.Flags.StringVar(&c.remoteURL, "remote-url", "luci-milo.appspot.com", "the URL of the server to connect to via the remote API. Do NOT include the protocol (\"https://\")")
	c.Flags.BoolVar(&c.dryRun, "dryrun", true, "if this run is a dryrun, and should make no modifications to the datastore.")
}

////////////////////////////////////////////////////////////////////////////////
// Git

var gitCmd = &subcommands.Command{
	UsageLine: "git",

	ShortDesc: "runs the local git backfiller",

	LongDesc: "Runs the local git backfiller. Reads history from a local git repo and uploads revision info to cloud datastore at remoteURL.",

	CommandRun: func() subcommands.CommandRun {
		c := &cmdGitRun{}
		c.Init()
		c.Flags.StringVar(&c.gitLog, "git-log", "", "file containing output of git log --topo-order --reverse -z --format=format:'%H,%P,%ae,%ct,%b'")
		c.Flags.StringVar(&c.repoURL, "repo-url", "", "the repo URL in the Revision entity group")
		return c
	},
}

type cmdGitRun struct {
	commandRunBase
	gitLog  string
	repoURL string
}

func (c *cmdGitRun) Run(a subcommands.Application, args []string) int {
	cfg := gologger.LoggerConfig{
		Format: "%{message}",
		Out:    os.Stdout,
	}
	ctx := cfg.Use(context.Background())

	if c.gitLog == "" || c.remoteURL == "" || c.repoURL == "" {
		log.Errorf(ctx, "Flags -git-log, -remote-url, and -repo-url must all be set.")
		return 1
	}

	// We need a gae context even in --dry-run mode to get the repository datastore key.
	if err := prod.UseRemote(&ctx, c.remoteURL, nil); err != nil {
		log.Errorf(ctx, "%s (remote URL: %s)", err, c.remoteURL)
		return 1
	}

	contents, err := ioutil.ReadFile(c.gitLog)
	if err != nil {
		log.Errorf(ctx, "%s", err)
		return 1
	}

	revisions, err := git.GetRevisions(string(contents))
	if err != nil {
		log.Errorf(ctx, "%s", err)
		return 1
	}

	for _, revision := range revisions {
		revision.Repository = &model.GetRepository(ctx, c.repoURL).Key
	}

	if c.dryRun {
		log.Infof(ctx, "Running in dry run mode, writing to stdout.")
		for _, r := range revisions {
			fmt.Printf("%+v\n", *r)
		}
		return 0
	}

	log.Infof(ctx, "Saving %d revisions.", len(revisions))
	if err := git.SaveRevisions(ctx, revisions); err != nil {
		log.Errorf(ctx, "%s", err)
		return 1
	}

	return 0
}
