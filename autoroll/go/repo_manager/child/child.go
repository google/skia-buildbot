package child

/*
   Package child contains implementations of the Child interface.
*/

import (
	"context"
	"net/http"

	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vfs"
)

// Child represents a Child (git repo or otherwise) which can be rolled into a
// Parent.
type Child interface {
	// Update updates the local view of the Child and returns the tip-of-
	// tree Revision and the list of not-yet-rolled revisions, or any error
	// which occurred, given the last-rolled revision.
	Update(context.Context, *revision.Revision) (*revision.Revision, []*revision.Revision, error)

	// GetRevision returns a Revision instance associated with the given
	// revision ID.
	GetRevision(context.Context, string) (*revision.Revision, error)

	// VFS returns a vfs.FS instance which reads from this Child at the given
	// Revision.
	VFS(context.Context, *revision.Revision) (vfs.FS, error)
}

// Config provides configuration for a Child.
// Exactly one of the fields should be specified.
type Config struct {
	CIPD              *CIPDConfig              `json:"cipd"`
	FuchsiaSDK        *FuchsiaSDKConfig        `json:"fuchsiaSDK"`
	GitCheckout       *GitCheckoutConfig       `json:"gitCheckout"`
	GitCheckoutGithub *GitCheckoutGithubConfig `json:"gitCheckoutGithub"`
	Gitiles           *GitilesConfig           `json:"gitiles"`
	SemVerGCS         *SemVerGCSConfig         `json:"semVerGCS"`
}

// Validate returns an error if the Config is invalid.
func (c Config) Validate() error {
	var cfgs []util.Validator
	if c.CIPD != nil {
		cfgs = append(cfgs, c.CIPD)
	}
	if c.FuchsiaSDK != nil {
		cfgs = append(cfgs, c.FuchsiaSDK)
	}
	if c.GitCheckout != nil {
		cfgs = append(cfgs, c.GitCheckout)
	}
	if c.GitCheckoutGithub != nil {
		cfgs = append(cfgs, c.GitCheckoutGithub)
	}
	if c.Gitiles != nil {
		cfgs = append(cfgs, c.Gitiles)
	}
	if c.SemVerGCS != nil {
		cfgs = append(cfgs, c.SemVerGCS)
	}
	if len(cfgs) != 1 {
		return skerr.Fmt("exactly one config must be provided, but got %d", len(cfgs))
	}
	return skerr.Wrap(cfgs[0].Validate())
}

// New returns a Child based on the given Config.
func New(ctx context.Context, c Config, reg *config_vars.Registry, client *http.Client, workdir, userName, userEmail string) (Child, error) {
	if err := c.Validate(); err != nil {
		return nil, skerr.Wrap(err)
	}
	if c.CIPD != nil {
		return NewCIPD(ctx, *c.CIPD, client, workdir)
	}
	if c.FuchsiaSDK != nil {
		return NewFuchsiaSDK(ctx, *c.FuchsiaSDK, client)
	}
	if c.GitCheckout != nil {
		return NewGitCheckout(ctx, *c.GitCheckout, reg, workdir, userName, userEmail)
	}
	if c.GitCheckoutGithub != nil {
		return NewGitCheckoutGithub(ctx, *c.GitCheckoutGithub, reg, client, workdir, userName, userEmail)
	}
	if c.Gitiles != nil {
		return NewGitiles(ctx, *c.Gitiles, reg, client)
	}
	if c.SemVerGCS != nil {
		return NewSemVerGCS(ctx, *c.SemVerGCS, reg, client)
	}
	return nil, skerr.Fmt("no known Child exists for this config")
}
