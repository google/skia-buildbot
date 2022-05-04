package config_vars

/*
	Package config_vars provides helpers for configuration templates.
*/

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"strings"
	"sync"

	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/skerr"
)

var (
	// These templates should not be used.
	bannedTmpls = []string{
		"Chromium.Master.Number",
	}
)

// Vars represents current values of variables which may be used in templates.
type Vars struct {
	Branches *Branches `json:"branches"`
}

// Validate returns an error if Vars is not valid.
func (v *Vars) Validate() error {
	if v.Branches == nil {
		return skerr.Fmt("Branches is required.")
	}
	if err := v.Branches.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Copy returns a deep copy of the Vars.
func (v *Vars) Copy() *Vars {
	if v == nil {
		return nil
	}
	return &Vars{
		Branches: v.Branches.Copy(),
	}
}

// DummyVars returns an instance of Vars with arbitrary contents which may be
// used during validation and testing.
func DummyVars() *Vars {
	return &Vars{
		Branches: &Branches{
			Chromium: &chrome_branch.Branches{
				Main: &chrome_branch.Branch{
					Milestone: 82,
					Number:    0,
					Ref:       chrome_branch.RefMain,
					V8Branch:  "8.2-lkgr",
				},
				Beta: &chrome_branch.Branch{
					Milestone: 81,
					Number:    4044,
					Ref:       chrome_branch.ReleaseBranchRef(4044),
					V8Branch:  "8.1-lkgr",
				},
				Stable: &chrome_branch.Branch{
					Milestone: 80,
					Number:    3987,
					Ref:       chrome_branch.ReleaseBranchRef(4044),
					V8Branch:  "8.0-lkgr",
				},
			},
		},
	}
}

// Branches represents named branches in git repositories.
type Branches struct {
	// Chromium release branches.
	Chromium *chrome_branch.Branches `json:"chromium"`
}

// Validate returns an error if Branches is not valid.
func (b *Branches) Validate() error {
	if b.Chromium == nil {
		return skerr.Fmt("Chromium is required.")
	}
	if err := b.Chromium.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Copy returns a deep copy of the Branches.
func (b *Branches) Copy() *Branches {
	if b == nil {
		return nil
	}
	var chromium *chrome_branch.Branches
	if b.Chromium != nil {
		chromium = b.Chromium.Copy()
	}
	return &Branches{
		Chromium: chromium,
	}
}

// Template is a text template which uses the contents of Vars. It is
// technically thread-safe, but users should take care to cache the value of
// Template.String() rather than calling it repeatedly, unless Update() is
// only ever called within the same thread as String().
type Template struct {
	mtx   sync.RWMutex
	raw   string
	tmpl  *template.Template
	value string
}

// NewTemplate returns a Template instance.
func NewTemplate(rawTmpl string) (*Template, error) {
	rv := &Template{}
	if err := rv.init(rawTmpl); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

// init initializes the template.
func (t *Template) init(rawTmpl string) error {
	for _, banned := range bannedTmpls {
		if strings.Contains(rawTmpl, banned) {
			return skerr.Fmt("Templates should not use %q", banned)
		}
	}
	tmpl, err := template.New("").Parse(rawTmpl)
	if err != nil {
		return skerr.Wrap(err)
	}
	tmpl.Option("missingkey=error")
	t.raw = rawTmpl
	t.tmpl = tmpl
	return skerr.Wrap(t.validate())
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *Template) UnmarshalJSON(b []byte) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	var raw string
	if err := json.Unmarshal(b, &raw); err != nil {
		return skerr.Wrap(err)
	}
	return skerr.Wrap(t.init(raw))
}

// MarshalJSON implements json.Marshaler.
func (t *Template) MarshalJSON() ([]byte, error) {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	return json.Marshal(t.raw)
}

// Validate returns an error if the Template is not valid.
func (t *Template) Validate() error {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	return skerr.Wrap(t.validate())
}

// validate returns an error if the Template is not valid. Assumes the caller
// holds t.mtx.
func (t *Template) validate() error {
	if t == nil || t.raw == "" || t.tmpl == nil {
		return skerr.Fmt("Template is missing.")
	}
	// Create dummy Vars to test the template.
	v := DummyVars()

	// Self-check, in case any of the members of Vars change.
	if err := v.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	var buf bytes.Buffer
	if err := t.tmpl.Execute(&buf, v); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Update executes the Template using the given Vars and updates its value.
func (t *Template) Update(v *Vars) error {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	var buf bytes.Buffer
	if err := t.tmpl.Execute(&buf, v); err != nil {
		return skerr.Wrap(err)
	}
	t.value = buf.String()
	return nil
}

// String returns the current value of the Template.
func (t *Template) String() string {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	return t.value
}

// Equal returns true iff the given Template is equivalent to this one. This
// allows us to test equality using deepequal.DeepEqual, despite the fact that
// text.Templates are never DeepEqual.
func (t *Template) Equal(t2 *Template) bool {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	t2.mtx.RLock()
	defer t2.mtx.RUnlock()
	if t.raw != t2.raw {
		return false
	}
	if t.value != t2.value {
		return false
	}
	return true
}

// RawTemplate returns the raw string value of the Template.
func (t *Template) RawTemplate() string {
	return t.raw
}

// Registry manages a set of Templates, updating their values when Update is
// called.
type Registry struct {
	cbc       chrome_branch.Client
	mtx       sync.Mutex
	templates []*Template
	vars      *Vars
}

// NewRegistry returns a Registry instance. Implicitly runs Update().
func NewRegistry(ctx context.Context, cbc chrome_branch.Client) (*Registry, error) {
	r := &Registry{
		cbc: cbc,
	}
	if err := r.Update(ctx); err != nil {
		return nil, err
	}
	return r, nil
}

// Register the given Template. Implicitly Updates its value using the most
// recent Vars.
func (r *Registry) Register(t *Template) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if err := t.Update(r.vars); err != nil {
		return err
	}
	r.templates = append(r.templates, t)
	return nil
}

// updateFrom updates all of the Templates managed by this Registry using the
// given Vars.
func (r *Registry) updateFrom(v *Vars) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	for _, tmpl := range r.templates {
		if err := tmpl.Update(v); err != nil {
			return skerr.Wrap(err)
		}
	}
	r.vars = v
	return nil
}

// Update all of the Templates managed by this Registry by loading new values.
func (r *Registry) Update(ctx context.Context) error {
	chromeBranches, err := r.cbc.Get(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	vars := &Vars{
		Branches: &Branches{
			Chromium: chromeBranches,
		},
	}
	return skerr.Wrap(r.updateFrom(vars))
}

// Vars returns a copy of the current value of the stored Vars.
func (r *Registry) Vars() *Vars {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	return r.vars.Copy()
}
