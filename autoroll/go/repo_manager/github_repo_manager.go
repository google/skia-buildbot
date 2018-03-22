package repo_manager

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"

	//"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	//"go.skia.org/infra/go/util"
)

const (
	//SERVICE_ACCOUNT      = "31977622648@project.gserviceaccount.com"
	//UPSTREAM_REMOTE_NAME = "remote"
	//REPO_BRANCH_NAME     = "merge"

	GITHUB_UPSTREAM_REMOTE_NAME = "remote"
	GITHUB_ROLL_BRANCH_NAME     = "roll_branch"
)

var (
	// Use this function to instantiate a NewGithubRepoManager. This is able to be
	// overridden for testing.
	NewGithubRepoManager func(context.Context, string, string, string, string, string, string, *gerrit.Gerrit, NextRollStrategy, []string, bool, string, string) (RepoManager, error) = newGithubRepoManager

	//IGNORE_MERGE_CONFLICT_FILES = []string{android_skia_checkout.SkUserConfigRelPath}

	//FILES_GENERATED_BY_GN_TO_GP = []string{android_skia_checkout.SkUserConfigRelPath, android_skia_checkout.AndroidBpRelPath}

	//AUTHOR_EMAIL_RE = regexp.MustCompile(".* \\((.*)\\)")
)

// androidRepoManager is a struct used by the autoroller for managing checkouts.
type githubRepoManager struct {
	*depsRepoManager
}

// newGithubRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func newGithubRepoManager(ctx context.Context, workdir, parentRepo, parentBranch, childPath, childBranch, depot_tools string, g *gerrit.Gerrit, strategy NextRollStrategy, preUploadStepNames []string, includeLog bool, gclientSpec, serverURL string) (RepoManager, error) {
	gclient := path.Join(depot_tools, GCLIENT)
	rollDep := path.Join(depot_tools, ROLL_DEP)

	wd := path.Join(workdir)
	if err := os.MkdirAll(wd, os.ModePerm); err != nil {
		return nil, err
	}
	parentBase := strings.TrimSuffix(path.Base(parentRepo), ".git")
	fmt.Println("PARENT BASE!!!!!!!!!!!!!!!!")
	fmt.Println(parentBase)
	parentDir := path.Join(wd, parentBase)
	childDir := path.Join(wd, childPath)
	childRepo := &git.Checkout{GitDir: git.GitDir(childDir)}

	//user, err := g.GetUserEmail()
	//if err != nil {
	//	return nil, fmt.Errorf("Failed to determine Gerrit user: %s", err)
	//}
	// Pass in or hardcode the github user name...
	user := "rmistry@google.com"
	sklog.Infof("Repo Manager user: %s", user)

	preUploadSteps, err := GetPreUploadSteps(preUploadStepNames)
	if err != nil {
		return nil, err
	}

	gr := &githubRepoManager{
		depsRepoManager: &depsRepoManager{
			depotToolsRepoManager: &depotToolsRepoManager{
				commonRepoManager: &commonRepoManager{
					parentBranch:   parentBranch,
					childDir:       childDir,
					childPath:      childPath,
					childRepo:      childRepo,
					childBranch:    childBranch,
					g:              g,
					preUploadSteps: preUploadSteps,
					serverURL:      serverURL,
					strategy:       strategy,
					user:           user,
					workdir:        wd,
				},
				depot_tools: depot_tools,
				gclient:     gclient,
				gclientSpec: gclientSpec,
				parentDir:   parentDir,
				parentRepo:  parentRepo,
			},
			includeLog: includeLog,
			rollDep:    rollDep,
		},
	}

	// TODO(borenet): This update can be extremely expensive. Consider
	// moving it out of the startup critical path.
	return gr, gr.Update(ctx)
}

