package main

import (
	"flag"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/supported_branches/cmd/new-branch/helper"
)

var (
	// Flags.
	branch         = flag.String("branch", "", "Name of the new branch, without refs/heads prefix.")
	deleteBranch   = flag.String("delete", "", "Name of an existing branch to delete, without refs/heads prefix.")
	excludeTrybots = common.NewMultiStringFlag("exclude-trybots", nil, "Regular expressions for trybot names to exclude.")
	owner          = flag.String("owner", "", "Owner of the new branch.")
	repoUrl        = flag.String("repo", common.REPO_SKIA, "URL of the git repository.")
	submit         = flag.Bool("submit", false, "If set, automatically submit the CL to update the CQ and supported branches.")
)

func main() {
	common.Init()

	if *branch == "" {
		sklog.Fatal("--branch is required.")
	}
	if *owner == "" {
		sklog.Fatal("--owner is required.")
	}
	if err := helper.AddSupportedBranch(*repoUrl, *branch, *owner, *deleteBranch, *excludeTrybots, *submit); err != nil {
		sklog.Fatal(err)
	}
}
