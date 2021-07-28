package try

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/cq"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/supported_branches"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	chromeBranchTmpl             = "chrome/m%s"
	flagDryRun                   = "dry-run"
	gerritProject                = "skia"
	milestoneFile                = "include/core/SkMilestone.h"
	milestoneTmpl                = "#define SK_MILESTONE %s"
	supportedChromeBranches      = 4
	updateMilestoneCommitMsgTmpl = "Update Skia milestone to %d"
	jobsJSONFile                 = "infra/bots/jobs.json"
	tasksJSONFile                = "infra/bots/tasks.json"
	cqJSONFile                   = "infra/skcq.json"
)

var (
	chromeBranchMilestoneRegex = regexp.MustCompile(fmt.Sprintf(chromeBranchTmpl, `(\d+)`))
	milestoneRegex             = regexp.MustCompile(fmt.Sprintf(milestoneTmpl, `(\d+)`))

	excludeTrybotsOnReleaseBranches = []*regexp.Regexp{
		regexp.MustCompile(".*CanvasKit.*"),
		regexp.MustCompile(".*PathKit.*"),
	}

	jobsJSONReplaceRegex    = regexp.MustCompile(`(?m)\{\n    "name": "(\S+)",\n    "cq_config": null\n  }`)
	jobsJSONReplaceContents = []byte(`{"name": "$1"}`)
)

