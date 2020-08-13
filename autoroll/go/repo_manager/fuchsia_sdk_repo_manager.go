package repo_manager

import (
	"context"
	"fmt"
	"net/http"

	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
)

const (
	FuchsiaSDKVersionFilePathLinux = "build/fuchsia/linux.sdk.sha1"
	FuchsiaSDKVersionFilePathMac   = "build/fuchsia/mac.sdk.sha1"
)

// FuchsiaSDKRepoManagerConfig provides configuration for the Fuchia SDK
// RepoManager.
type FuchsiaSDKRepoManagerConfig struct {
	Parent parent.GitilesConfig   `json:"parent"`
	Child  child.FuchsiaSDKConfig `json:"child"`
}

// See documentation for util.Validator interface.
func (c *FuchsiaSDKRepoManagerConfig) Validate() error {
	if err := c.Parent.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if err := c.Child.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// See documentation for RepoManagerConfig interface.
func (c *FuchsiaSDKRepoManagerConfig) ValidStrategies() []string {
	return []string{
		strategy.ROLL_STRATEGY_BATCH,
	}
}

// NewFuchsiaSDKRepoManager returns a RepoManager instance which rolls the
// Fuchsia SDK.
func NewFuchsiaSDKRepoManager(ctx context.Context, c *FuchsiaSDKRepoManagerConfig, reg *config_vars.Registry, workdir string, g gerrit.GerritInterface, serverURL string, authClient *http.Client, cr codereview.CodeReview, local bool) (*parentChildRepoManager, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to validate config: %s", err)
	}
	parentRM, err := parent.NewGitilesFile(ctx, c.Parent, reg, authClient, serverURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	childRM, err := child.NewFuchsiaSDK(ctx, c.Child, authClient)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return newParentChildRepoManager(ctx, parentRM, childRM, nil)
}