// Update syncs code in the relevant repositories.
func (gr *githubRepoManager) Update(ctx context.Context) error {
	fmt.Println("IN UPDATE!!!!!!!!!!")

	// Sync the projects.
	gr.repoMtx.Lock()
	defer gr.repoMtx.Unlock()

	if err := gr.createAndSyncParent(ctx); err != nil {
		return fmt.Errorf("Could not create and sync parent repo: %s", err)
	}

	// Add upstream remote to the parent repository.
	fmt.Println("Add upstream remote here!!!")
	// Check to see whether there is an upstream yet.
	// git remote add upstream git@github.com:flutter/engine.git
	fmt.Println(filepath.Join(gr.parentDir, "src", "flutter"))
	fmt.Println(gr.parentRepo)
	// TODO(rmistry): Make the path to the main repo configurable via a flag. Not sure what to call it though.
	mainParentDir := filepath.Join(gr.parentDir, "src", "flutter")
	// Check to see whether there is an upstream yet.
	remoteOutput, err := exec.RunCwd(ctx, mainParentDir, "git", "remote", "show")
	if err != nil {
		return err
	}
	if !strings.Contains(remoteOutput, GITHUB_UPSTREAM_REMOTE_NAME) {
		if _, err := exec.RunCwd(ctx, mainParentDir, "git", "remote", "add", GITHUB_UPSTREAM_REMOTE_NAME, gr.parentRepo); err != nil {
			return err
		}
	}
	// Checkout origin/master and create a new branch.
	// git checkout origin/master -b roll-skia
	if _, err := exec.RunCwd(ctx, mainParentDir, "git", "checkout", fmt.Sprintf("origin/%s", gr.parentBranch), "-b", GITHUB_ROLL_BRANCH_NAME); err != nil {
		// Branch probably already exists.
		// return err
	}
	// git pull upstream master
	if _, err := exec.RunCwd(ctx, mainParentDir, "git", "pull", "upstream", gr.parentBranch); err != nil {
		return err
	}

	// Get the last roll revision.
	lastRollRev, err := gr.getLastRollRev(ctx)
	if err != nil {
		return err
	}

	// Get the next roll revision.
	nextRollRev, err := gr.strategy.GetNextRollRev(ctx, gr.childRepo, lastRollRev)
	if err != nil {
		return err
	}

	// Find the number of not-rolled child repo commits.
	notRolled, err := gr.getCommitsNotRolled(ctx, lastRollRev)
	if err != nil {
		return err
	}

	gr.infoMtx.Lock()
	defer gr.infoMtx.Unlock()
	gr.lastRollRev = lastRollRev
	gr.nextRollRev = nextRollRev
	gr.commitsNotRolled = notRolled

	fmt.Println(gr.lastRollRev)
	fmt.Println(nextRollRev)
	fmt.Println(gr.commitsNotRolled)
	sklog.Fatal("GET TO THIS POINT!!!!!!!!!!!")
	return nil
}

//// getLastRollRev returns the commit hash of the last-completed DEPS roll.
//func (r *githubRepoManager) getLastRollRev(ctx context.Context) (string, error) {
//	output, err := exec.RunCwd(ctx, r.childRepo.Dir(), "git", "merge-base", fmt.Sprintf("refs/remotes/remote/%s", r.childBranch), fmt.Sprintf("refs/remotes/goog/%s", r.parentBranch))
//	if err != nil {
//		return "", err
//	}
//	return strings.TrimRight(output, "\n"), nil
//}

// FullChildHash returns the full hash of the given short hash or ref in the
// child repo.
//func (r *githubRepoManager) FullChildHash(ctx context.Context, shortHash string) (string, error) {
//	r.repoMtx.RLock()
//	defer r.repoMtx.RUnlock()
//	return r.childRepo.FullHash(ctx, shortHash)
//}

//// LastRollRev returns the last-rolled child commit.
//func (r *githubRepoManager) LastRollRev() string {
//	r.infoMtx.RLock()
//	defer r.infoMtx.RUnlock()
//	return r.lastRollRev
//}

// setChangeLabels sets the appropriate labels on the Gerrit change.
// It uses the Gerrit REST API to set the following labels on the change:
// * Code-Review=2
// * Autosubmit=1 (if dryRun=false else 0 is set)
// * Presubmit-Ready=1
func (r *githubRepoManager) setChangeLabels(change *gerrit.ChangeInfo, dryRun bool) error {
	labelValues := map[string]interface{}{
		gerrit.CODEREVIEW_LABEL:      "2",
		gerrit.PRESUBMIT_READY_LABEL: "1",
	}
	if dryRun {
		labelValues[gerrit.AUTOSUBMIT_LABEL] = gerrit.AUTOSUBMIT_LABEL_NONE
	} else {
		labelValues[gerrit.AUTOSUBMIT_LABEL] = gerrit.AUTOSUBMIT_LABEL_SUBMIT
	}
	return r.g.SetReview(change, "Roller setting labels to auto-land change.", labelValues)
}