// Command returns a cli.Command instance which represents the "release-branch"
// command.
func Command() *cli.Command {
	return &cli.Command{
		Name:        "release-branch",
		Usage:       "release-branch [options] <newly-created branch name>",
		Description: "Perform the necessary updates after creating a new release branch. The new branch must already exist.",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  flagDryRun,
				Value: false,
				Usage: "Create the CLs but do not submit them.",
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
func releaseBranch(ctx context.Context, newBranch string, dryRun bool) error {
	// Derive the new milestone number and the newly-expired branch name from
	// the new branch name.
	m := chromeBranchMilestoneRegex.FindStringSubmatch(newBranch)
	if len(m) != 2 {
		return skerr.Fmt("Invalid branch name %q; expected format: %s", newBranch, chromeBranchMilestoneRegex.String())
	}
	currentMilestone, err := strconv.Atoi(m[1])
	if err != nil {
		return skerr.Wrap(err)
	}
	newMilestone := currentMilestone + 1
	removeBranch := fmt.Sprintf(chromeBranchTmpl, strconv.Itoa(currentMilestone-supportedChromeBranches))

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
	baseCommit, err := repo.ResolveRef(ctx, git.MainBranch)
	if err != nil {
		return skerr.Wrap(err)
	}
	haveMilestone, milestoneFileContents, err := getCurrentMilestone(ctx, repo, baseCommit)
	if err != nil {
		return skerr.Wrap(err)
	}
	fmt.Println(fmt.Sprintf("Adding support for branch %s and removing support for %s", newBranch, removeBranch))

	if haveMilestone == newMilestone {
		fmt.Println(fmt.Sprintf("Milestone is up to date at %d; not updating.", newMilestone))
	} else {
		fmt.Println(fmt.Sprintf("Creating CL to update milestone from %d to %d...", haveMilestone, newMilestone))
		if err := updateMilestone(ctx, g, baseCommit, newMilestone, milestoneFileContents, dryRun); err != nil {
			return skerr.Wrap(err)
		}
	}
	fmt.Println("Creating CL to update supported branches...")
	if err := updateSupportedBranches(ctx, g, repo, removeBranch, newBranch, dryRun); err != nil {
		return skerr.Wrap(err)
	}
	fmt.Println(fmt.Sprintf("Creating CL to filter out unsupported CQ try jobs on %s...", newBranch))
	if err := updateTryjobs(ctx, g, repo, newBranch, dryRun); err != nil {
		return skerr.Wrap(err)
	}
	fmt.Println(fmt.Sprintf("Creating CL to remove CQ on %s", removeBranch))
	if err := removeCQ(ctx, g, repo, removeBranch, dryRun); err != nil {
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

// updateSupportedBranches updates the infra/config branch to edit the supported
// branches and commit queue config to add the new branch and remove the old.
func updateSupportedBranches(ctx context.Context, g gerrit.GerritInterface, repo *gitiles.Repo, removeBranch string, newBranch string, dryRun bool) error {
	owner, err := g.GetUserEmail(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	newRef := git.FullyQualifiedBranchName(newBranch)

	// Setup.
	baseCommitInfo, err := repo.Details(ctx, cq.CQ_CFG_REF)
	if err != nil {
		return skerr.Wrap(err)
	}
	baseCommit := baseCommitInfo.Hash

	// Download and modify the supported-branches.json file.
	branchesContents, err := repo.ReadFileAtRef(ctx, supported_branches.SUPPORTED_BRANCHES_FILE, baseCommit)
	if err != nil {
		return skerr.Wrap(err)
	}
	sbc, err := supported_branches.DecodeConfig(bytes.NewReader(branchesContents))
	if err != nil {
		return skerr.Wrap(err)
	}
	deleteRef := git.FullyQualifiedBranchName(removeBranch)
	foundNewRef := false
	deletedRef := false
	newBranches := make([]*supported_branches.SupportedBranch, 0, len(sbc.Branches)+1)
	for _, sb := range sbc.Branches {
		if sb.Ref != deleteRef {
			newBranches = append(newBranches, sb)
		} else {
			deletedRef = true
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
			Owner: owner,
		})
	}
	if !deletedRef {
		_, _ = fmt.Fprintf(os.Stderr, "%s not found in %s; not removing.\n", deleteRef, supported_branches.SUPPORTED_BRANCHES_FILE)
	}
	if !deletedRef && foundNewRef {
		// We didn't change anything, so don't upload a CL.
		return nil
	}
	sbc.Branches = newBranches
	buf := bytes.Buffer{}
	if err := supported_branches.EncodeConfig(&buf, sbc); err != nil {
		return skerr.Wrap(err)
	}

	// Create the Gerrit CL.
	commitMsg := fmt.Sprintf("Add supported branch %s, remove %s", newBranch, removeBranch)
	repoSplit := strings.Split(repo.URL(), "/")
	project := strings.TrimSuffix(repoSplit[len(repoSplit)-1], ".git")
	changes := map[string]string{
		supported_branches.SUPPORTED_BRANCHES_FILE: buf.String(),
	}
	ci, err := gerrit.CreateCLWithChanges(ctx, g, project, cq.CQ_CFG_REF, commitMsg, baseCommit, changes, !dryRun)
	if ci != nil {
		fmt.Println(fmt.Sprintf("Uploaded change %s", g.Url(ci.Issue)))
	}
	return skerr.Wrap(err)
}

func updateTryjobs(ctx context.Context, g gerrit.GerritInterface, repo *gitiles.Repo, newBranch string, dryRun bool) error {
	// Setup.
	newRef := git.FullyQualifiedBranchName(newBranch)
	baseCommitInfo, err := repo.Details(ctx, newRef)
	if err != nil {
		return skerr.Wrap(err)
	}
	baseCommit := baseCommitInfo.Hash
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return skerr.Wrap(err)
	}
	defer util.RemoveAll(tmp)
	co, err := git.NewCheckout(ctx, repo.URL(), tmp)
	if err != nil {
		return skerr.Wrap(err)
	}
	if err := co.CleanupBranch(ctx, newBranch); err != nil {
		return skerr.Wrap(err)
	}

	// Download and modify the jobs.json file.
	oldJobsContents, err := repo.ReadFileAtRef(ctx, jobsJSONFile, baseCommit)
	if err != nil {
		return skerr.Wrap(err)
	}
	var jobs []struct {
		Name     string                      `json:"name"`
		CqConfig *specs.CommitQueueJobConfig `json:"cq_config"`
	}
	if err := json.Unmarshal(oldJobsContents, &jobs); err != nil {
		return skerr.Wrapf(err, "failed to decode jobs.json")
	}
	for _, job := range jobs {
		for _, re := range excludeTrybotsOnReleaseBranches {
			if re.MatchString(job.Name) {
				job.CqConfig = nil
				break
			}
		}
	}
	newJobsContents, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return skerr.Wrapf(err, "failed to encode jobs.json")
	}
	// Replace instances of `"cq_config": null`; these are cluttery and
	// unnecessary, but we can't use omitempty because that causes the Marshaler
	// to also omit `"cq_config": {}` which indicates that a job *should* be on
	// the CQ. Also attempt to match the whitespace of the original files, to
	// help prevent conflicts during cherry-picks.
	newJobsContents = jobsJSONReplaceRegex.ReplaceAll(newJobsContents, jobsJSONReplaceContents)
	if err := ioutil.WriteFile(filepath.Join(co.Dir(), jobsJSONFile), newJobsContents, os.ModePerm); err != nil {
		return skerr.Wrapf(err, "failed to write %s", jobsJSONFile)
	}

	// Regenerate tasks.json.
	if _, err := exec.RunCwd(ctx, co.Dir(), "go", "run", "./infra/bots/gen_tasks.go"); err != nil {
		return skerr.Wrapf(err, "failed to regenerate tasks.json")
	}

	// Create the Gerrit CL.
	commitMsg := fmt.Sprintf("Filter unsupported CQ try jobs on %s", newBranch)
	repoSplit := strings.Split(repo.URL(), "/")
	project := strings.TrimSuffix(repoSplit[len(repoSplit)-1], ".git")
	ci, err := gerrit.CreateCLFromLocalDiffs(ctx, g, project, newBranch, commitMsg, !dryRun, co)
	if err != nil {
		return skerr.Wrap(err)
	}
	fmt.Println(fmt.Sprintf("Uploaded change %s", g.Url(ci.Issue)))
	return nil
}

func removeCQ(ctx context.Context, g gerrit.GerritInterface, repo *gitiles.Repo, oldBranch string, dryRun bool) error {
	// Setup.
	oldRef := git.FullyQualifiedBranchName(oldBranch)
	baseCommitInfo, err := repo.Details(ctx, oldRef)
	if err != nil {
		return skerr.Wrap(err)
	}
	baseCommit := baseCommitInfo.Hash

	// Create the Gerrit CL.
	if _, err := repo.ReadFileAtRef(ctx, cqJSONFile, oldRef); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Attempt to read %s on %s got error: %s; not removing CQ.\n", oldBranch, cqJSONFile, err)
		return nil
	}
	commitMsg := fmt.Sprintf("Remove CQ for unsupported branch %s", oldBranch)
	repoSplit := strings.Split(repo.URL(), "/")
	project := strings.TrimSuffix(repoSplit[len(repoSplit)-1], ".git")
	changes := map[string]string{
		cqJSONFile: "",
	}
	ci, err := gerrit.CreateCLWithChanges(ctx, g, project, oldRef, commitMsg, baseCommit, changes, !dryRun)
	if ci != nil {
		fmt.Println(fmt.Sprintf("Uploaded change %s", g.Url(ci.Issue)))
	}
	return skerr.Wrap(err)
}
