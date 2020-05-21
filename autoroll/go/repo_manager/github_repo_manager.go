package repo_manager

import (
	"context"
	"net/http"
	"os"
	"path"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/child/revision_filter"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
)

// GithubRepoManagerConfig provides configuration for the Github RepoManager.
type GithubRepoManagerConfig struct {
	CommonRepoManagerConfig
	Github        *codereview.GithubConfig `json:"gerrit,omitempty"`
	ChildRepoName string                   `json:"childRepoName"`
	ChildRepoURL  string                   `json:"childRepoURL"`
	ChildUserName string                   `json:"childUserName"`
	ForkRepoURL   string                   `json:"forkRepoURL"`
	// The roller will update this file with the child repo's revision.
	RevisionFile string `json:"revisionFile"`

	BuildbucketRevisionFilter *revision_filter.BuildbucketRevisionFilterConfig `json:"buildbucketFilter"`

	// TransitiveDeps is an optional mapping of dependency ID (eg. repo URL)
	// to the paths within the parent and child repo, respectively, where
	// those dependencies are versioned, eg. "DEPS".
	TransitiveDeps []*version_file_common.TransitiveDepConfig `json:"transitiveDeps,omitempty"`
}

// See documentation for util.Validator interface.
func (c *GithubRepoManagerConfig) Validate() error {
	_, _, err := c.splitParentChild()
	return skerr.Wrap(err)
}

// splitParentChild splits the GithubRepoManagerConfig into a
// parent.GitCheckoutGithubConfig and a child.GitCheckoutConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c GithubRepoManagerConfig) splitParentChild() (parent.GitCheckoutGithubFileConfig, child.GitCheckoutGithubConfig, error) {
	var childDeps []*version_file_common.VersionFileConfig
	if c.TransitiveDeps != nil {
		childDeps = make([]*version_file_common.VersionFileConfig, 0, len(c.TransitiveDeps))
		for _, dep := range c.TransitiveDeps {
			childDeps = append(childDeps, dep.Child)
		}
	}
	parentCfg := parent.GitCheckoutGithubFileConfig{
		GitCheckoutGithubConfig: parent.GitCheckoutGithubConfig{
			GitCheckoutConfig: parent.GitCheckoutConfig{
				GitCheckoutConfig: git_common.GitCheckoutConfig{
					Branch:      c.ParentBranch,
					RepoURL:     c.ParentRepo,
					RevLinkTmpl: "", // Not needed.
				},
				DependencyConfig: version_file_common.DependencyConfig{
					VersionFileConfig: version_file_common.VersionFileConfig{
						ID:   c.ChildRepoURL,
						Path: c.RevisionFile,
					},
					TransitiveDeps: c.TransitiveDeps,
				},
			},
			ForkRepoURL: c.ForkRepoURL,
		},
		PreUploadSteps: c.PreUploadSteps,
	}
	if err := parentCfg.Validate(); err != nil {
		return parent.GitCheckoutGithubFileConfig{}, child.GitCheckoutGithubConfig{}, skerr.Wrapf(err, "generated parent config is invalid")
	}
	childCfg := child.GitCheckoutGithubConfig{
		GitCheckoutConfig: child.GitCheckoutConfig{
			GitCheckoutConfig: git_common.GitCheckoutConfig{
				Branch:       c.ChildBranch,
				Dependencies: childDeps,
				RepoURL:      c.ChildRepoURL,
				RevLinkTmpl:  c.ChildRevLinkTmpl,
			},
		},
		BuildbucketRevisionFilter: c.BuildbucketRevisionFilter,
		GithubRepoName:            c.ChildRepoName,
		GithubUserName:            c.ChildUserName,
	}
	if err := childCfg.Validate(); err != nil {
		return parent.GitCheckoutGithubFileConfig{}, child.GitCheckoutGithubConfig{}, skerr.Wrapf(err, "generated child config is invalid")
	}
	return parentCfg, childCfg, nil
}

// NewGithubRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewGithubRepoManager(ctx context.Context, c *GithubRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, githubClient *github.GitHub, recipeCfgFile, serverURL string, client *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	wd := path.Join(workdir, "github_repos")
	if _, err := os.Stat(wd); err != nil {
		if err := os.MkdirAll(wd, 0755); err != nil {
			return nil, skerr.Wrap(err)
		}
	}

	parentCfg, childCfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg.ForkBranchName = rollerName
	parentRM, err := parent.NewGitCheckoutGithubFile(ctx, parentCfg, reg, client, githubClient, serverURL, wd, cr.UserName(), cr.UserEmail(), nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewGitCheckoutGithub(ctx, childCfg, reg, client, wd, cr.UserName(), cr.UserEmail(), nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM)
}
