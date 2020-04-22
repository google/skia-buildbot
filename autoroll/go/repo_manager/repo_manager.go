package repo_manager

import (
	"context"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/skerr"
)

// RepoManager is the interface used by different Autoroller implementations
// to manage checkouts.
type RepoManager interface {
	// Create a new roll attempt.
	CreateNewRoll(context.Context, *revision.Revision, *revision.Revision, []*revision.Revision, []string, string, bool) (int64, error)

	// Update the RepoManager's view of the world. Depending on the
	// implementation, this may sync repos and may take some time. Returns
	// the currently-rolled Revision, the tip-of-tree Revision, and a list
	// of all revisions which have not yet been rolled (ie. those between
	// the current and tip-of-tree, including the latter), in reverse
	// chronological order.
	Update(context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error)

	// GetRevision returns a revision.Revision instance from the given
	// revision ID.
	GetRevision(context.Context, string) (*revision.Revision, error)
}

// CommonRepoManagerConfig provides configuration for commonRepoManager.
type CommonRepoManagerConfig struct {
	// Required fields.

	// Branch of the child repo we want to roll.
	ChildBranch *config_vars.Template `json:"childBranch"`
	// Path of the child repo within the parent repo.
	ChildPath string `json:"childPath"`
	// If false, roll CLs do not link to bugs from the commits in the child
	// repo.
	IncludeBugs bool `json:"includeBugs"`
	// If true, include the "git log" (or other revision details) in the
	// commit message. This should be false for internal -> external rollers
	// to avoid leaking internal commit messages.
	IncludeLog bool `json:"includeLog"`
	// Branch of the parent repo we want to roll into.
	ParentBranch *config_vars.Template `json:"parentBranch"`
	// URL of the parent repo.
	ParentRepo string `json:"parentRepo"`

	// Optional fields.

	// ChildRevLinkTmpl is a template used to create links to revisions of
	// the child repo. If not supplied, no links will be created.
	ChildRevLinkTmpl string `json:"childRevLinkTmpl"`
	// CommitMsgTmpl is a template used to build commit messages. See the
	// parent.CommitMsgVars type for more information.
	CommitMsgTmpl string `json:"commitMsgTmpl"`
	// ChildSubdir indicates the subdirectory of the workdir in which
	// the childPath should be rooted. In most cases, this should be empty,
	// but if ChildPath is relative to the parent repo dir (eg. when DEPS
	// specifies use_relative_paths), then this is required.
	ChildSubdir string `json:"childSubdir,omitempty"`
	// Monorail project name associated with the parent repo.
	BugProject string `json:"bugProject,omitempty"`
	// Named steps to run before uploading roll CLs.
	PreUploadSteps []string `json:"preUploadSteps,omitempty"`
}

// Validate the config.
func (c *CommonRepoManagerConfig) Validate() error {
	if c.ChildBranch == nil {
		return skerr.Fmt("ChildBranch is required.")
	}
	if err := c.ChildBranch.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.ChildPath == "" {
		return skerr.Fmt("ChildPath is required.")
	}
	if c.ParentBranch == nil {
		return skerr.Fmt("ParentBranch is required.")
	}
	if err := c.ParentBranch.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if c.ParentRepo == "" {
		return skerr.Fmt("ParentRepo is required.")
	}
	if c.IncludeBugs && c.BugProject == "" {
		return skerr.Fmt("IncludeBugs is true, but BugProject is empty.")
	}
	if proj := issues.REPO_PROJECT_MAPPING[c.ParentRepo]; proj != "" && c.BugProject != "" && proj != c.BugProject {
		return skerr.Fmt("BugProject is non-empty but does not match the entry in issues.REPO_PROJECT_MAPPING.")
	}
	for _, s := range c.PreUploadSteps {
		if _, err := parent.GetPreUploadStep(s); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// See documentation for RepoManagerConfig interface.
func (r *CommonRepoManagerConfig) DefaultStrategy() string {
	return strategy.ROLL_STRATEGY_BATCH
}

// See documentation for RepoManagerConfig interface.
func (r *CommonRepoManagerConfig) NoCheckout() bool {
	return false
}

// See documentation for RepoManagerConfig interface.
func (r *CommonRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
		strategy.ROLL_STRATEGY_SINGLE,
	}
}

// DepotToolsRepoManagerConfig provides configuration for depotToolsRepoManager.
type DepotToolsRepoManagerConfig struct {
	CommonRepoManagerConfig

	// Optional fields.

	// Override the default gclient spec with this string.
	GClientSpec string `json:"gclientSpec,omitempty"`

	// Run "gclient runhooks" if true.
	RunHooks bool `json:"runhooks,omitempty"`
}

// NoCheckoutRepoManagerConfig provides configuration for RepoManagers which
// don't use a local checkout.
type NoCheckoutRepoManagerConfig struct {
	CommonRepoManagerConfig
}

// See documentation for RepoManagerConfig interface.
func (c *NoCheckoutRepoManagerConfig) NoCheckout() bool {
	return true
}

// See documentation for util.Validator interface.
func (c *NoCheckoutRepoManagerConfig) Validate() error {
	if err := c.CommonRepoManagerConfig.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if len(c.PreUploadSteps) > 0 {
		return skerr.Fmt("Checkout-less rollers don't support pre-upload steps")
	}
	return nil
}
