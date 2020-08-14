package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	cq_config "go.chromium.org/luci/cq/api/config/v2"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/cq"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/supported_branches"
	"go.skia.org/infra/go/util"
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
	newRef := fmt.Sprintf("refs/heads/%s", *branch)
	if *owner == "" {
		sklog.Fatal("--owner is required.")
	}
	excludeTrybotRegexp := make([]*regexp.Regexp, 0, len(*excludeTrybots))
	for _, excludeTrybot := range *excludeTrybots {
		re, err := regexp.Compile(excludeTrybot)
		if err != nil {
			sklog.Fatalf("Failed to compile regular expression from %q; %s", excludeTrybot, err)
		}
		excludeTrybotRegexp = append(excludeTrybotRegexp, re)
	}

	// Setup.
	wd, err := ioutil.TempDir("", "new-branch")
	if err != nil {
		sklog.Fatal(err)
	}
	defer util.RemoveAll(wd)

	ts, err := auth.NewDefaultTokenSource(true, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatal(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	gUrl := strings.Split(*repoUrl, ".googlesource.com")[0] + "-review.googlesource.com"
	g, err := gerrit.NewGerrit(gUrl, client)
	if err != nil {
		sklog.Fatal(err)
	}
	repo := gitiles.NewRepo(*repoUrl, client)
	ctx := context.Background()
	baseCommitInfo, err := repo.Details(ctx, cq.CQ_CFG_REF)
	if err != nil {
		sklog.Fatal(err)
	}
	baseCommit := baseCommitInfo.Hash

	// Download the CQ config file and modify it.
	cfgContents, err := repo.ReadFileAtRef(ctx, cq.CQ_CFG_FILE, baseCommit)
	if err != nil {
		sklog.Fatal(err)
	}
	newCfgBytes, err := cq.WithUpdateCQConfig(cfgContents, func(cfg *cq_config.Config) error {
		cg, _, _, err := cq.MatchConfigGroup(cfg, newRef)
		if err != nil {
			return err
		}
		if cg != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Already have %s in %s; not adding a duplicate.\n", newRef, cq.CQ_CFG_FILE)
		} else {
			if err := cq.CloneBranch(cfg, git.DefaultBranch, *branch, false, false, excludeTrybotRegexp); err != nil {
				return err
			}
		}
		if *deleteBranch != "" {
			if err := cq.DeleteBranch(cfg, *deleteBranch); err != nil {
				return err
			}
		}
		return nil
	})

	// Download and modify the supported-branches.json file.
	branchesContents, err := repo.ReadFileAtRef(ctx, supported_branches.SUPPORTED_BRANCHES_FILE, baseCommit)
	if err != nil {
		sklog.Fatal(err)
	}
	sbc, err := supported_branches.DecodeConfig(bytes.NewReader(branchesContents))
	if err != nil {
		sklog.Fatal(err)
	}
	deleteRef := ""
	if *deleteBranch != "" {
		deleteRef = fmt.Sprintf("refs/heads/%s", *deleteBranch)
	}
	foundNewRef := false
	newBranches := make([]*supported_branches.SupportedBranch, 0, len(sbc.Branches)+1)
	for _, sb := range sbc.Branches {
		if deleteRef == "" || deleteRef != sb.Ref {
			newBranches = append(newBranches, sb)
		}
		if sb.Ref == newRef {
			foundNewRef = true
		}
	}
	if foundNewRef {
		_, _ = fmt.Fprintf(os.Stderr, "Already have %s in %s; not adding a duplicate.\n", newRef, supported_branches.SUPPORTED_BRANCHES_FILE)
	} else {
		newBranches = append(newBranches, &supported_branches.SupportedBranch{
			Ref:   newRef,
			Owner: *owner,
		})
	}
	sbc.Branches = newBranches
	buf := bytes.Buffer{}
	if err := supported_branches.EncodeConfig(&buf, sbc); err != nil {
		sklog.Fatal(err)
	}

	// Create the Gerrit CL.
	commitMsg := fmt.Sprintf("Add supported branch %s", *branch)
	if *deleteBranch != "" {
		commitMsg += fmt.Sprintf(", remove %s", *deleteBranch)
	}
	repoSplit := strings.Split(*repoUrl, "/")
	project := strings.TrimSuffix(repoSplit[len(repoSplit)-1], ".git")
	ci, err := gerrit.CreateAndEditChange(ctx, g, project, cq.CQ_CFG_REF, commitMsg, baseCommit, func(ctx context.Context, g gerrit.GerritInterface, ci *gerrit.ChangeInfo) error {
		if err := g.EditFile(ctx, ci, cq.CQ_CFG_FILE, string(newCfgBytes)); err != nil {
			return err
		}
		if err := g.EditFile(ctx, ci, supported_branches.SUPPORTED_BRANCHES_FILE, string(buf.Bytes())); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		sklog.Fatal(err)
	}
	fmt.Println(fmt.Sprintf("Uploaded change https://skia-review.googlesource.com/%d", ci.Issue))
	if *submit {
		if ci.WorkInProgress {
			if err := g.SetReadyForReview(ctx, ci); err != nil {
				sklog.Fatalf("Failed to set ready for review: %s", err)
			}
		}
		if err := g.SetReview(ctx, ci, "", gerrit.CONFIG_CHROMIUM.SelfApproveLabels, nil); err != nil {
			sklog.Fatalf("Failed to set Code-Review+1: %s", err)
		}
		if err := g.Submit(ctx, ci); err != nil {
			sklog.Fatalf("Failed to submit CL: %s", err)
		}
	}
}
