package repo_manager

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/child/revision_filter"
	"go.skia.org/infra/autoroll/go/repo_manager/common/git_common"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/go/gerrit"
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

// Validate implements the util.Validator interface.
func (c *GithubRepoManagerConfig) Validate() error {
	if c.BuildbucketRevisionFilter != nil {
		if err := c.BuildbucketRevisionFilter.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	_, err := c.splitParentChild()
	return skerr.Wrap(err)
}

// splitParentChild splits the GithubRepoManagerConfig into a
// parent.GitCheckoutGithubConfig and a child.GitCheckoutConfig.
// TODO(borenet): Update the config format to directly define the parent
// and child. We shouldn't need most of the New.*RepoManager functions.
func (c GithubRepoManagerConfig) splitParentChild() (*ParentChildConfig, error) {
	var childDeps []*version_file_common.VersionFileConfig
	if c.TransitiveDeps != nil {
		childDeps = make([]*version_file_common.VersionFileConfig, 0, len(c.TransitiveDeps))
		for _, dep := range c.TransitiveDeps {
			childDeps = append(childDeps, dep.Child)
		}
	}
	cfg := &ParentChildConfig{
		Parent: parent.Config{
			GitCheckoutGithubFile: &parent.GitCheckoutGithubFileConfig{
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
			},
		},
		Child: child.Config{
			GitCheckoutGithub: &child.GitCheckoutGithubConfig{
				GitCheckoutConfig: child.GitCheckoutConfig{
					GitCheckoutConfig: git_common.GitCheckoutConfig{
						Branch:       c.ChildBranch,
						Dependencies: childDeps,
						RepoURL:      c.ChildRepoURL,
						RevLinkTmpl:  c.ChildRevLinkTmpl,
					},
				},
				GithubRepoName: c.ChildRepoName,
				GithubUserName: c.ChildUserName,
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		return nil, skerr.Wrapf(err, "generated config is invalid")
	}
	return cfg, nil
}

// NewGithubRepoManager returns a RepoManager instance which operates in the given
// working directory and updates at the given frequency.
func NewGithubRepoManager(ctx context.Context, c *GithubRepoManagerConfig, reg *config_vars.Registry, workdir, rollerName string, gerritClient *gerrit.Gerrit, githubClient *github.GitHub, recipeCfgFile, serverURL string, httpClient *http.Client, cr codereview.CodeReview, local bool) (*ParentChildRepoManager, error) {
	cfg, err := c.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildFromConfig(ctx, cfg, reg, httpClient, gerritClient, githubClient, serverURL, workdir, rollerName, cr.UserName(), cr.UserEmail(), recipeCfgFile)
}