// CreateNewRoll creates and uploads a new Android roll to the given commit.
// Returns the change number of the uploaded roll.
func (r *githubRepoManager) CreateNewRoll(ctx context.Context, from, to string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	//r.repoMtx.Lock()
	//defer r.repoMtx.Unlock()

	//// Update the upstream remote.
	//if _, err := exec.RunCwd(ctx, r.childDir, "git", "fetch", GITHUB_UPSTREAM_REMOTE_NAME); err != nil {
	//	return 0, err
	//}

	//// Create the roll CL.

	//cr := r.childRepo
	//commits, err := cr.RevList(ctx, fmt.Sprintf("%s..%s", from, to))
	//if err != nil {
	//	return 0, fmt.Errorf("Failed to list revisions: %s", err)
	//}

	//// Start the merge.

	//if _, err := exec.RunCwd(ctx, r.childDir, "git", "merge", to, "--no-commit"); err != nil {
	//	// Check to see if this was a merge conflict with IGNORE_MERGE_CONFLICT_FILES.
	//	conflictsOutput, conflictsErr := exec.RunCwd(ctx, r.childDir, "git", "diff", "--name-only", "--diff-filter=U")
	//	if conflictsErr != nil || conflictsOutput == "" {
	//		util.LogErr(conflictsErr)
	//		return 0, fmt.Errorf("Failed to roll to %s. Needs human investigation: %s", to, err)
	//	}
	//	for _, conflict := range strings.Split(conflictsOutput, "\n") {
	//		if conflict == "" {
	//			continue
	//		}
	//		ignoreConflict := false
	//		for _, ignore := range IGNORE_MERGE_CONFLICT_FILES {
	//			if conflict == ignore {
	//				ignoreConflict = true
	//				sklog.Infof("Ignoring conflict in %s", conflict)
	//				break
	//			}
	//		}
	//		if !ignoreConflict {
	//			util.LogErr(r.abortMerge(ctx))
	//			return 0, fmt.Errorf("Failed to roll to %s. Conflicts in %s: %s", to, conflictsOutput, err)
	//		}
	//	}
	//}

	//if err := android_skia_checkout.RunGnToBp(ctx, r.childDir); err != nil {
	//	util.LogErr(r.abortMerge(ctx))
	//	return 0, fmt.Errorf("Error when running gn_to_bp: %s", err)

	//}
	//for _, genFile := range FILES_GENERATED_BY_GN_TO_GP {
	//	if _, err := exec.RunCwd(ctx, r.childDir, "git", "add", genFile); err != nil {
	//		return 0, err
	//	}
	//}

	//// Run the pre-upload steps.
	//for _, s := range r.PreUploadSteps() {
	//	if err := s(ctx, r.workdir); err != nil {
	//		return 0, fmt.Errorf("Failed pre-upload step: %s", err)
	//	}
	//}

	//// Create a new repo branch.
	//if _, repoBranchErr := exec.RunCwd(ctx, r.childDir, r.repoToolPath, "start", REPO_BRANCH_NAME, "."); repoBranchErr != nil {
	//	util.LogErr(r.abortMerge(ctx))
	//	return 0, fmt.Errorf("Failed to create repo branch: %s", repoBranchErr)
	//}

	//// Get list of changes.
	//changeSummaries := []string{}
	//for _, c := range commits {
	//	d, err := cr.Details(ctx, c)
	//	if err != nil {
	//		return 0, err
	//	}
	//	changeSummary := fmt.Sprintf("%s %s %s", d.Timestamp.Format("2006-01-02"), AUTHOR_EMAIL_RE.FindStringSubmatch(d.Author)[1], d.Subject)
	//	changeSummaries = append(changeSummaries, changeSummary)
	//}

	//// Create commit message.
	//commitRange := fmt.Sprintf("%s..%s", from[:9], to[:9])
	//childRepoName := path.Base(r.childDir)
	//commitMsg := fmt.Sprintf(
	//	`Roll %s %s (%d commits)

	//https://%s.googlesource.com/%s.git/+log/%s

	//%s

	//%s

	//Test: Presubmit checks will test this change.
	//Exempt-From-Owner-Approval: The autoroll bot does not require owner approval.
	//`, r.childPath, commitRange, len(commits), childRepoName, childRepoName, commitRange, strings.Join(changeSummaries, "\n"), fmt.Sprintf(COMMIT_MSG_FOOTER_TMPL, r.serverURL))

	//// Loop through all commits:
	//// * Collect all bugs from b/xyz to add the commit message later.
	//// * Add all 'Test: ' lines to the commit message.
	//emailMap := map[string]bool{}
	//bugMap := map[string]bool{}
	//for _, c := range commits {
	//	d, err := cr.Details(ctx, c)
	//	if err != nil {
	//		return 0, err
	//	}
	//	// Extract out the email if it is a Googler.
	//	matches := AUTHOR_EMAIL_RE.FindStringSubmatch(d.Author)
	//	if strings.HasSuffix(matches[1], "@google.com") {
	//		emailMap[matches[1]] = true
	//	}
	//	// Extract out any bugs
	//	for k, v := range ExtractBugNumbers(d.Body) {
	//		bugMap[k] = v
	//	}
	//	// Extract out the Test lines and directly add them to the commit
	//	// message.
	//	for _, tl := range ExtractTestLines(d.Body) {
	//		commitMsg += fmt.Sprintf("\n%s", tl)
	//	}

	//}
	//// Create a single bug line and append it to the commit message.
	//if len(bugMap) > 0 {
	//	bugs := []string{}
	//	for b := range bugMap {
	//		bugs = append(bugs, b)
	//	}
	//	commitMsg += fmt.Sprintf("\nBug: %s", strings.Join(bugs, ", "))
	//}

	//if r.parentBranch != "master" {
	//	// If the parent branch is not master then:
	//	// Add all authors of merged changes to the email list. We do not do this
	//	// for the master branch because developers would get spammed due to multiple
	//	// rolls a day. Release branch rolls run rarely and developers should be
	//	// aware that their changes are being rolled there.
	//	for e := range emailMap {
	//		emails = append(emails, e)
	//	}
	//}
	//emailStr := strings.Join(emails, ",")

	//// Commit the change with the above message.
	//if _, commitErr := exec.RunCwd(ctx, r.childDir, "git", "commit", "-m", commitMsg); commitErr != nil {
	//	util.LogErr(r.abandonRepoBranch(ctx))
	//	return 0, fmt.Errorf("Nothing to merge; did someone already merge %s?: %s", commitRange, commitErr)
	//}

	//// Bypass the repo upload prompt by setting autoupload config to true.
	//if _, configErr := exec.RunCwd(ctx, r.childDir, "git", "config", fmt.Sprintf("review.%s/.autoupload", r.repoUrl), "true"); configErr != nil {
	//	util.LogErr(r.abandonRepoBranch(ctx))
	//	return 0, fmt.Errorf("Could not set autoupload config: %s", configErr)
	//}

	//// Upload the CL to Gerrit.
	//uploadCommand := &exec.Command{
	//	Name: r.repoToolPath,
	//	Args: []string{"upload", fmt.Sprintf("--re=%s", emailStr), "--verify"},
	//	Dir:  r.childDir,
	//	// The below is to bypass the blocking
	//	// "ATTENTION: You are uploading an unusually high number of commits."
	//	// prompt which shows up when a merge contains more than 5 commits.
	//	Stdin: strings.NewReader("yes"),
	//}
	//if _, uploadErr := exec.RunCommand(ctx, uploadCommand); uploadErr != nil {
	//	util.LogErr(r.abandonRepoBranch(ctx))
	//	return 0, fmt.Errorf("Could not upload to Gerrit: %s", uploadErr)
	//}

	//// Get latest hash to find Gerrit change number with.
	//commitHashOutput, revParseErr := exec.RunCwd(ctx, r.childDir, "git", "rev-parse", "HEAD")
	//if revParseErr != nil {
	//	util.LogErr(r.abandonRepoBranch(ctx))
	//	return 0, revParseErr
	//}
	//commitHash := strings.Split(commitHashOutput, "\n")[0]
	//// We no longer need the local branch. Abandon the repo.
	//util.LogErr(r.abandonRepoBranch(ctx))

	//// Get the change number.
	//change, err := r.getChangeForHash(commitHash)
	//if err != nil {
	//	util.LogErr(r.abandonRepoBranch(ctx))
	//	return 0, err
	//}
	//// Set the topic of the merge change.
	//if err := r.setTopic(change.Issue); err != nil {
	//	return 0, err
	//}
	//// Set labels.
	//if err := r.setChangeLabels(change, dryRun); err != nil {
	//	// Only throw exception here if parentBranch is master. This is
	//	// because other branches will not have permissions setup for the
	//	// bot to run CR+2.
	//	if r.parentBranch != "master" {
	//		sklog.Warningf("Could not set labels on %d: %s", change.Issue, err)
	//		sklog.Warningf("Not throwing error because %s branch is not master", r.parentBranch)
	//	} else {
	//		return 0, err
	//	}
	//}

	//return change.Issue, nil

	return -1, nil
}

func (r *githubRepoManager) User() string {
	return r.user
}

//func (r *githubRepoManager) getCommitsNotRolled(ctx context.Context, lastRollRev string) (int, error) {
//	output, err := r.childRepo.Git(ctx, "ls-remote", GITHUB_UPSTREAM_REMOTE_NAME, fmt.Sprintf("refs/heads/%s", r.childBranch), "-1")
//	if err != nil {
//		return -1, err
//	}
//	head := strings.Split(output, "\t")[0]
//	notRolled := 0
//	if head != lastRollRev {
//		commits, err := r.childRepo.RevList(ctx, fmt.Sprintf("%s..%s", lastRollRev, head))
//		if err != nil {
//			return -1, err
//		}
//		notRolled = len(commits)
//	}
//	return notRolled, nil
//}
