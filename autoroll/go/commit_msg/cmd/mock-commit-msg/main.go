package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"

	"cloud.google.com/go/datastore"
	"github.com/davecgh/go-spew/spew"
	"github.com/flynn/json5"
	"go.skia.org/infra/autoroll/go/commit_msg"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
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

	// Read the most recent roll from the roller.
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(true, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_GERRIT, datastore.ScopeDatastore, "https://www.googleapis.com/auth/devstorage.read_only")
	if err != nil {
		log.Fatal(err)
	}
	namespace := ds.AUTOROLL_NS
	if cfg.IsInternal {
		namespace = ds.AUTOROLL_INTERNAL_NS
	}
	if err := ds.InitWithOpt(common.PROJECT_ID, namespace, option.WithTokenSource(ts)); err != nil {
		log.Fatal(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	st, err := status.Get(ctx, cfg.RollerName)
	if err != nil {
		log.Fatalf("Failed to retrieve roller status: %s", err)
	}
	if len(st.Recent) > 0 {
		lastRoll := st.Recent[0]
		//var commitMsg string
		if cfg.Gerrit != nil {
			gc, err := cfg.Gerrit.GetConfig()
			if err != nil {
				log.Fatalf("Failed to get Gerrit config: %s", err)
			}
			g, err := gerrit.NewGerritWithConfig(gc, cfg.Gerrit.URL, client)
			if err != nil {
				log.Fatalf("Failed to create Gerrit client: %s", err)
			}
			ci, err := g.GetChange(ctx, fmt.Sprintf("%d", lastRoll.Issue))
			if err != nil {
				log.Fatalf("Failed to get change: %s", err)
			}
			fmt.Println(spew.Sdump(ci))
			//commitMsg = "halp"
		}
	}

	// Create the commit message builder.
	if *serverURL == "" {
		*serverURL = fmt.Sprintf("https://autoroll.skia.org/r/%s", cfg.RollerName)
	}
	b, err := commit_msg.NewBuilder(cfg.CommitMsgConfig, *serverURL, cfg.TransitiveDeps)
	if err != nil {
		log.Fatalf("Failed to create commit message builder: %s", err)
	}

	// Build a fake commit message.
	from, to, revs, reviewers := commit_msg.FakeCommitMsgInputs()
	msg, err := b.Build(from, to, revs, reviewers)
	if err != nil {
		log.Fatalf("Failed to build commit message: %s", err)
	}
	fmt.Println(msg)
}
