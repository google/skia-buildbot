package main

import (
	"flag"
	"io"
	"io/ioutil"
	"regexp"

	"go.chromium.org/luci/cq/api/config/v2"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/cq"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	configFile          = flag.String("cfg-file", "", "commit-queue.cfg file to edit.")
	newBranch           = flag.String("new-branch", "", "Short name of the new branch.")
	oldBranch           = flag.String("old-branch", git.DefaultBranch, "Short name of the existing branch whose config to copy.")
	excludeTrybots      = common.NewMultiStringFlag("exclude-trybots", nil, "Regular expressions for trybot names to exclude.")
	includeExperimental = flag.Bool("include-experimental", false, "If true, include experimental trybots.")
	includeTreeCheck    = flag.Bool("include-tree-check", false, "If true, include tree open check.")
)

func main() {
	common.Init()

	if *configFile == "" {
		sklog.Fatal("--cfg-file is required.")
	}
	if *newBranch == "" {
		sklog.Fatal("--new-branch is required.")
	}
	excludeTrybotRegexp := make([]*regexp.Regexp, 0, len(*excludeTrybots))
	for _, excludeTrybot := range *excludeTrybots {
		re, err := regexp.Compile(excludeTrybot)
		if err != nil {
			sklog.Fatalf("Failed to compile regular expression from %q; %s", excludeTrybot, err)
		}
		excludeTrybotRegexp = append(excludeTrybotRegexp, re)
	}

	// Read the config file.
	oldCfgBytes, err := ioutil.ReadFile(*configFile)
	if err != nil {
		sklog.Fatalf("Failed to read %s; %s", *configFile, err)
	}

	// Update the config.
	newCfgBytes, err := cq.WithUpdateCQConfig(oldCfgBytes, func(cfg *config.Config) error {
		return cq.CloneBranch(cfg, *oldBranch, *newBranch, *includeExperimental, *includeTreeCheck, excludeTrybotRegexp)
	})
	if err != nil {
		sklog.Fatal(err)
	}

	// Write the new config.
	if err := util.WithWriteFile(*configFile, func(w io.Writer) error {
		_, err := w.Write(newCfgBytes)
		return err
	}); err != nil {
		sklog.Fatalf("Failed to write config file: %s", err)
	}
}
