package repo_manager

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	//"time"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"

	//"go.skia.org/infra/go/common"
	//"github.com/google/go-github/github"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/travisci"
	"go.skia.org/infra/go/util"
)

const (
	//SERVICE_ACCOUNT      = "31977622648@project.gserviceaccount.com"
	//UPSTREAM_REMOTE_NAME = "remote"
	//REPO_BRANCH_NAME     = "merge"

	GITHUB_UPSTREAM_REMOTE_NAME = "remote"
)

var (
	// Use this function to instantiate a NewGithubRepoManager. This is able to be
	// overridden for testing.
	NewGithubRepoManager func(context.Context, string, string, string, string, string, string, *github.GitHub, *travisci.TravisCI, NextRollStrategy, []string, bool, string, string) (RepoManager, error) = newGithubRepoManager

	//IGNORE_MERGE_CONFLICT_FILES = []string{android_skia_checkout.SkUserConfigRelPath}

	//FILES_GENERATED_BY_GN_TO_GP = []string{android_skia_checkout.SkUserConfigRelPath, android_skia_checkout.AndroidBpRelPath}

	//AUTHOR_EMAIL_RE = regexp.MustCompile(".* \\((.*)\\)")
)

// androidRepoManager is a struct used by the autoroller for managing checkouts.
type githubRepoManager struct {
	*depsRepoManager
	githubClient *github.GitHub
	travisClient *travisci.TravisCI
}

// newGithubRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func newGithubRepoManager(ctx context.Context, workdir, parentRepo, parentBranch, childPath, childBranch, depot_tools string, g *github.GitHub, t *travisci.TravisCI, strategy NextRollStrategy, preUploadStepNames []string, includeLog bool, gclientSpec, serverURL string) (RepoManager, error) {

	// Github API client tests here!

	//client := github.NewClient(g)

	// Github API client tests here.

	gclient := path.Join(depot_tools, GCLIENT)
	rollDep := path.Join(depot_tools, ROLL_DEP)

	// TODO(rmistry): Get this from the flag for repo name? (should be engine)
	wd := path.Join(workdir, strings.TrimSuffix(path.Base(parentRepo), ".git"))
	if err := os.MkdirAll(wd, os.ModePerm); err != nil {
		return nil, fmt.Errorf("Error when creating %s: %s", wd, err)
	}
	// Run gclient sync in the wd to get pub/dart binaries downloaded.
	if _, err := exec.RunCwd(ctx, filepath.Join(wd, "src"), filepath.Join(depot_tools, "gclient"), "sync"); err != nil {
		return nil, fmt.Errorf("Error when running gclient sync: %s", err)
	}

	// TODO(rmistry): Construct this from the 2 new flags for github rolls
	parentDir := path.Join(wd, "src/flutter")
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
		githubClient: g,
		travisClient: t,
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

	if err := gr.createAndSyncDir(ctx, gr.workdir, gr.parentBranch); err != nil {
		return fmt.Errorf("Could not create and sync parent repo: %s", err)
	}

	// Add upstream remote to the parent repository.
	fmt.Println("Add upstream remote here!!!")
	// Check to see whether there is an upstream yet.
	// git remote add upstream git@github.com:flutter/engine.git
	fmt.Println(filepath.Join(gr.parentDir, "src", "flutter"))
	fmt.Println(gr.parentRepo)

	// Get on the roll branch
	// git checkout origin/roll_skia -b roll-skia
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "checkout", fmt.Sprintf("origin/%s", GITHUB_UPSTREAM_REMOTE_NAME), "-b", ROLL_BRANCH); err != nil {
		// Branch probably already exists.
		// return err
	}

	// TODO(rmistry): Make the path to the main repo configurable via a flag. Not sure what to call it though.
	// Check to see whether there is an upstream yet.
	remoteOutput, err := exec.RunCwd(ctx, gr.parentDir, "git", "remote", "show")
	if err != nil {
		return err
	}
	fmt.Println("REMOTE OUTPUT!!!!!!!!!!!")
	fmt.Println(remoteOutput)
	fmt.Println(!strings.Contains(remoteOutput, GITHUB_UPSTREAM_REMOTE_NAME))

	if !strings.Contains(remoteOutput, GITHUB_UPSTREAM_REMOTE_NAME) {
		fmt.Println("ADDING IT!!!!!!!")
		if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "remote", "add", GITHUB_UPSTREAM_REMOTE_NAME, gr.parentRepo); err != nil {
			return err
		}
	}

	//git fetch upstream
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "fetch", GITHUB_UPSTREAM_REMOTE_NAME, gr.parentBranch); err != nil {
		return err
	}
	//git reset --hard remote/master
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "reset", "--hard", fmt.Sprintf("remote/%s", gr.parentBranch)); err != nil {
		return err
	}
	// This is to get the must upto date info from remote

	//// Checkout origin/master and create a new branch.
	//// git checkout origin/roll_skia -b roll-skia
	//if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "checkout", fmt.Sprintf("origin/%s", GITHUB_UPSTREAM_REMOTE_NAME), "-b", ROLL_BRANCH); err != nil {
	//	// Branch probably already exists.
	//	// return err
	//}

	//git remote add upstream /url/to/original/repo
	//git fetch upstream
	//git checkout master

	//git fetch upstream
	//if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "fetch", GITHUB_UPSTREAM_REMOTE_NAME, gr.parentBranch); err != nil {
	//	return err
	//}
	////git reset --hard remote/master
	//if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "reset", "--hard", fmt.Sprintf("remote/%s", gr.parentBranch)); err != nil {
	//	return err
	//}
	////git push origin master --force (nope not master!!)
	//if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "push", "origin", ROLL_BRANCH, "-f"); err != nil {
	//	return err
	//}

	// git pull upstream master to get latest DEPS.
	//// TODO(rmistry): Problem. This does not get the latest DEPS. It gets the latest deps from the rmistry branch.
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "pull", GITHUB_UPSTREAM_REMOTE_NAME, gr.parentBranch); err != nil {
		return err
	}
	// gclient sync again to get latest version of child.
	if err := gr.createAndSyncDir(ctx, gr.workdir, gr.parentBranch); err != nil {
		return fmt.Errorf("Could not create and sync parent repo: %s", err)
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
	//sklog.Fatal("GET TO THIS POINT!!!!!!!!!!!")
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

//// setChangeLabels sets the appropriate labels on the Gerrit change.
//// It uses the Gerrit REST API to set the following labels on the change:
//// * Code-Review=2
//// * Autosubmit=1 (if dryRun=false else 0 is set)
//// * Presubmit-Ready=1
//func (r *githubRepoManager) setChangeLabels(change *gerrit.ChangeInfo, dryRun bool) error {
//	labelValues := map[string]interface{}{
//		gerrit.CODEREVIEW_LABEL:      "2",
//		gerrit.PRESUBMIT_READY_LABEL: "1",
//	}
//	if dryRun {
//		labelValues[gerrit.AUTOSUBMIT_LABEL] = gerrit.AUTOSUBMIT_LABEL_NONE
//	} else {
//		labelValues[gerrit.AUTOSUBMIT_LABEL] = gerrit.AUTOSUBMIT_LABEL_SUBMIT
//	}
//	return r.g.SetReview(change, "Roller setting labels to auto-land change.", labelValues)
//}

// cleanParent forces the parent checkout into a clean state.
func (gr *githubRepoManager) cleanParent(ctx context.Context) error {
	// Clean the parent
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "clean", "-d", "-f", "-f"); err != nil {
		return err
	}
	_, _ = exec.RunCwd(ctx, gr.parentDir, "git", "rebase", "--abort")

	//if _, err := exec.RunCwd(ctx, r.parentDir, "git", "checkout", fmt.Sprintf("origin/%s", r.parentBranch), "-f"); err != nil {
	//	return err
	//}
	//_, _ = exec.RunCwd(ctx, gr.parentDir, "git", "branch", "-D", ROLL_BRANCH)
	//if _, err := exec.RunCommand(ctx, &exec.Command{
	//	Dir:  gr.workdir,
	//	Env:  gr.GetEnvForDepotTools(),
	//	Name: "python",
	//	Args: []string{gr.gclient, "revert", "--nohooks"},
	//}); err != nil {
	//	return err
	//}
	return nil
}

