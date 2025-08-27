package deepvalidation

import (
	"context"
	"net/http"
	"os"
	"path"
	"strings"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// DeepValidate performs more in-depth validation of the config file,
// including checks requiring HTTP requests and possibly authentication to
// determine, for example, whether the configured repos, CIPD packages, etc,
// actually exist. This builds on the simple Validate() methods of the config
// structs.
func DeepValidate(ctx context.Context, client, githubHttpClient *http.Client, c *config.Config) error {
	cbc := chrome_branch.NewClient(client)
	reg, err := config_vars.NewRegistry(ctx, cbc)
	if err != nil {
		return skerr.Wrap(err)
	}
	dv := &deepvalidator{
		client:           client,
		reg:              reg,
		githubHttpClient: githubHttpClient,
	}
	return dv.deepValidate(ctx, c)
}

// deepvalidator is a helper for running deep validation which wraps up shared
// elements needed by most validation functions.
type deepvalidator struct {
	client           *http.Client
	reg              *config_vars.Registry
	githubHttpClient *http.Client
}

// deepValidate performs deep validation of the Config, making external
// network requests as needed.
func (dv *deepvalidator) deepValidate(ctx context.Context, c *config.Config) error {
	if c.GetGerrit() != nil {
		if err := dv.gerritConfig(ctx, c.GetGerrit()); err != nil {
			return skerr.Wrap(err)
		}
	}
	if c.GetGithub() != nil {
		if err := dv.gitHubConfig(ctx, c.GetGithub()); err != nil {
			return skerr.Wrap(err)
		}
	}
	var err error
	switch rm := c.RepoManager.(type) {
	case *config.Config_AndroidRepoManager:
		_, _, err = dv.androidRepoManagerConfig(ctx, rm.AndroidRepoManager)
	case *config.Config_CommandRepoManager:
		err = dv.commandRepoManagerConfig(ctx, rm.CommandRepoManager)
	case *config.Config_FreetypeRepoManager:
		_, _, err = dv.freeTypeRepoManagerConfig(ctx, rm.FreetypeRepoManager)
	case *config.Config_Google3RepoManager:
		err = dv.google3RepoManagerConfig(ctx, rm.Google3RepoManager)
	}
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// gerritConfig performs validation of the GerritConfig, making
// external network requests as needed.
func (dv *deepvalidator) gerritConfig(ctx context.Context, c *config.GerritConfig) error {
	cfg, ok := codereview.GerritConfigs[c.Config]
	if !ok {
		return skerr.Fmt("failed deep-validating GerritConfig: unknown config: %v", c.Config)
	}
	g, err := gerrit.NewGerritWithConfig(cfg, c.Url, dv.client)
	if err != nil {
		return skerr.Wrapf(err, "failed deep-validating GerritConfig")
	}
	results, err := g.Search(ctx, 1, false, gerrit.SearchProject(c.Project))
	if err != nil {
		return skerr.Wrapf(err, "failed deep-validating GerritConfig")
	}
	if len(results) == 0 {
		return skerr.Fmt("failed deep-validating GerritConfig: no changes found in project %q", c.Project)
	}
	return nil
}

// gitHubConfig performs validation of the GitHubConfig, making
// external network requests as needed.
func (dv *deepvalidator) gitHubConfig(ctx context.Context, c *config.GitHubConfig) error {
	gh, err := github.NewGitHub(ctx, c.RepoOwner, c.RepoName, dv.githubHttpClient)
	if err != nil {
		return skerr.Wrapf(err, "failed deep-validating GitHubConfig for %s/%s", c.RepoOwner, c.RepoName)
	}
	// Just perform an arbitrary read request which uses the configured owner
	// and repo name.
	if _, err := gh.ListOpenPullRequests(); err != nil {
		return skerr.Wrapf(err, "failed deep-validating GitHubConfig for %s/%s", c.RepoOwner, c.RepoName)
	}

	return nil
}

// versionFileConfig performs validation of the VersionFileConfig,
// making external network requests as needed.
func (dv *deepvalidator) versionFileConfig(ctx context.Context, c *config.VersionFileConfig, getFile version_file_common.GetFileFunc) error {
	if getFile == nil {
		return skerr.Fmt("failed to deep-validate VersionFileConfig: have no way to retrieve files!")
	}
	deps, err := version_file_common.GetPinnedRevs(ctx, []*config.VersionFileConfig{c}, getFile)
	if err != nil {
		return skerr.Wrapf(err, "failed to deep-validate VersionFileConfig")
	}
	if _, ok := deps[c.Id]; !ok {
		return skerr.Wrapf(err, "failed to deep-validate VersionFileConfig: dependency %q does not exist in %s", c.Id, c.File[0].Path)
	}
	return nil
}

func makeGitilesGetFileFunc(repo gitiles.GitilesRepo, branch string) version_file_common.GetFileFunc {
	return func(ctx context.Context, path string) (string, error) {
		b, err := repo.ReadFileAtRef(ctx, path, branch)
		return string(b), err
	}
}

func (dv *deepvalidator) makeGitilesGetFileFuncFromConfig(c *config.GitilesConfig) (version_file_common.GetFileFunc, error) {
	repo := gitiles.NewRepo(c.RepoUrl, dv.client)
	branchTmpl, err := config_vars.NewTemplate(c.Branch)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := dv.reg.Register(branchTmpl); err != nil {
		return nil, skerr.Wrap(err)
	}
	branch := branchTmpl.String()
	return makeGitilesGetFileFunc(repo, branch), nil
}

// deepValidateGitilesRepo performs deep validation of a Gitiles repo.
func (dv *deepvalidator) deepValidateGitilesRepo(ctx context.Context, repoUrl string, branchTmpl *config_vars.Template) (version_file_common.GetFileFunc, *revision.Revision, error) {
	repo := gitiles.NewRepo(repoUrl, dv.client)
	branch := branchTmpl.String()
	details, err := repo.Details(ctx, branch)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "failed to resolve branch %q of repo %q", branch, repoUrl)
	}
	tipRev := revision.FromLongCommit("", "", details)
	return makeGitilesGetFileFunc(repo, tipRev.Id), tipRev, nil
}

