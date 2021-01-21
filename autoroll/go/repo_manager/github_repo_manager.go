package repo_manager

import (
	"context"
	"net/http"
	"os"
	"path"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
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
	ChildRepoName string `json:"childRepoName"`
	ChildRepoURL  string `json:"childRepoURL"`
	ChildUserName string `json:"childUserName"`
	ForkRepoURL   string `json:"forkRepoURL"`

	// The roller will update this file with the child repo's revision.
	RevisionFile string `json:"revisionFile"`

	BuildbucketRevisionFilter *revision_filter.BuildbucketRevisionFilterConfig `json:"buildbucketFilter,omitempty"`

	// TransitiveDeps is an optional mapping of dependency ID (eg. repo URL)
	// to the paths within the parent and child repo, respectively, where
	// those dependencies are versioned, eg. "DEPS".
	TransitiveDeps []*version_file_common.TransitiveDepConfig `json:"transitiveDeps,omitempty"`
}

// Validate implements util.Validator.
func (c *GithubRepoManagerConfig) Validate() error {
	// Set some unused variables on the embedded RepoManager.
	c.ChildPath = "N/A"
	c.ChildSubdir = "N/A"
	if err := c.CommonRepoManagerConfig.Validate(); err != nil {
		return err
	}
	// Unset the unused variables.
	c.ChildPath = ""
	c.ChildSubdir = ""

	if c.BuildbucketRevisionFilter != nil {
		if err := c.BuildbucketRevisionFilter.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
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
					Branch:  c.ParentBranch,
					RepoURL: c.ParentRepo,
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
		GithubRepoName: c.ChildRepoName,
		GithubUserName: c.ChildUserName,
	}
	if err := childCfg.Validate(); err != nil {
		return parent.GitCheckoutGithubFileConfig{}, child.GitCheckoutGithubConfig{}, skerr.Wrapf(err, "generated child config is invalid")
	}
	return parentCfg, childCfg, nil
}

// GithubRepoManagerConfigToProto converts a GithubRepoManagerConfig to a
// config.ParentChildRepoManager.
func GithubRepoManagerConfigToProto(cfg *GithubRepoManagerConfig) (*config.ParentChildRepoManagerConfig, error) {
	parentCfg, childCfg, err := cfg.splitParentChild()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv := &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_GitCheckoutGithubFileParent{
			GitCheckoutGithubFileParent: parent.GitCheckoutGithubFileConfigToProto(&parentCfg),
		},
		Child: &config.ParentChildRepoManagerConfig_GitCheckoutGithubChild{
			GitCheckoutGithubChild: child.GitCheckoutGithubConfigToProto(&childCfg),
		},
	}
	if cfg.BuildbucketRevisionFilter != nil {
		rv.RevisionFilter = &config.ParentChildRepoManagerConfig_BuildbucketRevisionFilter{
			BuildbucketRevisionFilter: revision_filter.BuildBucketRevisionFilterConfigToProto(cfg.BuildbucketRevisionFilter),
		}
	}
	return rv, nil
}

// ProtoToGithubRepoManagerConfig converts a config.ParentChildRepoManager to a
// GithubRepoManagerConfig.
func ProtoToGithubRepoManagerConfig(cfg *config.ParentChildRepoManagerConfig) (*GithubRepoManagerConfig, error) {
	childCfg := cfg.GetGitCheckoutGithubChild()
	childBranch, err := config_vars.NewTemplate(childCfg.GitCheckout.GitCheckout.Branch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	parentCfg := cfg.GetGitCheckoutGithubFileParent()
	parentBranch, err := config_vars.NewTemplate(parentCfg.GitCheckout.GitCheckout.GitCheckout.Branch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv := &GithubRepoManagerConfig{
		CommonRepoManagerConfig: CommonRepoManagerConfig{
			ChildBranch:      childBranch,
			ParentBranch:     parentBranch,
			ParentRepo:       parentCfg.GitCheckout.GitCheckout.GitCheckout.RepoUrl,
			ChildRevLinkTmpl: childCfg.GitCheckout.GitCheckout.RevLinkTmpl,
			PreUploadSteps:   parent.ProtoToPreUploadSteps(parentCfg.PreUploadSteps),
		},
		ChildRepoName:  childCfg.RepoName,
		ChildRepoURL:   childCfg.GitCheckout.GitCheckout.RepoUrl,
		ChildUserName:  childCfg.RepoOwner,
		ForkRepoURL:    parentCfg.GitCheckout.ForkRepoUrl,
		RevisionFile:   parentCfg.GitCheckout.GitCheckout.Dep.Primary.Path,
		TransitiveDeps: version_file_common.ProtoToTransitiveDepConfigs(parentCfg.GitCheckout.GitCheckout.Dep.Transitive),
	}
	if cfg.RevisionFilter != nil {
		if f, ok := cfg.RevisionFilter.(*config.ParentChildRepoManagerConfig_BuildbucketRevisionFilter); ok {
			rv.BuildbucketRevisionFilter = revision_filter.ProtoToBuildbucketRevisionFilterConfig(f.BuildbucketRevisionFilter)
		}
	}
	return rv, nil
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
	parentRM, err := parent.NewGitCheckoutGithubFile(ctx, parentCfg, reg, client, githubClient, serverURL, wd, cr.UserName(), cr.UserEmail(), rollerName, nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewGitCheckoutGithub(ctx, childCfg, reg, client, wd, cr.UserName(), cr.UserEmail(), nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var rf revision_filter.RevisionFilter
	if c.BuildbucketRevisionFilter != nil {
		rf, err = revision_filter.NewBuildbucketRevisionFilter(client, c.BuildbucketRevisionFilter.Project, c.BuildbucketRevisionFilter.Bucket)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, rf)
}
