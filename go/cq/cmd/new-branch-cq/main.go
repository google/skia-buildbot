package main

import (
	"context"
	"flag"
	"path/filepath"
	"regexp"

	"github.com/bazelbuild/buildtools/build"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/cq"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
)

var (
	configFile          = flag.String("cfg-file", "", "commit-queue.cfg file to edit.")
	newBranch           = flag.String("new-branch", "", "Short name of the new branch.")
	oldBranch           = flag.String("old-branch", git.MasterBranch, "Short name of the existing branch whose config to copy.")
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

	ctx := context.Background()
	generatedDir := filepath.Join(filepath.Dir(*configFile), "generated")
	if err := cq.WithUpdateCQConfig(ctx, *configFile, generatedDir, func(f *build.File) error {
		return cq.CloneBranch(f, *oldBranch, *newBranch, *includeExperimental, *includeTreeCheck, excludeTrybotRegexp)
	}); err != nil {
		sklog.Fatal(err)
	}
}