// deepValidateGitHubRepo performs deep validation of a GitHub repo.
func (dv *deepvalidator) deepValidateGitHubRepo(ctx context.Context, repoUrl string, branchTmpl *config_vars.Template) (version_file_common.GetFileFunc, *revision.Revision, error) {
	repoOwner, repoName, err := github.ParseRepoOwnerAndName(repoUrl)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	gh, err := github.NewGitHub(ctx, repoOwner, repoName, dv.githubHttpClient)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	branch := branchTmpl.String()
	ref, err := gh.GetReference(repoOwner, repoName, git_common.RefsHeadsPrefix+branch)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "failed GetReference for %s/%s @ %s", repoOwner, repoName, branch)
	}
	commit, err := gh.GetCommit(*ref.Object.SHA)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "failed GetCommit for %s/%s @ %s", repoOwner, repoName, *ref.Object.SHA)
	}
	tipRev := revision.FromLongCommit("", "", github.CommitToLongCommit(commit))
	return func(ctx context.Context, path string) (string, error) {
		return gh.ReadRawFile(*ref.Object.SHA, path)
	}, tipRev, nil
}

// deepValidateGitRepo performs deep validation of an arbitrary Git repo.
func (dv *deepvalidator) deepValidateGitRepo(ctx context.Context, repoUrl, branch string) (version_file_common.GetFileFunc, *revision.Revision, error) {
	branchTmpl, err := config_vars.NewTemplate(branch)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	if err := dv.reg.Register(branchTmpl); err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	if strings.Contains(repoUrl, "googlesource") {
		return dv.deepValidateGitilesRepo(ctx, repoUrl, branchTmpl)
	} else if strings.Contains(repoUrl, "github") {
		return dv.deepValidateGitHubRepo(ctx, repoUrl, branchTmpl)
	} else {
		return nil, nil, skerr.Fmt("unknown git repo source for %q", repoUrl)
	}
}

