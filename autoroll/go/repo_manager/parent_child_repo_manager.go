package repo_manager

import (
	"context"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/child/revision_filter"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// ParentChildRepoManagerConfig provides configuration for a RepoManager which
// combines a Parent and Child.
type ParentChildRepoManagerConfig struct {
	// Exactly one of the following Parent types must be provided.
	CopyParent                  *parent.CopyConfig                  `json:"copyParent,omitempty"`
	DEPSLocalGitHubParent       *parent.DEPSLocalGithubConfig       `json:"depsLocalGitHubParent,omitempty"`
	DEPSLocalGerritParent       *parent.DEPSLocalGerritConfig       `json:"depsLocalGerritParent,omitempty"`
	GitCheckoutGithubFileParent *parent.GitCheckoutGithubFileConfig `json:"gitCheckoutGitHubFileParent,omitempty"`
	GitilesParent               *parent.GitilesConfig               `json:"gitilesParent,omitempty"`

	// Exactly one of the following Child types must be provided.
	CIPDChild              *child.CIPDConfig              `json:"cipdChild,omitempty"`
	FuchsiaSDKChild        *child.FuchsiaSDKConfig        `json:"fuchsiaSdkChild,omitempty"`
	GitCheckoutChild       *child.GitCheckoutConfig       `json:"gitCheckoutChild,omitempty"`
	GitCheckoutGitHubChild *child.GitCheckoutGithubConfig `json:"gitCheckoutGitHubChild,omitempty"`
	GitilesChild           *child.GitilesConfig           `json:"gitilesChild,omitempty"`
	SemVerGCSChild         *child.SemVerGCSConfig         `json:"semVerGcsChild,omitempty"`

	// Revision filters. Optional.
	BuildbucketRevisionFilter *revision_filter.BuildbucketRevisionFilterConfig `json:"buildbucketFilter,omitempty"`
}

// Validate implements util.Validator.
func (c *ParentChildRepoManagerConfig) Validate() error {
	// Parent.
	var parents []util.Validator
	if c.CopyParent != nil {
		parents = append(parents, c.CopyParent)
	}
	if c.DEPSLocalGitHubParent != nil {
		parents = append(parents, c.DEPSLocalGitHubParent)
	}
	if c.DEPSLocalGerritParent != nil {
		parents = append(parents, c.DEPSLocalGerritParent)
	}
	if c.GitCheckoutGithubFileParent != nil {
		parents = append(parents, c.GitCheckoutGithubFileParent)
	}
	if c.GitilesParent != nil {
		parents = append(parents, c.GitilesParent)
	}
	if len(parents) != 1 {
		return skerr.Fmt("ParentChildRepoManagerConfig requires exactly one Parent")
	}
	if err := parents[0].Validate(); err != nil {
		return skerr.Wrap(err)
	}

	// Child.
	var children []util.Validator
	if c.CIPDChild != nil {
		children = append(children, c.CIPDChild)
	}
	if c.FuchsiaSDKChild != nil {
		children = append(children, c.FuchsiaSDKChild)
	}
	if c.GitCheckoutChild != nil {
		children = append(children, c.GitCheckoutChild)
	}
	if c.GitCheckoutGitHubChild != nil {
		children = append(children, c.GitCheckoutGitHubChild)
	}
	if c.GitilesChild != nil {
		children = append(children, c.GitilesChild)
	}
	if c.SemVerGCSChild != nil {
		children = append(children, c.SemVerGCSChild)
	}
	if len(children) != 1 {
		return skerr.Fmt("ParentChildRepoManagerConfig requires exactly one Child")
	}
	if err := children[0].Validate(); err != nil {
		return skerr.Wrap(err)
	}

	// RevisionFilters.
	if c.BuildbucketRevisionFilter != nil {
		if err := c.BuildbucketRevisionFilter.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}

	return nil
}

// ParentChildRepoManagerConfigToProto converts a ParentChildRepoManagerConfig
// to a config.ParentChildRepoManagerConfig.
func ParentChildRepoManagerConfigToProto(cfg *ParentChildRepoManagerConfig) *config.ParentChildRepoManagerConfig {
	rv := &config.ParentChildRepoManagerConfig{}

	// Parent.
	if cfg.CopyParent != nil {
		rv.Parent = &config.ParentChildRepoManagerConfig_CopyParent{
			CopyParent: parent.CopyConfigToProto(cfg.CopyParent),
		}
	} else if cfg.DEPSLocalGitHubParent != nil {
		rv.Parent = &config.ParentChildRepoManagerConfig_DepsLocalGithubParent{
			DepsLocalGithubParent: parent.DEPSLocalGithubConfigToProto(cfg.DEPSLocalGitHubParent),
		}
	} else if cfg.DEPSLocalGerritParent != nil {
		rv.Parent = &config.ParentChildRepoManagerConfig_DepsLocalGerritParent{
			DepsLocalGerritParent: parent.DEPSLocalGerritConfigToProto(cfg.DEPSLocalGerritParent),
		}
	} else if cfg.GitCheckoutGithubFileParent != nil {
		rv.Parent = &config.ParentChildRepoManagerConfig_GitCheckoutGithubFileParent{
			GitCheckoutGithubFileParent: parent.GitCheckoutGithubFileConfigToProto(cfg.GitCheckoutGithubFileParent),
		}
	} else if cfg.GitilesParent != nil {
		rv.Parent = &config.ParentChildRepoManagerConfig_GitilesParent{
			GitilesParent: parent.GitilesConfigToProto(cfg.GitilesParent),
		}
	}

	// Child.
	if cfg.CIPDChild != nil {
		rv.Child = &config.ParentChildRepoManagerConfig_CipdChild{
			CipdChild: child.CIPDConfigToProto(cfg.CIPDChild),
		}
	} else if cfg.FuchsiaSDKChild != nil {
		rv.Child = &config.ParentChildRepoManagerConfig_FuchsiaSdkChild{
			FuchsiaSdkChild: child.FuchsiaSDKConfigToProto(cfg.FuchsiaSDKChild),
		}
	} else if cfg.GitCheckoutChild != nil {
		rv.Child = &config.ParentChildRepoManagerConfig_GitCheckoutChild{
			GitCheckoutChild: child.GitCheckoutConfigToProto(cfg.GitCheckoutChild),
		}
	} else if cfg.GitCheckoutGitHubChild != nil {
		rv.Child = &config.ParentChildRepoManagerConfig_GitCheckoutGithubChild{
			GitCheckoutGithubChild: child.GitCheckoutGithubConfigToProto(cfg.GitCheckoutGitHubChild),
		}
	} else if cfg.GitilesChild != nil {
		rv.Child = &config.ParentChildRepoManagerConfig_GitilesChild{
			GitilesChild: child.GitilesConfigToProto(cfg.GitilesChild),
		}
	} else if cfg.SemVerGCSChild != nil {
		rv.Child = &config.ParentChildRepoManagerConfig_SemverGcsChild{
			SemverGcsChild: child.SemVerGCSConfigToProto(cfg.SemVerGCSChild),
		}
	}

	// Revision filter.
	if cfg.BuildbucketRevisionFilter != nil {
		rv.RevisionFilter = &config.ParentChildRepoManagerConfig_BuildbucketRevisionFilter{
			BuildbucketRevisionFilter: revision_filter.BuildBucketRevisionFilterConfigToProto(cfg.BuildbucketRevisionFilter),
		}
	}

	return rv
}

// ProtoToParentChildRepoManagerConfig converts a
// config.ParentChildRepoManagerConfig to a ParentChildRepoManagerConfig.
func ProtoToParentChildRepoManagerConfig(cfg *config.ParentChildRepoManagerConfig) (*ParentChildRepoManagerConfig, error) {
	rv := &ParentChildRepoManagerConfig{}

	// Parent.
	var err error
	if cfg.Parent != nil {
		if p, ok := cfg.Parent.(*config.ParentChildRepoManagerConfig_CopyParent); ok {
			rv.CopyParent, err = parent.ProtoToCopyConfig(p.CopyParent)
		} else if p, ok := cfg.Parent.(*config.ParentChildRepoManagerConfig_DepsLocalGithubParent); ok {
			rv.DEPSLocalGitHubParent, err = parent.ProtoToDEPSLocalGithubConfig(p.DepsLocalGithubParent)
		} else if p, ok := cfg.Parent.(*config.ParentChildRepoManagerConfig_DepsLocalGerritParent); ok {
			rv.DEPSLocalGerritParent, err = parent.ProtoToDEPSLocalGerritConfig(p.DepsLocalGerritParent)
		} else if p, ok := cfg.Parent.(*config.ParentChildRepoManagerConfig_GitCheckoutGithubFileParent); ok {
			rv.GitCheckoutGithubFileParent, err = parent.ProtoToGitCheckoutGithubFileConfig(p.GitCheckoutGithubFileParent)
		} else if p, ok := cfg.Parent.(*config.ParentChildRepoManagerConfig_GitilesParent); ok {
			rv.GitilesParent, err = parent.ProtoToGitilesConfig(p.GitilesParent)
		}
	}

	// Child.
	if cfg.Child != nil {
		if c, ok := cfg.Child.(*config.ParentChildRepoManagerConfig_CipdChild); ok {
			rv.CIPDChild = child.ProtoToCIPDConfig(c.CipdChild)
		} else if c, ok := cfg.Child.(*config.ParentChildRepoManagerConfig_FuchsiaSdkChild); ok {
			rv.FuchsiaSDKChild = child.ProtoToFuchsiaSDKConfig(c.FuchsiaSdkChild)
		} else if c, ok := cfg.Child.(*config.ParentChildRepoManagerConfig_GitCheckoutChild); ok {
			rv.GitCheckoutChild, err = child.ProtoToGitCheckoutConfig(c.GitCheckoutChild)
		} else if c, ok := cfg.Child.(*config.ParentChildRepoManagerConfig_GitCheckoutGithubChild); ok {
			rv.GitCheckoutGitHubChild, err = child.ProtoToGitCheckoutGithubConfig(c.GitCheckoutGithubChild)
		} else if c, ok := cfg.Child.(*config.ParentChildRepoManagerConfig_GitilesChild); ok {
			rv.GitilesChild, err = child.ProtoToGitilesConfig(c.GitilesChild)
		} else if c, ok := cfg.Child.(*config.ParentChildRepoManagerConfig_SemverGcsChild); ok {
			rv.SemVerGCSChild, err = child.ProtoToSemVerGCSConfig(c.SemverGcsChild)
		}
	}

	// Revision filter.
	if cfg.RevisionFilter != nil {
		if f, ok := cfg.RevisionFilter.(*config.ParentChildRepoManagerConfig_BuildbucketRevisionFilter); ok {
			rv.BuildbucketRevisionFilter = revision_filter.ProtoToBuildbucketRevisionFilterConfig(f.BuildbucketRevisionFilter)
		}
	}

	return rv, skerr.Wrap(err)
}

// parentChildRepoManager combines a Parent and a Child to implement the
// RepoManager interface.
type parentChildRepoManager struct {
	child.Child
	parent.Parent
	revFilter revision_filter.RevisionFilter
}

// newParentChildRepoManager returns a RepoManager which pairs a Parent with a
// Child.
func newParentChildRepoManager(ctx context.Context, p parent.Parent, c child.Child, revFilter revision_filter.RevisionFilter) (*parentChildRepoManager, error) {
	return &parentChildRepoManager{
		Child:     c,
		Parent:    p,
		revFilter: revFilter,
	}, nil
}

// See documentation for RepoManager interface.
func (rm *parentChildRepoManager) Update(ctx context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	lastRollRevId, err := rm.Parent.Update(ctx)
	if err != nil {
		return nil, nil, nil, skerr.Wrapf(err, "failed to update Parent")
	}
	lastRollRev, err := rm.Child.GetRevision(ctx, lastRollRevId)
	if err != nil {
		sklog.Errorf("Last roll rev %q not found. This is acceptable for some rollers which allow outside versions to be rolled manually (eg. AFDO roller). A human should verify that this is indeed caused by a manual roll. Attempting to continue with no last-rolled revision. The revisions listed in the commit message will be incorrect!", lastRollRevId)
		lastRollRev = &revision.Revision{Id: lastRollRevId}
	}
	tipRev, notRolledRevs, err := rm.Child.Update(ctx, lastRollRev)
	if err != nil {
		return nil, nil, nil, skerr.Wrapf(err, "failed to get next revision to roll from Child")
	}
	// Optionally filter not-rolled revisions.
	if rm.revFilter != nil {
		if err := revision_filter.MaybeSetInvalid(ctx, rm.revFilter, tipRev); err != nil {
			return nil, nil, nil, skerr.Wrap(err)
		}
		for _, notRolledRev := range notRolledRevs {
			if err := revision_filter.MaybeSetInvalid(ctx, rm.revFilter, notRolledRev); err != nil {
				return nil, nil, nil, skerr.Wrap(err)
			}
		}
	}
	return lastRollRev, tipRev, notRolledRevs, nil
}

// parentChildRepoManager implements RepoManager.
var _ RepoManager = &parentChildRepoManager{}
