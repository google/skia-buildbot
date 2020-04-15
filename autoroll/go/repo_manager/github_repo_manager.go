package repo_manager

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	TMPL_COMMIT_MSG_GITHUB = `Roll {{.ChildPath}} {{.RollingFrom.String}}..{{.RollingTo.String}} ({{len .Revisions}} commits)

{{.ChildRepo}}/compare/{{.RollingFrom.String}}...{{.RollingTo.String}}

{{if .IncludeLog}}git log {{.RollingFrom}}..{{.RollingTo}} --first-parent --oneline
{{range .Revisions}}{{.Timestamp.Format "2006-01-02"}} {{.Author}} {{.Description}}
{{end}}{{end}}{{if len .TransitiveDeps}}
Also rolling transitive DEPS:
{{range .TransitiveDeps}}  {{.ParentPath}} {{.RollingFrom}}..{{.RollingTo}}
{{end}}{{end}}

If this roll has caused a breakage, revert this CL and stop the roller
using the controls here:
{{.ServerURL}}
Please CC {{stringsJoin .Reviewers ","}} on the revert to ensure that a human
is aware of the problem.

To report a problem with the AutoRoller itself, please file a bug:
https://bugs.chromium.org/p/skia/issues/entry?template=Autoroller+Bug

Documentation for the AutoRoller is here:
https://skia.googlesource.com/buildbot/+/master/autoroll/README.md

`
)

var (
	pullRequestInLogRE = regexp.MustCompile(`(?m) \((#[0-9]+)\)$`)
)

// GithubRepoManagerConfig provides configuration for the Github RepoManager.
type GithubRepoManagerConfig struct {
	CommonRepoManagerConfig
	// URL of the child repo.
	ChildRepoURL string `json:"childRepoURL"`
	// The roller will update this file with the child repo's revision.
	RevisionFile string `json:"revisionFile"`

	BuildbucketRevisionFilter *BuildbucketRevisionFilterConfig `json:"buildbucketFilter"`

	// Optional; transitive dependencies to roll. This is a mapping of
	// dependencies of the child repo which are also dependencies of the
	// parent repo and should be rolled at the same time. Keys are paths
	// to transitive dependencies within the child repo (as specified in
	// DEPS), and values are paths to the files that must be updated within
	// the parent repo. The files just contain the revision ID
	// (eg: https://github.com/flutter/flutter/pull/51569/files).
	TransitiveDeps map[string]string `json:"transitiveDeps"`
}

type BuildbucketRevisionFilterConfig struct {
	Project string `json:"project"`
	Bucket  string `json:"bucket"`
}

// githubRepoManager is a struct used by the autoroller for managing checkouts.
type githubRepoManager struct {
	*commonRepoManager
	githubClient  *github.GitHub
	parentRepo    *git.Checkout
	parentRepoURL string
	childRepoURL  string
	revisionFile  string

	revFilter RevisionFilter

	transitiveDeps map[string]string
	// Will be used for getdeps if transitive dependences are specified in the
	// config.
	depotTools string
}

// Refactor this out to commonRepoManager one day to be able to define any
// filter for any roller.
type RevisionFilter interface {
	// Skip returns a non-empty string if the revision should be skipped. The
	// string will contain the reason the revision should be skipped. An empty
	// string is returned if the revision should not be skipped.
	// If an error is returned then an empty string will be returned.
	Skip(context.Context, *revision.Revision) (string, error)
}
type bbRevisionFilter struct {
	bb      buildbucket.BuildBucketInterface
	project string
	bucket  string
}

