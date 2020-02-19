package config_vars

/*
	Package config_template provides helpers for configuration templates.

	Template is designed to be created directly from config files via eg.
	json.Unmarshal. Init() must be called before any Template is used, and
	Update() should be called periodically. Users should be careful to
	either cache the value of Template.String() or stage calls to Update()
	so that the resulting value does not change during use.
*/

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"net/http"
	"sync"

	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/skerr"
)

var (
	// global Vars instance; manipulate via Update().
	global *Vars
	// globalMtx protects the global Vars instance.
	globalMtx sync.RWMutex
	// globalBranchClient is set during Init() and may be overridden for
	// testing.
	globalBranchClient chrome_branch.Client
)

// Vars represents current values of variables which may be used in templates.
type Vars struct {
	Branches *Branches `json:"branches"`
}

// Validate returns an error if Vars is not valid.
func (v *Vars) Validate() error {
	if v.Branches == nil {
		return errors.New("Branches is required.")
	}
	if err := v.Branches.Validate(); err != nil {
		return err
	}
	return nil
}

// Branches represents named branches in git repositories.
type Branches struct {
	// Chromium release branches.
	Chromium *chrome_branch.Branches `json:"chromium"`
}

// Validate returns an error if Branches is not valid.
func (b *Branches) Validate() error {
	if b.Chromium == nil {
		return errors.New("Chromium is required.")
	}
	if err := b.Chromium.Validate(); err != nil {
		return err
	}
	return nil
}

// Template is a text template which uses the contents of Vars.
type Template string

// See documentation for BranchConfig interface.
func (t *Template) Validate() error {
	if t == nil || *t == "" {
		return skerr.Fmt("Template is missing.")
	}
	// Create dummy Vars to test the template.
	v := &Vars{
		Branches: &Branches{
			Chromium: &chrome_branch.Branches{
				Beta: &chrome_branch.Branch{
					Milestone: 81,
					Number:    4044,
				},
				Stable: &chrome_branch.Branch{
					Milestone: 80,
					Number:    3987,
				},
			},
		},
	}
	// Self-check, in case any of the members of Vars change.
	if err := v.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := t.Execute(v); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Execute the Template using the given Vars.
func (t *Template) Execute(v *Vars) (string, error) {
	tmpl, err := template.New("").Parse(string(*t))
	if err != nil {
		return "", skerr.Wrap(err)
	}
	tmpl.Option("missingkey=error")
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, v); err != nil {
		return "", skerr.Wrap(err)
	}
	return buf.String(), nil
}

// String executes the Template against the global Vars instance and returns the
// resulting string. Panics if any error is encountered. Update must complete
// successfully at least once before Template.String() can be called.
func (t *Template) String() string {
	globalMtx.RLock()
	defer globalMtx.RUnlock()
	rv, err := t.Execute(global)
	if err != nil {
		panic(err)
	}
	return rv
}

// Initialize the global Vars instance. Must complete successfully before
// Template.String() can be called.
func Init(ctx context.Context, c *http.Client) error {
	globalBranchClient = chrome_branch.NewClient(c)
	return Update(ctx)
}

// Update the global Vars instance.
func Update(ctx context.Context) error {
	chromeBranches, err := globalBranchClient.Get(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	v := &Vars{
		Branches: &Branches{
			Chromium: chromeBranches,
		},
	}
	if err := v.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	globalMtx.Lock()
	defer globalMtx.Unlock()
	global = v
	return nil
}
