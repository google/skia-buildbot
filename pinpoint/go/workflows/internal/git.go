// Activities to access Chromium git repositories.

package internal

import (
	"context"
	"fmt"
	"os"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/common"
	pb "go.skia.org/infra/pinpoint/proto/v1"

	"golang.org/x/oauth2/google"
)

const (
	REPO_CHROMIUM   = "https://chromium.googlesource.com/chromium/src.git"
	GERRIT_CHROMIUM = "https://chromium-review.googlesource.com"
)

// gitClient info.
type gitClient struct {
	ctx          context.Context
	repo         *gitiles.Repo
	repoUrl      string
	gerritClient *gerrit.Gerrit
	gitExec      string
	repoDir      string
}

func NewGitChromium(ctx context.Context) (*gitClient, error) {
	return NewGitClient(ctx, REPO_CHROMIUM, GERRIT_CHROMIUM)
}

func NewGitClient(ctx context.Context, repoUrl string, gerritUrl string) (*gitClient, error) {
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	repo := gitiles.NewRepo(repoUrl, httpClient)
	gerritClient, err := gerrit.NewGerrit(gerritUrl, httpClient)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	git, err := git.Executable(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &gitClient{
		ctx:          ctx,
		repo:         repo,
		repoUrl:      repoUrl,
		gerritClient: gerritClient,
		gitExec:      git,
	}, nil
}

// ReadGitFileActivity is an Activity that reads the contents of a file from a git commit.
func ReadGitFileActivity(ctx context.Context, combinedCommit *common.CombinedCommit, path string) ([]byte, error) {
	sklog.Info("ReadGitFileActivity started")
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "problem setting up default token source")
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).Client()

	var commit *pb.Commit
	if len(combinedCommit.ModifiedDeps) > 0 {
		commit = combinedCommit.ModifiedDeps[len(combinedCommit.ModifiedDeps)-1]
	} else {
		commit = combinedCommit.Main
	}

	repo := gitiles.NewRepo(commit.Repository, httpClient)
	return repo.ReadFileAtRef(ctx, path, commit.GitHash)
}

// ShallowClone creates a shallow clone of the given repo at the given commit.
func (client *gitClient) ShallowClone(branchName string, isDev bool) error {
	dirNamePattern := fmt.Sprintf("%s-*", branchName)
	checkoutDir, err := os.MkdirTemp("", dirNamePattern)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create folder: %s", checkoutDir)
	}
	sklog.Infof("Repository temp dir: %s", checkoutDir)
	if _, err := exec.RunCwd(client.ctx, checkoutDir, client.gitExec, "init"); err != nil {
		return skerr.Wrapf(err, "Failed to init Git")
	}
	if !isDev {
		if _, err := exec.RunCwd(client.ctx, checkoutDir, client.gitExec, "config",
			"user.email", "pinpoint-worker@skia-infra-corp.iam.gserviceaccount.com"); err != nil {
			return skerr.Wrapf(err, "Failed to config user email")
		}
		if _, err := exec.RunCwd(client.ctx, checkoutDir, client.gitExec, "config", "user.name", "Pinpoint Worker"); err != nil {
			return skerr.Wrapf(err, "Failed to config user name")
		}
	}
	if _, err := exec.RunCwd(client.ctx, checkoutDir, client.gitExec, "remote", "add", "origin", client.repoUrl); err != nil {
		return skerr.Wrapf(err, "Failed to add remote Git")
	}
	sklog.Info("Git fetch --depth=1 origin/main")
	if _, err := exec.RunCwd(client.ctx, checkoutDir, client.gitExec, "fetch", "--depth=1", "origin", "main"); err != nil {
		return skerr.Wrapf(err, "Failed to fetch origin/main")
	}
	sklog.Info("Git checkout FETCH_HEAD")
	if _, err := exec.RunCwd(client.ctx, checkoutDir, client.gitExec, "checkout", "FETCH_HEAD"); err != nil {
		return skerr.Wrapf(err, "Failed to checkout FETCH_HEAD")
	}
	if _, err := exec.RunCwd(
		client.ctx, checkoutDir, client.gitExec, "checkout", "-b",
		branchName, "-t", "origin/main"); err != nil {
		return skerr.Wrapf(err, "Failed to create a new branch")
	}

	client.repoDir = checkoutDir
	sklog.Infof("Git clone of %s to %s was successful!", client.repoUrl, checkoutDir)
	return nil
}