// See RevisionFilter interface.
func (f bbRevisionFilter) Skip(ctx context.Context, r *revision.Revision) (string, error) {
	pred := &buildbucketpb.BuildPredicate{
		Builder: &buildbucketpb.BuilderID{Project: f.project, Bucket: f.bucket},
		Tags: []*buildbucketpb.StringPair{
			{Key: "buildset", Value: fmt.Sprintf("commit/git/%s", r.Id)},
		},
	}
	builds, err := f.bb.Search(ctx, pred)
	if err != nil {
		return "", err
	}
	if len(builds) == 0 {
		sklog.Infof("[bbFilter] Builds for %s have not started yet", r.Id)
		return fmt.Sprintf("Builds have not started yet"), nil
	}

	// statuses stores the statuses of builders. This is used to account for luci build retries.
	// It is used to determine if there was any successful build for a builder. We should have ideally used
	// the most recent status but there appears to be strange behavior with flutter luci builds where
	// INFRA_FAILURE builds appear to be coming after SUCCESSFUL builds. Eg:
	// https://cr-buildbucket.appspot.com/rpcexplorer/services/buildbucket.v2.Builds/SearchBuilds?request={"predicate":{"builder":{"project": "flutter","bucket": "prod"},"tags":[{"key": "buildset","value": "commit/git/18962926012965f815c273e58409cda3144998f5"}]}}
	// This has been brought up with the flutter team.
	statuses := map[string]buildbucketpb.Status{}
	for _, build := range builds {
		prev, ok := statuses[build.Builder.Builder]
		if !ok || prev != buildbucketpb.Status_SUCCESS {
			statuses[build.Builder.Builder] = build.Status
		}
	}
	for b, status := range statuses {
		if status == buildbucketpb.Status_SUCCESS {
			sklog.Infof("[bbFilter] Found successful build of \"%s\" for %s", b, r.Id)
		} else {
			sklog.Infof("[bbFilter] Could not find successful build of \"%s\" for %s: %s", b, r.Id, status)
			return fmt.Sprintf("Luci builds of \"%s\" for %s was %s", b, r.Id, status), nil
		}
	}
	sklog.Infof("[bbFilter] All builds of %s were %s", r.Id, buildbucketpb.Status_SUCCESS)
	return "", nil
}

func newBuildbucketRevisionFilter(client *http.Client, project, bucket string) (*bbRevisionFilter, error) {
	if project == "" || bucket == "" {
		return nil, errors.New("Both project and bucket must be specified for buildbucketFilter.")
	}
	return &bbRevisionFilter{
		bb:      buildbucket.NewClient(client),
		project: project,
		bucket:  bucket,
	}, nil
}

// NewGithubRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewGithubRepoManager(ctx context.Context, c *GithubRepoManagerConfig, reg *config_vars.Registry, workdir string, githubClient *github.GitHub, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (RepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	wd := path.Join(workdir, "github_repos")
	if _, err := os.Stat(wd); err != nil {
		if err := os.MkdirAll(wd, 0755); err != nil {
			return nil, err
		}
	}

	// Create and populate the parent directory if needed.
	_, repo := GetUserAndRepo(c.ParentRepo)
	userFork := fmt.Sprintf("git@github.com:%s/%s.git", cr.UserName(), repo)
	parentRepo, err := git.NewCheckout(ctx, userFork, wd)
	if err != nil {
		return nil, err
	}

	if c.CommitMsgTmpl == "" {
		c.CommitMsgTmpl = TMPL_COMMIT_MSG_GITHUB
	}
	crm, err := newCommonRepoManager(ctx, c.CommonRepoManagerConfig, reg, wd, serverURL, nil, client, cr, local)
	if err != nil {
		return nil, err
	}

	// Create and populate the child directory if needed.
	if _, err := os.Stat(crm.childDir); err != nil {
		if err := os.MkdirAll(crm.childDir, 0755); err != nil {
			return nil, err
		}
		if _, err := git.GitDir(crm.childDir).Git(ctx, "clone", c.ChildRepoURL, "."); err != nil {
			return nil, err
		}
	}

	var f RevisionFilter
	if c.BuildbucketRevisionFilter != nil {
		f, err = newBuildbucketRevisionFilter(client, c.BuildbucketRevisionFilter.Project, c.BuildbucketRevisionFilter.Bucket)
		if err != nil {
			return nil, err
		}
	}

	depotTools, err := depot_tools.GetDepotTools(ctx, workdir, recipeCfgFile)
	if err != nil {
		return nil, err
	}

	gr := &githubRepoManager{
		commonRepoManager: crm,
		githubClient:      githubClient,
		parentRepo:        parentRepo,
		parentRepoURL:     c.ParentRepo,
		childRepoURL:      c.ChildRepoURL,
		revisionFile:      c.RevisionFile,
		revFilter:         f,
		transitiveDeps:    c.TransitiveDeps,
		depotTools:        depotTools,
	}

	return gr, nil
}

// Fix pull request linkification in the commit details.
func (rm *githubRepoManager) fixPullRequestLinks(rev *revision.Revision) {
	user, repo := GetUserAndRepo(rm.childRepoURL)
	// Github autolinks PR numbers to be of the same repository in logStr. Fix this by
	// explicitly adding the child repo to the PR number.
	rev.Description = pullRequestInLogRE.ReplaceAllString(rev.Description, fmt.Sprintf(" (%s/%s$1)", user, repo))
	rev.Details = pullRequestInLogRE.ReplaceAllString(rev.Details, fmt.Sprintf(" (%s/%s$1)", user, repo))
}

