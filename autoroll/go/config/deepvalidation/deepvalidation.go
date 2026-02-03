package deepvalidation

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/child/revision_filter"
	"go.skia.org/infra/autoroll/go/repo_manager/common/version_file_common"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/docker"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/progress"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog/sklogimpl"
)

// DeepValidate performs more in-depth validation of the config file,
// including checks requiring HTTP requests and possibly authentication to
// determine, for example, whether the configured repos, CIPD packages, etc,
// actually exist. This builds on the simple Validate() methods of the config
// structs.
func DeepValidate(ctx context.Context, client, githubHttpClient *http.Client, c *config.Config) error {
	bbClient := buildbucket.NewClient(client)
	cipdClient, err := cipd.NewClient(ctx, "", cipd.DefaultServiceURL)
	if err != nil {
		return skerr.Wrap(err)
	}
	dockerClient, err := docker.NewClient(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	dv := &deepvalidator{
		bbClient:         bbClient,
		client:           client,
		cipdClient:       cipdClient,
		dockerClient:     dockerClient,
		githubHttpClient: githubHttpClient,
	}

	ch := make(chan error)
	go func() {
		ch <- dv.deepValidate(ctx, c)
	}()

	tick := time.Tick(30 * time.Second)
	for {
		select {
		case err := <-ch:
			return err
		case <-tick:
			fmt.Printf("Still validating %s\n", c.RollerName)
		}
	}
}

// numMultiValidateWorkers is the size of the worker pool used in
// DeepValidateMulti.
const numMultiValidateWorkers = 10

// DeepValidateMulti validates multiple config files.
func DeepValidateMulti(ctx context.Context, client, githubHttpClient *http.Client, configs map[string]*config.Config) error {
	// Override logging to reduce noise from the packages we use.
	unSuppressLogs := sklogimpl.SuppressLogs()
	defer unSuppressLogs()

	type configToValidate struct {
		filename string
		config   *config.Config
	}
	configsToValidate := make(chan *configToValidate)

	go func() {
		for file, cfg := range configs {
			configsToValidate <- &configToValidate{
				filename: file,
				config:   cfg,
			}
		}
		close(configsToValidate)
	}()

	pt := progress.New(int64(len(configs)))
	pt.AtInterval(ctx, 5*time.Second, func(count, total int64) {
		fmt.Printf("Validated %d of %d\n", count, total)
	})
	var wg sync.WaitGroup
	results := sync.Map{}
	for range numMultiValidateWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cfg := range configsToValidate {
				if err := DeepValidate(ctx, client, githubHttpClient, cfg.config); err != nil {
					results.Store(cfg.filename, err)
				}
				pt.Inc(1)
			}
		}()
	}
	wg.Wait()
	var combinedErr error
	results.Range(func(file, err any) bool {
		combinedErr = errors.Append(combinedErr, skerr.Fmt("Deep validation failed for %s: %s", file, err))
		return true
	})
	return combinedErr
}

