package parent

/*
   Package parent contains implementations of the Parent interface.
*/

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// Parent represents a git repo (or other destination) which depends on a Child
// and is capable of producing rolls.
type Parent interface {
	// Update returns the pinned version of the dependency at the most
	// recent revision of the Parent. For implementations which use local
	// checkouts, this implies a sync.
	Update(context.Context) (string, error)

	// CreateNewRoll uploads a CL which updates the pinned version of the
	// dependency to the given Revision.
	CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, dryRun bool, commitMsg string) (int64, error)
}

// Config provides configuration for a Parent.
// Exactly one of the fields should be specified.
type Config struct {
	Copy                  *CopyConfig                  `json:"copy"`
	Freetype              *GitilesConfig               `json:"freetype"`
	GitCheckoutGithubFile *GitCheckoutGithubFileConfig `json:"gitCheckoutGithubFile"`
	GithubDEPSLocal       *GithubDEPSLocalConfig       `json:"githubDepsLocal"`
	GitilesDEPSLocal      *DEPSLocalConfig             `json:"gitilesDEPSLocal"`
	GitilesFile           *GitilesConfig               `json:"gitilesFile"`
}

// Validate returns an error if the Config is invalid.
func (c Config) Validate() error {
	var cfgs []util.Validator
	if c.Copy != nil {
		cfgs = append(cfgs, c.Copy)
	}
	if c.Freetype != nil {
		cfgs = append(cfgs, c.Freetype)
	}
	if c.GitCheckoutGithubFile != nil {
		cfgs = append(cfgs, c.GitCheckoutGithubFile)
	}
	if c.GithubDEPSLocal != nil {
		cfgs = append(cfgs, c.GithubDEPSLocal)
	}
	if c.GitilesFile != nil {
		cfgs = append(cfgs, c.GitilesFile)
	}
	if len(cfgs) != 1 {
		return skerr.Fmt("exactly one config must be provided, but got %d", len(cfgs))
	}
	return skerr.Wrap(cfgs[0].Validate())
}

// New returns a Parent based on the given Config.
func New(ctx context.Context, c Config, reg *config_vars.Registry, client *http.Client, gerritClient *gerrit.Gerrit, githubClient *github.GitHub, serverURL, workdir, rollerName, userName, userEmail, recipeCfgFile string, childRM child.Child) (Parent, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	if c.Copy != nil {
		return NewCopy(ctx, *c.Copy, reg, client, serverURL, workdir, userName, userEmail, childRM)
	} else if c.Freetype != nil {
		return NewFreeTypeParent(ctx, *c.Freetype, reg, workdir, client, serverURL)
	} else if c.GitCheckoutGithubFile != nil {
		return NewGitCheckoutGithubFile(ctx, *c.GitCheckoutGithubFile, reg, client, githubClient, serverURL, workdir, userName, userEmail, rollerName, nil)
	} else if c.GithubDEPSLocal != nil {
		return NewGithubDEPSLocal(ctx, *c.GithubDEPSLocal, reg, client, githubClient, serverURL, workdir, rollerName, userName, userEmail, recipeCfgFile)
	} else if c.GitilesDEPSLocal != nil {
		return NewGitilesDEPSLocal(ctx, *c.GitilesDEPSLocal, reg, client, gerritClient, serverURL, workdir, userName, userEmail, recipeCfgFile)
	} else if c.GitilesFile != nil {
		return NewGitilesFile(ctx, *c.GitilesFile, reg, client, serverURL)
	}
	return nil, skerr.Fmt("no known Parent exists for this config")
}
