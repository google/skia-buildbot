package repo_manager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
)

var (
	// Use this function to instantiate a RepoManager. This is able to be
	// overridden for testing.
	NewNoCheckoutDEPSRepoManager func(context.Context, *NoCheckoutDEPSRepoManagerConfig, string, gerrit.GerritInterface, string, string, string, *http.Client, codereview.CodeReview, bool) (RepoManager, error) = newNoCheckoutDEPSRepoManager
)

// NoCheckoutDEPSRepoManagerConfig provides configuration for RepoManagers which
// don't use a local checkout.
type NoCheckoutDEPSRepoManagerConfig struct {
	NoCheckoutRepoManagerConfig
	// URL of the child repo.
	ChildRepo string `json:"childRepo"` // TODO(borenet): Can we just get this from DEPS?
	// If false, roll CLs do not link to bugs from the commits in the child
	// repo.
	IncludeBugs bool `json:"includeBugs"`
	// If false, roll CLs do not include a git log.
	IncludeLog bool `json:"includeLog"`
	// Branch of the parent repo we want to roll into.
}

func (c *NoCheckoutDEPSRepoManagerConfig) Validate() error {
	if err := c.NoCheckoutRepoManagerConfig.Validate(); err != nil {
		return err
	}
	if c.ChildRepo == "" {
		return errors.New("ChildRepo is required.")
	}
	if c.ParentBranch == "" {
		return errors.New("ParentBranch is required.")
	}
	if c.ParentRepo == "" {
		return errors.New("ParentRepo is required.")
	}
	for _, s := range c.PreUploadSteps {
		if _, err := GetPreUploadStep(s); err != nil {
			return err
		}
	}
	return nil
}

type noCheckoutDEPSRepoManager struct {
	*noCheckoutRepoManager
	childRepo     *gitiles.Repo
	childRepoUrl  string
	depotTools    string
	gclient       string
	includeBugs   bool
	includeLog    bool
	parentRepoUrl string
}

// newNoCheckoutDEPSRepoManager returns a RepoManager instance which does not use
// a local checkout.
func newNoCheckoutDEPSRepoManager(ctx context.Context, c *NoCheckoutDEPSRepoManagerConfig, workdir string, g gerrit.GerritInterface, recipeCfgFile, serverURL, gitcookiesPath string, client *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(workdir, os.ModePerm); err != nil {
		return nil, err
	}

	depotTools, err := depot_tools.GetDepotTools(ctx, workdir, recipeCfgFile)
	if err != nil {
		return nil, err
	}

	rv := &noCheckoutDEPSRepoManager{
		childRepo:     gitiles.NewRepo(c.ChildRepo, gitcookiesPath, client),
		childRepoUrl:  c.ChildRepo,
		depotTools:    depotTools,
		gclient:       path.Join(depotTools, GCLIENT),
		includeBugs:   c.IncludeBugs,
		includeLog:    c.IncludeLog,
		parentRepoUrl: c.ParentRepo,
	}
	ncrm, err := newNoCheckoutRepoManager(ctx, c.NoCheckoutRepoManagerConfig, workdir, g, serverURL, gitcookiesPath, client, cr, rv.createRoll, rv.updateHelper, local)
	if err != nil {
		return nil, err
	}
	rv.noCheckoutRepoManager = ncrm

	return rv, nil
}