// deepvalidator is a helper for running deep validation which wraps up shared
// elements needed by most validation functions.
type deepvalidator struct {
	bbClient         buildbucket.BuildBucketInterface
	client           *http.Client
	cipdClient       cipd.CIPDClient
	dockerClient     docker.Client
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
	for _, reviewer := range c.Reviewer {
		if err := dv.reviewer(ctx, reviewer); err != nil {
			return skerr.Wrap(err)
		}
	}

	// CommitMsg should never be nil, but that case would be detected by normal
	// validation. We allow it to be nil here to simplify testing.
	if c.CommitMsg != nil {
		if err := dv.commitMsg(ctx, c.CommitMsg); err != nil {
			return skerr.Wrap(err)
		}
	}

	var getFileParent, getFileChild version_file_common.GetFileFunc
	// RepoManager should never be nil, but that case would be detected by
	// normal validation. We allow it to be nil here to simplify testing.
	if c.RepoManager != nil {
		var err error
		switch rm := c.RepoManager.(type) {
		case *config.Config_AndroidRepoManager:
			getFileParent, getFileChild, err = dv.androidRepoManagerConfig(ctx, rm.AndroidRepoManager)
		case *config.Config_CommandRepoManager:
			err = dv.commandRepoManagerConfig(ctx, rm.CommandRepoManager)
		case *config.Config_FreetypeRepoManager:
			getFileParent, getFileChild, err = dv.freeTypeRepoManagerConfig(ctx, rm.FreetypeRepoManager)
		case *config.Config_Google3RepoManager:
			err = dv.google3RepoManagerConfig(ctx, rm.Google3RepoManager)
		case *config.Config_ParentChildRepoManager:
			getFileParent, getFileChild, err = dv.parentChildRepoManagerConfig(ctx, rm.ParentChildRepoManager)
		default:
			return skerr.Fmt("Unknown repo manager type: %s", rm)
		}
		if err != nil {
			return skerr.Wrap(err)
		}
	}
	for _, td := range c.TransitiveDeps {
		if err := dv.transitiveDepConfig(ctx, td, getFileParent, getFileChild); err != nil {
			return skerr.Wrap(err)
		}
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

// transitiveDepConfig performs validation of the TransitiveDepConfig,
// making external network requests as needed.
func (dv *deepvalidator) transitiveDepConfig(ctx context.Context, c *config.TransitiveDepConfig, getFileParent, getFileChild version_file_common.GetFileFunc) error {
	if err := dv.versionFileConfig(ctx, c.Parent, getFileParent); err != nil {
		return skerr.Wrap(err)
	}
	if err := dv.versionFileConfig(ctx, c.Child, getFileChild); err != nil {
		return skerr.Wrap(err)
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
	return makeGitilesGetFileFunc(repo, c.Branch), nil
}

// deepValidateGitilesRepo performs deep validation of a Gitiles repo.
func (dv *deepvalidator) deepValidateGitilesRepo(ctx context.Context, repoUrl, branch string) (version_file_common.GetFileFunc, *revision.Revision, error) {
	repo := gitiles.NewRepo(repoUrl, dv.client)
	details, err := repo.Details(ctx, branch)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "failed to resolve branch %q of repo %q", branch, repoUrl)
	}
	tipRev := revision.FromLongCommit("", "", details)
	return makeGitilesGetFileFunc(repo, tipRev.Id), tipRev, nil
}

// deepValidateGitHubRepo performs deep validation of a GitHub repo.
func (dv *deepvalidator) deepValidateGitHubRepo(ctx context.Context, repoUrl, branch string) (version_file_common.GetFileFunc, *revision.Revision, error) {
	repoOwner, repoName, err := github.ParseRepoOwnerAndName(repoUrl)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	gh, err := github.NewGitHub(ctx, repoOwner, repoName, dv.githubHttpClient)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
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
	if strings.Contains(repoUrl, "googlesource") {
		return dv.deepValidateGitilesRepo(ctx, repoUrl, branch)
	} else if strings.Contains(repoUrl, "github") {
		return dv.deepValidateGitHubRepo(ctx, repoUrl, branch)
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
	getFileChild, tipRev, err := dv.deepValidateGitRepo(ctx, c.ChildRepoUrl, c.ChildBranch)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	if c.PreUploadCommands != nil {
		if err := dv.preUploadConfig(ctx, c.PreUploadCommands, tipRev, getFileParent); err != nil {
			return nil, nil, skerr.Wrap(err)
		}
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
	tipRev := makeFakeRevision()
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
	placeholders := fakePlaceholders(tipRev)
	ctx = exec.WithOverrideExecutableExists(ctx, func(fp string) bool {
		if path.Base(fp) == fp {
			// We're expecting to find the executable in PATH. If any
			// subdirectory of the CIPD root is in PATH, we can't confirm
			// whether the executable exists without downloading the packages,
			// so we have to assume it'll be present when the roller actually
			// runs.
			var pathVar string
			for _, envVar := range env {
				split := strings.Split(envVar, "=")
				if len(split) == 2 && split[0] == "PATH" {
					pathVar = split[1]
					break
				}
			}
			for _, entry := range strings.Split(pathVar, string(os.PathSeparator)) {
				if strings.HasPrefix(entry, placeholders.CIPDRoot) {
					return true
				}
			}
		}
		if !path.IsAbs(fp) {
			// We're looking in the checkout dir.
			fp = path.Join(cwd, fp)
			_, err := getFile(ctx, fp)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to retrieve file %q: %s\n", fp, err)
			}
			return err == nil
		}
		if strings.HasPrefix(fp, placeholders.CIPDRoot+"/") {
			// The command is an absolute path within a CIPD package. We can't
			// confirm whether the executable exists without downloading the
			// packages, so we have to assume it'll be present when the roller
			// actually runs.
			return true
		}
		// Fall back to the default.
		return exec.DefaultExecutableExists(fp) == nil
	})
	_, err := placeholders.Command(ctx, cmd, cwd, env)
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

// parentChildRepoManagerConfig performs validation of the
// ParentChildRepoManagerConfig, making external network requests as needed.
func (dv *deepvalidator) parentChildRepoManagerConfig(ctx context.Context, c *config.ParentChildRepoManagerConfig) (version_file_common.GetFileFunc, version_file_common.GetFileFunc, error) {
	var getFileParent, getFileChild version_file_common.GetFileFunc
	var tipRev *revision.Revision
	var err error
	switch child := c.Child.(type) {
	case *config.ParentChildRepoManagerConfig_CipdChild:
		tipRev, err = dv.cipdChildConfig(ctx, child.CipdChild)
	case *config.ParentChildRepoManagerConfig_FuchsiaSdkChild:
		tipRev, err = dv.fuchsiaSDKChildConfig(ctx, child.FuchsiaSdkChild)
	case *config.ParentChildRepoManagerConfig_GitCheckoutChild:
		getFileChild, tipRev, err = dv.gitCheckoutChildConfig(ctx, child.GitCheckoutChild)
	case *config.ParentChildRepoManagerConfig_GitCheckoutGithubChild:
		getFileChild, tipRev, err = dv.gitCheckoutGitHubChildConfig(ctx, child.GitCheckoutGithubChild)
	case *config.ParentChildRepoManagerConfig_GitilesChild:
		getFileChild, tipRev, err = dv.gitilesChildConfig(ctx, child.GitilesChild)
	case *config.ParentChildRepoManagerConfig_SemverGcsChild:
		tipRev, err = dv.semVerGCSChildConfig(ctx, child.SemverGcsChild)
	case *config.ParentChildRepoManagerConfig_DockerChild:
		tipRev, err = dv.dockerChildConfig(ctx, child.DockerChild)
	case *config.ParentChildRepoManagerConfig_GitSemverChild:
		getFileChild, tipRev, err = dv.gitSemVerChildConfig(ctx, child.GitSemverChild)
	default:
		return nil, nil, skerr.Fmt("Unknown child type: %s", child)
	}
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	switch parent := c.Parent.(type) {
	case *config.ParentChildRepoManagerConfig_CopyParent:
		getFileParent, err = dv.copyParentConfig(ctx, parent.CopyParent, getFileChild)
	case *config.ParentChildRepoManagerConfig_DepsLocalGithubParent:
		getFileParent, err = dv.depsLocalGitHubParentConfig(ctx, parent.DepsLocalGithubParent, tipRev, getFileChild)
	case *config.ParentChildRepoManagerConfig_DepsLocalGerritParent:
		getFileParent, err = dv.depsLocalGerritParentConfig(ctx, parent.DepsLocalGerritParent, tipRev, getFileChild)
	case *config.ParentChildRepoManagerConfig_GitCheckoutGithubFileParent:
		getFileParent, err = dv.gitCheckoutGitHubFileParentConfig(ctx, parent.GitCheckoutGithubFileParent, tipRev, getFileChild)
	case *config.ParentChildRepoManagerConfig_GitCheckoutGerritParent:
		getFileParent, err = dv.gitCheckoutGerritParentConfig(ctx, parent.GitCheckoutGerritParent, tipRev, getFileChild)
	case *config.ParentChildRepoManagerConfig_GitilesParent:
		getFileParent, err = dv.gitilesParentConfig(ctx, parent.GitilesParent, getFileChild)
	case *config.ParentChildRepoManagerConfig_GoModGerritParent:
		getFileParent, err = dv.goModGerritParentConfig(ctx, parent.GoModGerritParent, tipRev)
	default:
		return nil, nil, skerr.Fmt("Unknown parent type: %s", parent)
	}
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	for _, rf := range c.GetBuildbucketRevisionFilter() {
		if err := dv.buildbucketRevisionFilterConfig(ctx, rf, tipRev); err != nil {
			return nil, nil, skerr.Wrap(err)
		}
	}
	for _, rf := range c.GetCipdRevisionFilter() {
		if err := dv.cipdRevisionFilterConfig(ctx, rf, tipRev); err != nil {
			return nil, nil, skerr.Wrap(err)
		}
	}
	return getFileParent, getFileChild, nil
}

// gitCheckoutGitHubChildConfig performs validation of the
// GitCheckoutGitHubChildConfig, making external network requests as needed.
func (dv *deepvalidator) gitCheckoutGitHubChildConfig(ctx context.Context, c *config.GitCheckoutGitHubChildConfig) (version_file_common.GetFileFunc, *revision.Revision, error) {
	return dv.gitCheckoutChildConfig(ctx, c.GitCheckout)
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
	child, err := child.NewGitiles(ctx, c, dv.client)
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

// gitSemVerChildConfig performs validation of the GitSemVerChildConfig,
// making external network requests as needed.
func (dv *deepvalidator) gitSemVerChildConfig(ctx context.Context, c *config.GitSemVerChildConfig) (version_file_common.GetFileFunc, *revision.Revision, error) {
	child, err := child.NewGitSemVerChild(ctx, c, dv.client)
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
	p, err := parent.NewGitilesFile(ctx, c, dv.client, "")
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

// goModGerritParentConfig performs validation of the
// GoModGerritParentConfig, making external network requests as needed.
func (dv *deepvalidator) goModGerritParentConfig(ctx context.Context, c *config.GoModGerritParentConfig, tipRev *revision.Revision) (version_file_common.GetFileFunc, error) {
	if err := dv.gerritConfig(ctx, c.Gerrit); err != nil {
		return nil, skerr.Wrap(err)
	}
	return dv.goModParentConfig(ctx, c.GoMod, tipRev)
}

// goModParentConfig performs validation of the GoModParentConfig,
// making external network requests as needed.
func (dv *deepvalidator) goModParentConfig(ctx context.Context, c *config.GoModParentConfig, tipRev *revision.Revision) (version_file_common.GetFileFunc, error) {
	parentGetFile, _, err := dv.gitCheckoutConfig(ctx, c.GitCheckout)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if c.PreUploadCommands != nil {
		if err := dv.preUploadConfig(ctx, c.PreUploadCommands, tipRev, parentGetFile); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return parentGetFile, nil
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
	for _, dep := range c.Transitive {
		if err := dv.transitiveDepConfig(ctx, dep, getFileParent, getFileChild); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// fuchsiaSDKChildConfig performs validation of the
// FuchsiaSDKChildConfig, making external network requests as needed.
func (dv *deepvalidator) fuchsiaSDKChildConfig(ctx context.Context, c *config.FuchsiaSDKChildConfig) (*revision.Revision, error) {
	fuchsiaChild, err := child.NewFuchsiaSDK(ctx, c, dv.client)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create fuchsia sdk child")
	}
	// Create a fake revision to pass into Update. We don't have a real
	// last-rolled revision, but Update requires one.
	fakeRev := makeFakeRevision()
	tipRev, _, err := fuchsiaChild.Update(ctx, fakeRev)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to update fuchsia sdk child")
	}
	return tipRev, nil
}

// cipdChildConfig performs validation of the CIPDChildConfig, making
// external network requests as needed.
func (dv *deepvalidator) cipdChildConfig(ctx context.Context, c *config.CIPDChildConfig) (*revision.Revision, error) {
	cipdChild, err := child.NewCIPD(ctx, c, dv.client, dv.cipdClient, "")
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create cipd child")
	}
	// Create a fake revision to pass into Update. We don't have a real
	// last-rolled revision, but Update requires one.
	fakeRev := makeFakeRevision()
	if c.GitilesRepo != "" || c.SourceRepo != nil {
		repoUrl := c.GitilesRepo
		if c.SourceRepo != nil {
			repoUrl = c.SourceRepo.RepoUrl
		}
		repo := gitiles.NewRepo(repoUrl, dv.client)
		// TODO(borenet): Can we rely on main being present?
		head, err := repo.Details(ctx, git.MainBranch)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		details, err := repo.Details(ctx, head.Parents[0])
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		fakeRev = revision.FromLongCommit("", "", details)
		fakeRev.Id = child.CIPDGitRevisionTag(fakeRev.Id)
	}
	tipRev, _, err := cipdChild.Update(ctx, fakeRev)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return tipRev, nil
}

// gitCheckoutChildConfig performs validation of the
// GitCheckoutChildConfig, making external network requests as needed.
func (dv *deepvalidator) gitCheckoutChildConfig(ctx context.Context, c *config.GitCheckoutChildConfig) (version_file_common.GetFileFunc, *revision.Revision, error) {
	getFile, tipRev, err := dv.gitCheckoutConfig(ctx, c.GitCheckout)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	return getFile, tipRev, nil
}

// semVerGCSChildConfig performs validation of the
// SemVerGCSChildConfig, making external network requests as needed.
func (dv *deepvalidator) semVerGCSChildConfig(ctx context.Context, c *config.SemVerGCSChildConfig) (*revision.Revision, error) {
	semverChild, err := child.NewSemVerGCS(ctx, c, dv.client)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create semver gcs child")
	}
	// Create a fake revision to pass into Update. We don't have a real
	// last-rolled revision, but Update requires one.
	fakeRev := makeFakeRevision()
	tipRev, _, err := semverChild.Update(ctx, fakeRev)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to update semver gcs child")
	}
	if tipRev.Id == fakeRev.Id {
		return nil, skerr.Fmt("found no matching objects in %s/%s", c.Gcs.GcsBucket, c.Gcs.GcsPath)
	}
	return tipRev, nil
}

// copyParentConfig performs validation of the CopyParentConfig, making
// external network requests as needed.
func (dv *deepvalidator) copyParentConfig(ctx context.Context, c *config.CopyParentConfig, getFileChild version_file_common.GetFileFunc) (version_file_common.GetFileFunc, error) {
	p, err := parent.NewCopy(ctx, c, dv.client, "", nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if _, err := p.Update(ctx); err != nil {
		return nil, skerr.Wrap(err)
	}
	// Update() may not check the presence of the copied files; go ahead and
	// do that here.
	getFileParent, err := dv.makeGitilesGetFileFuncFromConfig(c.Gitiles.Gitiles)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	for _, cpy := range c.Copies {
		if err := dv.copyParentConfig_CopyEntry(ctx, cpy, getFileParent, getFileChild); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return getFileParent, nil
}

// copyParentConfig_CopyEntry performs validation of the
// CopyParentConfig_CopyEntry, making external network requests as needed.
func (dv *deepvalidator) copyParentConfig_CopyEntry(ctx context.Context, c *config.CopyParentConfig_CopyEntry, getFileParent, getFileChild version_file_common.GetFileFunc) error {
	if _, err := getFileChild(ctx, c.SrcRelPath); err != nil {
		return skerr.Wrapf(err, "failed to read src %q", c.SrcRelPath)
	}
	if _, err := getFileParent(ctx, c.DstRelPath); err != nil {
		return skerr.Wrapf(err, "failed to read dst %q", c.DstRelPath)
	}
	return nil
}

// depsLocalGitHubParentConfig performs validation of the
// DEPSLocalGitHubParentConfig, making external network requests as needed.
func (dv *deepvalidator) depsLocalGitHubParentConfig(ctx context.Context, c *config.DEPSLocalGitHubParentConfig, tipRev *revision.Revision, getFileChild version_file_common.GetFileFunc) (version_file_common.GetFileFunc, error) {
	if err := dv.gitHubConfig(ctx, c.Github); err != nil {
		return nil, skerr.Wrap(err)
	}
	if _, _, err := dv.deepValidateGitHubRepo(ctx, c.ForkRepoUrl, git.MainBranch); err != nil {
		return nil, skerr.Wrap(err)
	}
	return dv.depsLocalParentConfig(ctx, c.DepsLocal, tipRev, getFileChild)
}

// depsLocalParentConfig performs validation of the
// DEPSLocalParentConfig, making external network requests as needed.
func (dv *deepvalidator) depsLocalParentConfig(ctx context.Context, c *config.DEPSLocalParentConfig, tipRev *revision.Revision, getFileChild version_file_common.GetFileFunc) (version_file_common.GetFileFunc, error) {
	getFileParent, err := dv.gitCheckoutParentConfig(ctx, c.GitCheckout, getFileChild)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if c.PreUploadCommands != nil {
		if err := dv.preUploadConfig(ctx, c.PreUploadCommands, tipRev, getFileParent); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return getFileParent, nil
}

// gitCheckoutParentConfig performs validation of the
// GitCheckoutParentConfig, making external network requests as needed.
func (dv *deepvalidator) gitCheckoutParentConfig(ctx context.Context, c *config.GitCheckoutParentConfig, getFileChild version_file_common.GetFileFunc) (version_file_common.GetFileFunc, error) {
	getFileParent, _, err := dv.gitCheckoutConfig(ctx, c.GitCheckout)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if err := dv.dependencyConfig(ctx, c.Dep, getFileParent, getFileChild); err != nil {
		return nil, skerr.Wrap(err)
	}
	return getFileParent, nil
}

// depsLocalGerritParentConfig performs validation of the
// DEPSLocalGerritParentConfig, making external network requests as needed.
func (dv *deepvalidator) depsLocalGerritParentConfig(ctx context.Context, c *config.DEPSLocalGerritParentConfig, tipRev *revision.Revision, getFileChild version_file_common.GetFileFunc) (version_file_common.GetFileFunc, error) {
	if err := dv.gerritConfig(ctx, c.Gerrit); err != nil {
		return nil, skerr.Wrap(err)
	}
	return dv.depsLocalParentConfig(ctx, c.DepsLocal, tipRev, getFileChild)
}

// gitCheckoutGitHubFileParentConfig performs validation of the
// GitCheckoutGitHubFileParentConfig, making external network requests as needed.
func (dv *deepvalidator) gitCheckoutGitHubFileParentConfig(ctx context.Context, c *config.GitCheckoutGitHubFileParentConfig, tipRev *revision.Revision, getFileChild version_file_common.GetFileFunc) (version_file_common.GetFileFunc, error) {
	getFileParent, err := dv.gitCheckoutGitHubParentConfig(ctx, c.GitCheckout, getFileChild)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if c.PreUploadCommands != nil {
		if err := dv.preUploadConfig(ctx, c.PreUploadCommands, tipRev, getFileParent); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return getFileParent, nil
}

// gitCheckoutGitHubParentConfig performs validation of the
// GitCheckoutGitHubParentConfig, making external network requests as needed.
func (dv *deepvalidator) gitCheckoutGitHubParentConfig(ctx context.Context, c *config.GitCheckoutGitHubParentConfig, getFileChild version_file_common.GetFileFunc) (version_file_common.GetFileFunc, error) {
	// TODO(borenet): Can we rely on main being present?
	if _, _, err := dv.deepValidateGitHubRepo(ctx, c.ForkRepoUrl, git.MainBranch); err != nil {
		return nil, skerr.Wrap(err)
	}
	return dv.gitCheckoutParentConfig(ctx, c.GitCheckout, getFileChild)
}

// gitCheckoutGerritParentConfig performs validation of the
// GitCheckoutGerritParentConfig, making external network requests as needed.
func (dv *deepvalidator) gitCheckoutGerritParentConfig(ctx context.Context, c *config.GitCheckoutGerritParentConfig, tipRev *revision.Revision, getFileChild version_file_common.GetFileFunc) (version_file_common.GetFileFunc, error) {
	getFileParent, err := dv.gitCheckoutParentConfig(ctx, c.GitCheckout, getFileChild)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if c.PreUploadCommands != nil {
		if err := dv.preUploadConfig(ctx, c.PreUploadCommands, tipRev, getFileParent); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return getFileParent, nil
}

// buildbucketRevisionFilterConfig performs validation of the
// BuildbucketRevisionFilterConfig, making external network requests as needed.
func (dv *deepvalidator) buildbucketRevisionFilterConfig(ctx context.Context, c *config.BuildbucketRevisionFilterConfig, rev *revision.Revision) error {
	filter, err := revision_filter.NewBuildbucketRevisionFilter(dv.bbClient, c)
	if err != nil {
		return skerr.Wrap(err)
	}
	if _, err := filter.Skip(ctx, *rev); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// cipdRevisionFilterConfig performs validation of the
// CIPDRevisionFilterConfig, making external network requests as needed.
func (dv *deepvalidator) cipdRevisionFilterConfig(ctx context.Context, c *config.CIPDRevisionFilterConfig, rev *revision.Revision) error {
	filter, err := revision_filter.NewCIPDRevisionFilter(dv.cipdClient, c)
	if err != nil {
		return skerr.Wrap(err)
	}
	// Note: there could still be a problem here even if no error is returned;
	// if skipReason is not empty, it could indicate that the filter is in some
	// way invalid.
	if _, err := filter.Skip(ctx, *rev); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// dockerChildConfig performs validation of the DockerChildConfig,
// making external network requests as needed.
func (dv *deepvalidator) dockerChildConfig(ctx context.Context, c *config.DockerChildConfig) (*revision.Revision, error) {
	dockerChild, err := child.NewDocker(ctx, c, dv.dockerClient)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create docker child")
	}
	// Create a fake revision to pass into Update. We don't have a real
	// last-rolled revision, but Update requires one.
	fakeRev := makeFakeRevision()
	tipRev, _, err := dockerChild.Update(ctx, fakeRev)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to get revision for tag %q", c.Tag)
	}
	return tipRev, nil
}

// preUploadConfig performs validation of the PreUploadConfig, making
// external network requests as needed.
func (dv *deepvalidator) preUploadConfig(ctx context.Context, c *config.PreUploadConfig, tipRev *revision.Revision, getFile version_file_common.GetFileFunc) error {
	// Note: we don't directly call Placeholders.Command (or
	// Placeholders.PreUploadConfig) because that's done in deepValidateCommand.
	cipdPkgs, err := fakePlaceholders(tipRev).CIPDPackages(c.CipdPackage)
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, pkg := range cipdPkgs {
		if _, err := dv.cipdClient.ResolveVersion(ctx, pkg.Name, pkg.Version); err != nil {
			return skerr.Wrapf(err, "failed deep-validating CIPDChildConfig")
		}
	}
	for _, cmd := range c.Command {
		// TODO(borenet): Callers generally pass in depot tools env to
		// RunPreUploadStep.
		cmdEnv := exec.MergeEnv(os.Environ(), cmd.Env)
		if err := deepValidateCommand(ctx, strings.Fields(cmd.Command), cmd.Cwd, cmdEnv, tipRev, getFile); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// reviewer performs validation of a reviewer, making external network requests
// as needed.
func (dv *deepvalidator) reviewer(ctx context.Context, reviewer string) error {
	_, err := roller.GetReviewersFromURLOrEmail(dv.client, reviewer)
	return skerr.Wrap(err)
}

// cqExtraTrybot performs validation of the cqExtraTrybot, making external
// network requests as needed.
func (dv *deepvalidator) cqExtraTrybot(ctx context.Context, trybot string) error {
	project, bucket, builders, err := config.ParseTrybotName(trybot)
	if err != nil {
		return skerr.Wrap(err)
	}
	if project == "skia" {
		// Skia is a special case: we use dynamic builders, which won't have a
		// definition that can be found via GetBuilder. Here we just have to
		// assume that the builder exists.
		return nil
	}
	for _, builder := range builders {
		bbBuilder, err := dv.bbClient.GetBuilder(ctx, &buildbucketpb.GetBuilderRequest{
			Id: &buildbucketpb.BuilderID{
				Project: project,
				Bucket:  bucket,
				Builder: builder,
			},
		})
		if project == "skia" {
			// Skia is a special case: we use dynamic builders, which won't have
			// a definition that can be found via GetBuilder. Here we just have
			// to assume that the builder exists.
			return nil
		}
		if err != nil {
			return skerr.Wrapf(err, "failed to retrieve buildbucket builder for %q (project %q bucket %q builder %q)", trybot, project, bucket, builder)
		} else if bbBuilder == nil {
			return skerr.Fmt("no buildbucket builder for %q (project %q bucket %q builder %q)", trybot, project, bucket, builder)
		}
	}
	return nil
}

// commitMsg performs validation of the CommitMsg, making external network
// requests as needed.
func (dv *deepvalidator) commitMsg(ctx context.Context, c *config.CommitMsgConfig) error {
	for _, trybot := range c.CqExtraTrybots {
		if err := dv.cqExtraTrybot(ctx, trybot); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// makeFakeRevision can be used wherever we need to pass in a Revision but the
// actual value is not important.
func makeFakeRevision() *revision.Revision {
	return &revision.Revision{
		Id: "fake",
	}
}