// google3RepoManagerConfig performs validation of the
// Google3RepoManagerConfig, making external network requests as needed.
func (dv *deepvalidator) google3RepoManagerConfig(ctx context.Context, c *config.Google3RepoManagerConfig) error {
	if _, _, err := dv.deepValidateGitRepo(ctx, c.ChildRepo, c.ChildBranch); err != nil {
		return skerr.Wrapf(err, "failed deep-validating Google3RepoManagerConfig")
	}
	return nil
}

// androidRepoManagerConfig performs validation of the
// AndroidRepoManagerConfig, making external network requests as needed.
func (dv *deepvalidator) androidRepoManagerConfig(ctx context.Context, c *config.AndroidRepoManagerConfig) (version_file_common.GetFileFunc, version_file_common.GetFileFunc, error) {
	getFileParent, _, err := dv.deepValidateGitRepo(ctx, c.ParentRepoUrl, c.ParentBranch)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	getFileChild, _, err := dv.deepValidateGitRepo(ctx, c.ChildRepoUrl, c.ChildBranch)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	return getFileParent, getFileChild, nil
}

// commandRepoManagerConfig performs validation of the
// CommandRepoManagerConfig, making external network requests as needed.
func (dv *deepvalidator) commandRepoManagerConfig(ctx context.Context, c *config.CommandRepoManagerConfig) error {
	getFile, _, err := dv.gitCheckoutConfig(ctx, c.GitCheckout)
	if err != nil {
		return skerr.Wrap(err)
	}
	tipRev := &revision.Revision{
		Id: "fake",
	}
	if err := dv.commandRepoManagerConfig_CommandConfig(ctx, c.GetTipRev, tipRev, getFile); err != nil {
		return skerr.Wrap(err)
	}
	if err := dv.commandRepoManagerConfig_CommandConfig(ctx, c.GetPinnedRev, tipRev, getFile); err != nil {
		return skerr.Wrap(err)
	}
	if err := dv.commandRepoManagerConfig_CommandConfig(ctx, c.SetPinnedRev, tipRev, getFile); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func fakePlaceholders(tipRev *revision.Revision) parent.Placeholders {
	return parent.Placeholders{
		CIPDRoot:      "/fake/cipd/root",
		ParentRepoDir: "/fake/parent/repo/dir",
		PathVar:       os.Getenv("PATH"),
		RollingFromID: "rolling-from",
		RollingToID:   tipRev.Id,
	}
}

func deepValidateCommand(ctx context.Context, cmd []string, cwd string, env []string, tipRev *revision.Revision, getFile version_file_common.GetFileFunc) error {
	ctx = exec.WithOverrideExecutableExists(ctx, func(fp string) bool {
		if !path.IsAbs(fp) {
			// We're looking in the checkout dir.
			fp = path.Join(cwd, fp)
			_, err := getFile(ctx, fp)
			if err != nil {
				sklog.Errorf("failed to retrieve file %q: %s", fp, err)
			}
			return err == nil
		}
		// We're looking elsewhere. Fall back to the default.
		return exec.DefaultExecutableExists(fp) == nil
	})
	_, err := fakePlaceholders(tipRev).Command(ctx, cmd, cwd, env)
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// commandRepoManagerConfig_CommandConfig performs validation of the
// CommandRepoManagerConfig_CommandConfig, making external network requests as
// needed.
func (dv *deepvalidator) commandRepoManagerConfig_CommandConfig(ctx context.Context, c *config.CommandRepoManagerConfig_CommandConfig, tipRev *revision.Revision, getFile version_file_common.GetFileFunc) error {
	return deepValidateCommand(ctx, c.Command, c.Dir, c.Env, tipRev, getFile)
}

// freeTypeRepoManagerConfig performs validation of the
// FreeTypeRepoManagerConfig, making external network requests as needed.
func (dv *deepvalidator) freeTypeRepoManagerConfig(ctx context.Context, c *config.FreeTypeRepoManagerConfig) (version_file_common.GetFileFunc, version_file_common.GetFileFunc, error) {
	getFileChild, _, err := dv.gitilesChildConfig(ctx, c.Child)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	getFileParent, err := dv.freeTypeParentConfig(ctx, c.Parent, getFileChild)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	return getFileParent, getFileChild, nil
}

// gitCheckoutConfig performs validation of the GitCheckoutConfig,
// making external network requests as needed.
func (dv *deepvalidator) gitCheckoutConfig(ctx context.Context, c *config.GitCheckoutConfig) (version_file_common.GetFileFunc, *revision.Revision, error) {
	getFile, tipRev, err := dv.deepValidateGitRepo(ctx, c.RepoUrl, c.Branch)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	for _, dep := range c.Dependencies {
		if err := dv.versionFileConfig(ctx, dep, getFile); err != nil {
			return nil, nil, skerr.Wrap(err)
		}
	}
	return getFile, tipRev, nil
}

// gitilesChildConfig performs validation of the GitilesChildConfig,
// making external network requests as needed.
func (dv *deepvalidator) gitilesChildConfig(ctx context.Context, c *config.GitilesChildConfig) (version_file_common.GetFileFunc, *revision.Revision, error) {
	child, err := child.NewGitiles(ctx, c, dv.reg, dv.client)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	tipRev, err := child.GetTipRevision(ctx)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	if _, _, err := child.Update(ctx, tipRev); err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	getFile, err := dv.makeGitilesGetFileFuncFromConfig(c.Gitiles)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	return getFile, tipRev, nil
}

// freeTypeParentConfig performs validation of the
// FreeTypeParentConfig, making external network requests as needed.
func (dv *deepvalidator) freeTypeParentConfig(ctx context.Context, c *config.FreeTypeParentConfig, getFileChild version_file_common.GetFileFunc) (version_file_common.GetFileFunc, error) {
	// FreeType parent creates a local child repo, but we can get the
	// information we need by simply validating the Gitiles part of it.
	return dv.gitilesParentConfig(ctx, c.Gitiles, getFileChild)
}

// gitilesParentConfig performs validation of the GitilesParentConfig,
// making external network requests as needed.
func (dv *deepvalidator) gitilesParentConfig(ctx context.Context, c *config.GitilesParentConfig, getFileChild version_file_common.GetFileFunc) (version_file_common.GetFileFunc, error) {
	p, err := parent.NewGitilesFile(ctx, c, dv.reg, dv.client, "")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if _, err := p.Update(ctx); err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := dv.gerritConfig(ctx, c.Gerrit); err != nil {
		return nil, skerr.Wrap(err)
	}
	getFileParent, err := dv.makeGitilesGetFileFuncFromConfig(c.Gitiles)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := dv.dependencyConfig(ctx, c.Dep, getFileParent, getFileChild); err != nil {
		return nil, skerr.Wrap(err)
	}
	return getFileParent, nil
}

// dependencyConfig performs validation of the DependencyConfig, making
// external network requests as needed.
func (dv *deepvalidator) dependencyConfig(ctx context.Context, c *config.DependencyConfig, getFileParent, getFileChild version_file_common.GetFileFunc) error {
	if err := dv.versionFileConfig(ctx, c.Primary, getFileParent); err != nil {
		return skerr.Wrap(err)
	}
	for _, findAndReplaceFile := range c.FindAndReplace {
		if _, err := getFileParent(ctx, findAndReplaceFile); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}