// getDEPSFile downloads and returns the path to the DEPS file, and a cleanup
// function to run when finished with it. Requires that the caller holds
// rm.infoMtx.
func (rm *noCheckoutDEPSRepoManager) getDEPSFile(ctx context.Context, baseCommit string) (rv string, cleanup func(), rvErr error) {
	wd, err := ioutil.TempDir("", "")
	if err != nil {
		return "", nil, err
	}
	defer func() {
		if rvErr != nil {
			util.RemoveAll(wd)
		}
	}()

	// Download the DEPS file from the parent repo.
	buf := bytes.NewBuffer([]byte{})
	if err := rm.parentRepo.ReadFileAtRef("DEPS", baseCommit, buf); err != nil {
		return "", nil, err
	}

	// Use "gclient getdep" to retrieve the last roll revision.

	// "gclient getdep" requires a .gclient file.
	if _, err := exec.RunCwd(ctx, wd, "python", rm.gclient, "config", rm.parentRepo.URL); err != nil {
		return "", nil, err
	}
	splitRepo := strings.Split(rm.parentRepo.URL, "/")
	fakeCheckoutDir := path.Join(wd, strings.TrimSuffix(splitRepo[len(splitRepo)-1], ".git"))
	if err := os.Mkdir(fakeCheckoutDir, os.ModePerm); err != nil {
		return "", nil, err
	}
	depsFile := path.Join(fakeCheckoutDir, "DEPS")
	if err := ioutil.WriteFile(depsFile, buf.Bytes(), os.ModePerm); err != nil {
		return "", nil, err
	}
	return depsFile, func() { util.RemoveAll(wd) }, nil
}

// See documentation for noCheckoutRepoManagerCreateRollHelperFunc.
func (rm *noCheckoutDEPSRepoManager) createRoll(ctx context.Context, from, to, serverURL, cqExtraTrybots string, emails []string) (string, map[string]string, error) {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()

	// Build the commit message.
	bugs := []string{}
	monorailProject := issues.REPO_PROJECT_MAPPING[rm.parentRepoUrl]
	if monorailProject == "" {
		sklog.Warningf("Found no entry in issues.REPO_PROJECT_MAPPING for %q", rm.parentRepoUrl)
	}

	nextRollCommits := make([]*vcsinfo.LongCommit, 0, len(rm.notRolledRevs))
	found := false
	for _, rev := range rm.notRolledRevs {
		if rev.Id == to {
			found = true
		}
		if found {
			c, err := rm.childRepo.GetCommit(rev.Id)
			if err != nil {
				return "", nil, err
			}
			nextRollCommits = append(nextRollCommits, c)
		}
	}
	logStr := ""
	for _, c := range nextRollCommits {
		date := c.Timestamp.Format("2006-01-02")
		author := c.Author
		authorSplit := strings.Split(c.Author, "(")
		if len(authorSplit) > 1 {
			author = strings.TrimRight(strings.TrimSpace(authorSplit[1]), ")")
		}
		logStr += fmt.Sprintf("%s %s %s\n", date, author, c.Subject)

		// Bugs list.
		if rm.includeBugs && monorailProject != "" {
			b := util.BugsFromCommitMsg(c.Body)
			for _, bug := range b[monorailProject] {
				bugs = append(bugs, fmt.Sprintf("%s:%s", monorailProject, bug))
			}
		}
	}

	commitMsg, err := buildCommitMsg(from, to, rm.childPath, cqExtraTrybots, rm.childRepoUrl, rm.serverURL, logStr, bugs, len(nextRollCommits), rm.includeLog)
	if err != nil {
		return "", nil, fmt.Errorf("Failed to build commit msg: %s", err)
	}
	commitMsg += "TBR=" + strings.Join(emails, ",")

	// Download the DEPS file from the parent repo.
	depsFile, cleanup, err := rm.getDEPSFile(ctx, rm.baseCommit)
	if err != nil {
		return "", nil, err
	}
	defer cleanup()

	// Write the new DEPS content.
	args := []string{"setdep", "-r", fmt.Sprintf("%s@%s", rm.childPath, to)}
	if _, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  path.Dir(depsFile),
		Env:  depot_tools.Env(rm.depotTools),
		Name: rm.gclient,
		Args: args,
	}); err != nil {
		return "", nil, err
	}

	// Read the updated DEPS content.
	newDEPSContent, err := ioutil.ReadFile(depsFile)
	if err != nil {
		return "", nil, err
	}

	return commitMsg, map[string]string{"DEPS": string(newDEPSContent)}, nil
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) RolledPast(ctx context.Context, hash string) (bool, error) {
	rm.infoMtx.RLock()
	defer rm.infoMtx.RUnlock()
	if hash == rm.lastRollRev {
		return true, nil
	}
	commits, err := rm.childRepo.Log(hash, rm.lastRollRev)
	if err != nil {
		return false, err
	}
	return len(commits) > 0, nil
}

