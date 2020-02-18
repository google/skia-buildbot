package branch

import (
	"context"
	"net/http"

	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// BranchConfig represents configuration for a Branch.
type BranchConfig interface {
	util.Validator

	// Create an instance of Branch based on this BranchConfig.
	Create(context.Context, *http.Client) (Branch, error)
}

// Config is the top-level configuration for a Branch.
type Config struct {
	// Specific types of branch configs. Exactly one must be provided.
	Static *StaticBranchConfig `json:"static"`
	Chrome *ChromeBranchConfig `json:"chrome"`
}

// branchConfig returns the specific BranchConfig or an error if zero or more
// than one BranchConfig was provided.
func (c *Config) branchConfig() (BranchConfig, error) {
	cfgs := []BranchConfig{}
	if c.Static != nil {
		cfgs = append(cfgs, c.Static)
	}
	if c.Chrome != nil {
		cfgs = append(cfgs, c.Chrome)
	}
	if len(cfgs) == 1 {
		return cfgs[0], nil
	}
	return nil, skerr.Fmt("Exactly one branch configuration is required.")
}

// See documentation for BranchConfig interface.
func (c *Config) Validate() error {
	bc, err := c.branchConfig()
	if err != nil {
		return skerr.Wrap(err)
	}
	return bc.Validate()
}

// See documentation for BranchConfig interface.
func (c *Config) Create(ctx context.Context, client *http.Client) (Branch, error) {
	cfg, err := c.branchConfig()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return cfg.Create(ctx, client)
}

// StaticBranchConfig is configuration for a StaticBranch. It is simply the name
// or fully-qualified ref of the desired branch.
type StaticBranchConfig string

// See documentation for BranchConfig interface.
func (c *StaticBranchConfig) Validate() error {
	if string(*c) == "" {
		return skerr.Fmt("Ref is required.")
	}
	return nil
}

// See documentation for BranchConfig interface.
func (c *StaticBranchConfig) Create(ctx context.Context, client *http.Client) (Branch, error) {
	return NewStaticBranch(string(*c)), nil
}

// ChromeBranchConfig is configuration for a ChromeBranch. It is a template
// which will be used in combination with the current set of Chrome release
// branches at a given time to derive the actual ref.
type ChromeBranchConfig string

// See documentation for BranchConfig interface.
func (c *ChromeBranchConfig) Validate() error {
	tmpl := string(*c)
	if tmpl == "" {
		return skerr.Fmt("Template is required.")
	}
	branches := &chrome_branch.Branches{
		Beta:   "beta",
		Stable: "stable",
	}
	// Self-check, in case the chrome_branch.Branches struct changes.
	if err := branches.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	ref, err := chrome_branch.Execute(tmpl, branches)
	if err != nil {
		return skerr.Wrap(err)
	}
	if ref == tmpl {
		return skerr.Fmt("Template does not make use of Chrome branch; use static branch instead.")
	}
	return nil
}

// See documentation for BranchConfig interface.
func (c *ChromeBranchConfig) Create(ctx context.Context, client *http.Client) (Branch, error) {
	return NewChromeBranch(ctx, client, string(*c))
}

// Assert that the config structs implement the BranchConfig interface.
var _ BranchConfig = &Config{}
var _ BranchConfig = new(StaticBranchConfig)
var _ BranchConfig = new(ChromeBranchConfig)