// See documentation for RepoManager interface.
func (rm *githubRepoManager) Update(ctx context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	// Sync the projects.
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()

	// Update the repositories.
	if err := rm.parentRepo.Update(ctx); err != nil {
		return nil, nil, nil, err
	}
	if err := rm.childRepo.Update(ctx); err != nil {
		return nil, nil, nil, err
	}

	// Check to see whether there is an upstream yet.
	remoteOutput, err := rm.parentRepo.Git(ctx, "remote", "show")
	if err != nil {
		return nil, nil, nil, err
	}
	remoteFound := false
	remoteLines := strings.Split(remoteOutput, "\n")
	for _, remoteLine := range remoteLines {
		if remoteLine == GITHUB_UPSTREAM_REMOTE_NAME {
			remoteFound = true
			break
		}
	}
	if !remoteFound {
		if _, err := rm.parentRepo.Git(ctx, "remote", "add", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentRepoURL); err != nil {
			return nil, nil, nil, err
		}
	}
	// Fetch upstream.
	if _, err := rm.parentRepo.Git(ctx, "fetch", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentBranch.String()); err != nil {
		return nil, nil, nil, err
	}

	// Read the contents of the revision file to determine the last roll rev.
	revisionFileContents, err := rm.githubClient.ReadRawFile(rm.parentBranch.String(), rm.revisionFile)
	if err != nil {
		return nil, nil, nil, err
	}
	lastRollHash := strings.TrimRight(revisionFileContents, "\n")
	lastRollDetails, err := rm.childRepo.Details(ctx, lastRollHash)
	if err != nil {
		return nil, nil, nil, err
	}
	lastRollRev := revision.FromLongCommit(rm.childRevLinkTmpl, lastRollDetails)
	rm.fixPullRequestLinks(lastRollRev)

	// Get the tip-of-tree revision. Because we filter the notRolledRevs,
	// this may not end up being present in that list.
	tipRev, err := rm.getTipRev(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	rm.fixPullRequestLinks(tipRev)

	// Find the not-rolled child repo commits.
	notRolledRevs, err := rm.getCommitsNotRolled(ctx, lastRollRev, tipRev)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, rev := range notRolledRevs {
		rm.fixPullRequestLinks(rev)
	}

	// Optionally filter not-rolled revisions.
	if rm.revFilter != nil {
		for _, notRolledRev := range notRolledRevs {
			invalidReason, err := rm.revFilter.Skip(ctx, notRolledRev)
			if err != nil {
				return nil, nil, nil, err
			}
			if invalidReason != "" {
				notRolledRev.InvalidReason = invalidReason
			}
		}
	}

	return lastRollRev, tipRev, notRolledRevs, nil
}

func (rm *githubRepoManager) cleanParent(ctx context.Context) error {
	if _, err := rm.parentRepo.Git(ctx, "clean", "-d", "-f", "-f"); err != nil {
		return err
	}
	_, _ = rm.parentRepo.Git(ctx, "rebase", "--abort")
	if _, err := rm.parentRepo.Git(ctx, "checkout", fmt.Sprintf("%s/%s", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentBranch), "-f"); err != nil {
		return err
	}
	_, _ = rm.parentRepo.Git(ctx, "branch", "-D", ROLL_BRANCH)
	return nil
}

// See documentation for RepoManager interface.
func (rm *githubRepoManager) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	rm.repoMtx.Lock()
	defer rm.repoMtx.Unlock()

	sklog.Info("Creating a new Github Roll")

	// Clean the checkout, get onto a fresh branch.
	if err := rm.cleanParent(ctx); err != nil {
		return 0, err
	}
	if _, err := rm.parentRepo.Git(ctx, "checkout", fmt.Sprintf("%s/%s", GITHUB_UPSTREAM_REMOTE_NAME, rm.parentBranch), "-b", ROLL_BRANCH); err != nil {
		return 0, err
	}
	// Defer cleanup.
	defer func() {
		util.LogErr(rm.cleanParent(ctx))
	}()

	// Make sure the forked repo is at the same hash as the target repo before
	// creating the pull request.
	if _, err := rm.parentRepo.Git(ctx, "push", "origin", ROLL_BRANCH, "-f"); err != nil {
		return 0, err
	}

	// Make sure the right name and email are set.
	if !rm.local {
		if _, err := rm.parentRepo.Git(ctx, "config", "user.name", rm.codereview.UserName()); err != nil {
			return 0, err
		}
		if _, err := rm.parentRepo.Git(ctx, "config", "user.email", rm.codereview.UserEmail()); err != nil {
			return 0, err
		}
	}

	// Build the commit message.
	childRepo := strings.ReplaceAll(rm.childRepoURL, "git@github.com:", "https://github.com/")
	childRepo = strings.ReplaceAll(childRepo, ".git", "")
	commitMsg, err := rm.buildCommitMsg(&parent.CommitMsgVars{
		ChildPath:   rm.childPath,
		ChildRepo:   childRepo,
		Reviewers:   emails,
		Revisions:   rolling,
		RollingFrom: from,
		RollingTo:   to,
		ServerURL:   rm.serverURL,
	})

	for i := len(rolling) - 1; i >= 0; i-- {
		// Write the file.
		if err := ioutil.WriteFile(path.Join(rm.parentRepo.Dir(), rm.revisionFile), []byte(rolling[i].Id+"\n"), os.ModePerm); err != nil {
			return 0, err
		}

		// Commit.
		msg := fmt.Sprintf("%s %s", rolling[i].Id[:9], rolling[i].Description)
		if _, err := rm.parentRepo.Git(ctx, "commit", "-a", "-m", msg); err != nil {
			return 0, err
		}
	}

	// Update any transitive DEPS.
	if len(rm.transitiveDeps) > 0 {
		for childPath, parentPath := range rm.transitiveDeps {
			output, err := exec.RunCwd(ctx, rm.childRepo.Dir(), "python", path.Join(rm.depotTools, parent.GClient), "getdep", "-r", childPath, "--spec={\"host_os\":\"linux\"}")
			if err != nil {
				return 0, err
			}
			targetRev := strings.TrimSpace(output)
			// TODO(rmistry): Is this always 44 chars?
			if len(targetRev) != 44 {
				return 0, fmt.Errorf("Got invalid output for `gclient getdep`: %s", output)
			}

			// Compare with the already existing contents to see if anything needs to be updated.
			parentFilePath := path.Join(rm.parentRepo.Dir(), parentPath)
			existingContents, err := ioutil.ReadFile(parentFilePath)
			if err != nil {
				return 0, err
			}
			if strings.TrimSpace(string(existingContents)) == targetRev {
				sklog.Infof("%s is already in %s. Not going to update it.", targetRev, parentFilePath)
			} else {
				//Update the file in parentPath with the targetRev.
				if err := ioutil.WriteFile(parentFilePath, []byte(targetRev+"\n"), os.ModePerm); err != nil {
					return 0, err
				}
				// Commit.
				msg := fmt.Sprintf("Updated %s", parentPath)
				if _, err := rm.parentRepo.Git(ctx, "commit", "-a", "-m", msg); err != nil {
					return 0, err
				}
			}
		}
	}

	// Run the pre-upload steps.
	for _, s := range rm.preUploadSteps {
		if err := s(ctx, nil, rm.httpClient, rm.parentRepo.Dir()); err != nil {
			return 0, fmt.Errorf("Error when running pre-upload step: %s", err)
		}
	}

	// Push to the forked repository.
	if _, err := rm.parentRepo.Git(ctx, "push", "origin", ROLL_BRANCH, "-f"); err != nil {
		return 0, err
	}

	// Grab the first line of the commit msg to use as the title of the pull request.
	title := strings.Split(commitMsg, "\n")[0]
	// Use the remaining part of the commit message as the pull request description.
	commitMsgLines := strings.Split(commitMsg, "\n")
	var descComment []string
	if len(commitMsgLines) > 50 {
		// Truncate too large description comment because Github API cannot handle large comments.
		descComment = commitMsgLines[1:50]
		descComment = append(descComment, "...")
	} else {
		descComment = commitMsgLines[1:]
	}
	// Create a pull request.
	headBranch := fmt.Sprintf("%s:%s", rm.codereview.UserName(), ROLL_BRANCH)
	pr, err := rm.githubClient.CreatePullRequest(title, rm.parentBranch.String(), headBranch, strings.Join(descComment, "\n"))
	if err != nil {
		return 0, err
	}

	// Add appropriate label to the pull request.
	if !dryRun {
		if err := rm.githubClient.AddLabel(pr.GetNumber(), github.WAITING_FOR_GREEN_TREE_LABEL); err != nil {
			return 0, err
		}
	}

	return int64(pr.GetNumber()), nil
}

func GetUserAndRepo(githubRepo string) (string, string) {
	repoTokens := strings.Split(githubRepo, ":")
	user := strings.Split(repoTokens[1], "/")[0]
	repo := strings.TrimRight(strings.Split(repoTokens[1], "/")[1], ".git")
	return user, repo
}