func (rm *noCheckoutDEPSRepoManager) getNextRollRev(ctx context.Context, notRolled []*vcsinfo.LongCommit, lastRollRev string) (string, error) {
	rm.strategyMtx.RLock()
	defer rm.strategyMtx.RUnlock()
	nextRollRev, err := rm.strategy.GetNextRollRev(ctx, notRolled)
	if err != nil {
		return "", err
	}
	if nextRollRev == "" {
		nextRollRev = lastRollRev
	}
	return nextRollRev, nil
}

// See documentation for noCheckoutRepoManagerUpdateHelperFunc.
func (rm *noCheckoutDEPSRepoManager) updateHelper(ctx context.Context, strat strategy.NextRollStrategy, parentRepo *gitiles.Repo, baseCommit string) (string, string, []*Revision, error) {
	rm.infoMtx.Lock()
	defer rm.infoMtx.Unlock()

	depsFile, cleanup, err := rm.getDEPSFile(ctx, baseCommit)
	if err != nil {
		return "", "", nil, err
	}
	defer cleanup()
	output, err := exec.RunCwd(ctx, path.Dir(depsFile), "python", rm.gclient, "getdep", "-r", rm.childPath)
	if err != nil {
		return "", "", nil, err
	}
	splitGetdep := strings.Split(strings.TrimSpace(output), "\n")
	lastRollRev := strings.TrimSpace(splitGetdep[len(splitGetdep)-1])
	if len(lastRollRev) != 40 {
		return "", "", nil, fmt.Errorf("Got invalid output for `gclient getdep`: %s", output)
	}

	// Find the not-yet-rolled child repo commits.
	// Only consider commits on the "main" branch as roll candidates.
	notRolled, err := rm.childRepo.LogLinear(lastRollRev, rm.childBranch)
	if err != nil {
		return "", "", nil, err
	}

	// Get the next roll revision.
	nextRollRev, err := rm.getNextRollRev(ctx, notRolled, lastRollRev)
	if err != nil {
		return "", "", nil, err
	}

	// Get the list of not-yet-rolled revisions.
	notRolledRevs := make([]*Revision, 0, len(notRolled))
	for _, rev := range notRolled {
		notRolledRevs = append(notRolledRevs, &Revision{
			Id:          rev.Hash,
			Display:     rev.Hash[:7],
			Description: rev.Subject,
			Timestamp:   rev.Timestamp,
			URL:         fmt.Sprintf("%s/+/%s", rm.childRepoUrl, rev.Hash),
		})
	}

	return lastRollRev, nextRollRev, notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (rm *noCheckoutDEPSRepoManager) FullChildHash(ctx context.Context, ref string) (string, error) {
	c, err := rm.childRepo.GetCommit(ref)
	if err != nil {
		return "", err
	}
	return c.Hash, nil
}

// See documentation for RepoManager interface.
func (r *noCheckoutDEPSRepoManager) CreateNextRollStrategy(ctx context.Context, s string) (strategy.NextRollStrategy, error) {
	return strategy.GetNextRollStrategy(ctx, s, r.childBranch, DEFAULT_REMOTE, "", []string{}, nil, nil)
}

// See documentation for RepoManager interface.
func (r *noCheckoutDEPSRepoManager) SetStrategy(s strategy.NextRollStrategy) {
	r.strategyMtx.Lock()
	defer r.strategyMtx.Unlock()
	r.strategy = s
}

// See documentation for RepoManager interface.
func (r *noCheckoutDEPSRepoManager) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// See documentation for RepoManager interface.
func (r *noCheckoutDEPSRepoManager) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
		strategy.ROLL_STRATEGY_SINGLE,
	}
}
