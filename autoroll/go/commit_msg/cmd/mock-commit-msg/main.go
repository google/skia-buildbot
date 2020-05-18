package main

import (
	"flag"
	"fmt"
	"io"
	"log"

	"github.com/flynn/json5"
	"go.skia.org/infra/autoroll/go/commit_msg"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/util"
)

var (
	configFile = flag.String("config", "", "Config file to parse. Required.")
	serverURL  = flag.String("server_url", "", "Server URL. Optional.")
)

func main() {
	common.Init()

	// Validation.
	if *configFile == "" {
		log.Fatal("--config is required.")
	}

	// Read the roller config file.
	var cfg roller.AutoRollerConfig
	if err := util.WithReadFile(*configFile, func(r io.Reader) error {
		return json5.NewDecoder(r).Decode(&cfg)
	}); err != nil {
		log.Fatalf("Failed to read %s: %s", *configFile, err)
	}

	// Fake the serverURL based on the roller name.
	if *serverURL == "" {
		*serverURL = fmt.Sprintf("https://autoroll.skia.org/r/%s", cfg.RollerName)
	}

	// Obtain fake data to use for the commit message.
	from, to, revs, _ := commit_msg.FakeCommitMsgInputs()
	reviewers, err := roller.GetSheriff(cfg.RollerName, cfg.Sheriff, cfg.SheriffBackup)
	if err != nil {
		log.Fatalf("Failed to retrieve sheriff: %s", err)
	}

	// Create the commit message builder.
	b, err := commit_msg.NewBuilder(cfg.CommitMsgConfig, cfg.ChildDisplayName, *serverURL, cfg.TransitiveDeps)
	if err != nil {
		log.Fatalf("Failed to create commit message builder: %s", err)
	}

	// Build the commit message.
	genCommitMsg, err := b.Build(from, to, revs, reviewers)
	if err != nil {
		log.Fatalf("Failed to build commit message: %s", err)
	}
	fmt.Println(genCommitMsg)
}