// CreateNewRoll creates and uploads a new Android roll to the given commit.
// Returns the change number of the uploaded roll.
func (gr *githubRepoManager) CreateNewRoll(ctx context.Context, from, to string, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {

	// TODO(rmistyr)": Use default branch name to avoid confusion in cleanParent!
	fmt.Println("IN CREATE NEW ROLL!!!!!")

	gr.repoMtx.Lock()
	defer gr.repoMtx.Unlock()

	// Clean the checkout.
	if err := gr.cleanParent(ctx); err != nil {
		return 0, err
	}

	// Defer some more cleanup.
	defer func() {
		util.LogErr(gr.cleanParent(ctx))
	}()

	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "config", "user.name", gr.user); err != nil {
		return 0, err
	}
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "config", "user.email", gr.user); err != nil {
		return 0, err
	}

	// Make sure the forked repo is checked out at the same hash as the target repo.
	//git fetch upstream
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "fetch", GITHUB_UPSTREAM_REMOTE_NAME, gr.parentBranch); err != nil {
		return 0, err
	}
	//git reset --hard remote/master
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "reset", "--hard", fmt.Sprintf("remote/%s", gr.parentBranch)); err != nil {
		return 0, err
	}
	//git push origin roll_branch --force (do not push to master!!)
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "push", "origin", ROLL_BRANCH, "-f"); err != nil {
		return 0, err
	}
	// Run gclient sync to make third_party/skia match the new DEPS.
	if _, err := exec.RunCwd(ctx, gr.parentDir, filepath.Join(gr.depot_tools, "gclient"), "sync"); err != nil {
		return 0, fmt.Errorf("Error when running gclient sync: %s", err)
	}

	//sklog.Fatal("EXAMINE STUFF! FAILING HERE!!!!!!")

	//cr := gr.childRepo
	//commits, err := cr.RevList(ctx, fmt.Sprintf("%s..%s", from, to))
	//if err != nil {
	//	return 0, fmt.Errorf("Failed to list revisions: %s", err)
	//}
	//// Find relevant bugs.
	//bugs := []string{}
	//monorailProject := issues.REPO_PROJECT_MAPPING[dr.parentRepo]
	//if monorailProject == "" {
	//	sklog.Warningf("Found no entry in issues.REPO_PROJECT_MAPPING for %q", dr.parentRepo)
	//} else {
	//	for _, c := range commits {
	//		d, err := cr.Details(ctx, c)
	//		if err != nil {
	//			return 0, fmt.Errorf("Failed to obtain commit details: %s", err)
	//		}
	//		b := util.BugsFromCommitMsg(d.Body)
	//		for _, bug := range b[monorailProject] {
	//			bugs = append(bugs, fmt.Sprintf("%s:%s", monorailProject, bug))
	//		}
	//	}
	//}

	// Create the roll CL.

	// Run roll-dep.
	args := []string{filepath.Join("..", gr.childPath), "--ignore-dirty-tree", "--roll-to", to}
	//if len(bugs) > 0 {
	//	args = append(args, "--bug", strings.Join(bugs, ","))
	//}
	if !gr.includeLog {
		args = append(args, "--no-log")
	}
	sklog.Infof("Running command: roll-dep %s", strings.Join(args, " "))
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  gr.parentDir,
		Env:  gr.GetEnvForDepotTools(),
		Name: gr.rollDep,
		Args: args,
	}); err != nil {
		return 0, err
	}
	// Build the commit message, starting with the message provided by roll-dep.
	commitMsg, err := exec.RunCwd(ctx, gr.parentDir, "git", "log", "-n1", "--format=%B", "HEAD")
	if err != nil {
		return 0, err
	}
	commitMsg += fmt.Sprintf(COMMIT_MSG_FOOTER_TMPL, gr.serverURL)

	// This will be the license step!!!!!!!
	// Run the pre-upload steps.
	for _, s := range gr.PreUploadSteps() {
		if err := s(ctx, gr.parentDir); err != nil {
			return 0, fmt.Errorf("Failed pre-upload step: %s", err)
		}
	}

	// rmistry: Remove this!!
	//sklog.Fatal("EXAMINE STUFF! FAILING HERE!!!!!!")

	// Push to the forked repository. Does this work??
	// Do you really need the -f? removed!
	if _, err := exec.RunCwd(ctx, gr.parentDir, "git", "push", "origin", ROLL_BRANCH, "-f"); err != nil {
		return 0, err
	}

	// Create a pull request now.
	// Grab the first line of the commit msg to use as the title of the pull request. Nope
	title := strings.Split(commitMsg, "\n")[0]
	//title := fmt.Sprintf("Rolled Skia to %s", to)
	headBranch := fmt.Sprintf("%s:%s", strings.Split(gr.user, "@")[0], ROLL_BRANCH)
	pr, err := gr.githubClient.CreatePullRequest(title, gr.parentBranch, headBranch)
	if err != nil {
		return 0, err
	}

	// After this  you need
	fmt.Println("CREATED IT!!!!!!!!!!!!!!!!!!1")
	//fmt.Println(commits)
	fmt.Println(pr.GetNumber())
	// 4876 is what you need to return!!!

	// Create new pull request now and reutrn it's number???

	return int64(pr.GetNumber()), nil

	// This should use the pull request API I think???!!?!?
	//// Upload the CL.
	//uploadCmd := &exec.Command{
	//	Dir:     dr.parentDir,
	//	Env:     dr.GetEnvForDepotTools(),
	//	Name:    "git",
	//	Args:    []string{"cl", "upload", "--bypass-hooks", "-f", "-v", "-v"},
	//	Timeout: 2 * time.Minute,
	//}
	//if dryRun {
	//	uploadCmd.Args = append(uploadCmd.Args, "--cq-dry-run")
	//} else {
	//	uploadCmd.Args = append(uploadCmd.Args, "--use-commit-queue")
	//}
	//uploadCmd.Args = append(uploadCmd.Args, "--gerrit")
	//tbr := "\nTBR="
	//if emails != nil && len(emails) > 0 {
	//	emailStr := strings.Join(emails, ",")
	//	tbr += emailStr
	//	uploadCmd.Args = append(uploadCmd.Args, "--send-mail", "--cc", emailStr)
	//}
	//commitMsg += tbr
	//uploadCmd.Args = append(uploadCmd.Args, "-m", commitMsg)

	//// Upload the CL.
	//sklog.Infof("Running command: git %s", strings.Join(uploadCmd.Args, " "))
	//if _, err := exec.RunCommand(ctx, uploadCmd); err != nil {
	//	return 0, err
	//}

	//// Obtain the issue number.
	//tmp, err := ioutil.TempDir("", "")
	//if err != nil {
	//	return 0, err
	//}
	//defer util.RemoveAll(tmp)
	//jsonFile := path.Join(tmp, "issue.json")
	//if _, err := exec.RunCommand(ctx, &exec.Command{
	//	Dir:  dr.parentDir,
	//	Env:  dr.GetEnvForDepotTools(),
	//	Name: "git",
	//	Args: []string{"cl", "issue", fmt.Sprintf("--json=%s", jsonFile)},
	//}); err != nil {
	//	return 0, err
	//}
	//f, err := os.Open(jsonFile)
	//if err != nil {
	//	return 0, err
	//}
	//var issue issueJson
	//if err := json.NewDecoder(f).Decode(&issue); err != nil {
	//	return 0, err
	//}
	//return issue.Issue, nil
}

func (r *githubRepoManager) User() string {
	return r.user
}

func (r *githubRepoManager) GetFullHistoryUrl() string {
	// TODO(rmistry): Use flags.
	repoUser := "flutter"
	repoName := "engine"
	user := strings.Split(r.user, "@")[0]
	return fmt.Sprintf("https://github.com/%s/%s/pulls/%s", repoUser, repoName, user)
}

// TODO(rmistry): Change all of these to gr instead of r.
func (r *githubRepoManager) GetIssueUrlBase() string {
	// TODO(rmistry): Use flags.
	repoUser := "flutter"
	repoName := "engine"
	return fmt.Sprintf("https://github.com/%s/%s/pull/", repoUser, repoName)
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
