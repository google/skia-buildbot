package try

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"

	"github.com/urfave/cli/v2"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/supported_branches/cmd/new-branch/helper"
)

const (
	chromeBranchTmpl             = "chrome/m%d"
	flagDryRun                   = "dry-run"
	gerritProject                = "skia"
	milestoneFile                = "include/core/SkMilestone.h"
	milestoneTmpl                = "#define SK_MILESTONE %s"
	supportedChromeBranches      = 4
	updateMilestoneCommitMsgTmpl = "Update Skia milestone to %d"
)

var (
	milestoneRegex = regexp.MustCompile(fmt.Sprintf(milestoneTmpl, `(\d+)`))

	excludeTrybotsOnReleaseBranches = []string{
		"chromium.*",
		".*Android_Framework.*",
		".*G3_Framework.*",
		".*CanvasKit.*",
		".*PathKit.*",
	}
)

// Command returns a cli.Command instance which represents the "release-branch"
// command.
func Command() *cli.Command {
	return &cli.Command{
		Name:        "release-branch",
		Usage:       "release-branch <commit hash>",
		Description: "Create a new Skia release branch at the given hash.",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  flagDryRun,
				Value: false,
				Usage: "Create the branch CLs but do not submit them or create the branch itself.",
			},
		},
		Action: func(ctx *cli.Context) error {
			args := ctx.Args().Slice()
			if len(args) != 1 {
				return skerr.Fmt("Exactly one positional argument is expected.")
			}
			return releaseBranch(ctx.Context, args[0], ctx.Bool(flagDryRun))
		},
	}
}

// releaseBranch performs the actions necessary to create a new Skia release
// branch.
func releaseBranch(ctx context.Context, commit string, dryRun bool) error {
	// Setup.
	ts, err := auth.NewDefaultTokenSource(true, auth.SCOPE_GERRIT)
	if err != nil {
		return skerr.Wrap(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	repo := gitiles.NewRepo(common.REPO_SKIA, client)
	g, err := gerrit.NewGerrit(gerrit.GerritSkiaURL, client)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Retrieve the current milestone.
	baseCommit, err := repo.ResolveRef(ctx, git.DefaultRef)
	if err != nil {
		return skerr.Wrap(err)
	}
	milestone, milestoneFileContents, err := getCurrentMilestone(ctx, repo, baseCommit)
	if err != nil {
		return skerr.Wrap(err)
	}
	newBranch := fmt.Sprintf(chromeBranchTmpl, milestone)
	oldBranch := fmt.Sprintf(chromeBranchTmpl, milestone-supportedChromeBranches)
	fmt.Println(fmt.Sprintf("Creating branch %s and removing support (eg. CQ) for %s", newBranch, oldBranch))

	fmt.Println(fmt.Sprintf("Creating branch %s...", newBranch))
	if err := createNewBranch(ctx, newBranch, commit, dryRun); err != nil {
		return skerr.Wrap(err)
	}
	fmt.Println("Creating CL to update milestone...")
	if err := updateMilestone(ctx, g, baseCommit, milestone+1, milestoneFileContents, dryRun); err != nil {
		return skerr.Wrap(err)
	}
	fmt.Println("Creating CL to update CQ...")
	if err := updateInfraConfig(ctx, g, oldBranch, newBranch, dryRun); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// getCurrentMilestone retrieves the current milestone value.
func getCurrentMilestone(ctx context.Context, repo gitiles.GitilesRepo, baseCommit string) (int, string, error) {
	contents, err := repo.ReadFileAtRef(ctx, milestoneFile, baseCommit)
	if err != nil {
		return 0, "", skerr.Wrap(err)
	}
	match := milestoneRegex.FindAllStringSubmatch(string(contents), 1)
	if len(match) != 1 {
		return 0, "", skerr.Fmt("Unable to parse milestone number from: %s", string(contents))
	}
	if len(match[0]) != 2 {
		return 0, "", skerr.Fmt("Unable to parse milestone number from: %s", string(contents))
	}
	milestone, err := strconv.Atoi(match[0][1])
	if err != nil {
		return 0, "", skerr.Wrap(err)
	}
	return milestone, string(contents), nil
}

// createNewBranch creates a branch with the given name at the given commit.
func createNewBranch(ctx context.Context, name, commit string, dryRun bool) (rvErr error) {
	// Create a temporary checkout of Skia.
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return skerr.Wrap(err)
	}
	if !dryRun {
		defer func() {
			if err := os.RemoveAll(tmp); err != nil {
				if rvErr == nil {
					rvErr = err
				}
			}
		}()
	}
	co, err := git.NewCheckout(ctx, common.REPO_SKIA, tmp)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Create and push the branch.
	if _, err := co.Git(ctx, "checkout", "-b", name); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := co.Git(ctx, "reset", "--hard", commit); err != nil {
		return skerr.Wrap(err)
	}
	if dryRun {
		fmt.Fprintf(os.Stderr, "Branch %s created in %s; not pushing because --dry-run was specified.\n", name, co.Dir())
	} else {
		if _, err := co.Git(ctx, "push", "--set-upstream", "origin", name); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// updateMilestone creates a CL to update the Skia milestone.
func updateMilestone(ctx context.Context, g gerrit.GerritInterface, baseCommit string, milestone int, oldMilestoneContents string, dryRun bool) error {
	commitMsg := fmt.Sprintf(updateMilestoneCommitMsgTmpl, milestone)
	newContents := milestoneRegex.ReplaceAllString(oldMilestoneContents, fmt.Sprintf(milestoneTmpl, strconv.Itoa(milestone)))
	changes := map[string]string{
		milestoneFile: newContents,
	}
	ci, err := gerrit.CreateCLWithChanges(ctx, g, gerritProject, git.MainBranch, commitMsg, baseCommit, changes, !dryRun)
	if ci != nil {
		fmt.Println(fmt.Sprintf("Uploaded change %s", g.Url(ci.Issue)))
	}
	return skerr.Wrap(err)
}

// updateInfraConfig updates the infra/config branch to edit the supported
// branches and commit queue config to add the new branch and remove the old.
func updateInfraConfig(ctx context.Context, g gerrit.GerritInterface, oldBranch, newBranch string, dryRun bool) error {
	owner, err := g.GetUserEmail(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	return skerr.Wrap(helper.AddSupportedBranch(common.REPO_SKIA, newBranch, owner, oldBranch, excludeTrybotsOnReleaseBranches, !dryRun))
}
