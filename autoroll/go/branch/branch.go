package branch

/*
   Package branch provides abstractions for dealing with branches.
*/

import (
	"context"
	"net/http"

	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/skerr"
)

// Branch represents a branch in a Git repo.
type Branch interface {
	// String returns the current fully-qualified ref.
	String() string

	// Update the Branch.
	Update(context.Context) error
}

// StaticBranch is a Branch which does not change.
type StaticBranch struct {
	ref string
}

// NewStaticBranch returns a StaticBranch instance.
func NewStaticBranch(branch string) *StaticBranch {
	return &StaticBranch{
		ref: branch,
	}
}

// See documentation for Branch interface.
func (b *StaticBranch) String() string {
	return b.ref
}

// See documentation for Branch interface.
func (b *StaticBranch) Update(ctx context.Context) error {
	return nil
}

// ChromeBranch is a Branch which updates according to the Chrome release
// schedule.
type ChromeBranch struct {
	m    chrome_branch.Manager
	ref  string
	tmpl string
}

// See documentation for Branch interface.
func (b *ChromeBranch) String() string {
	return b.ref
}

// See documentation for Branch interface.
func (b *ChromeBranch) Update(ctx context.Context) error {
	if err := b.m.Update(ctx); err != nil {
		return skerr.Wrap(err)
	}
	return b.updateRef()
}

// updateRef executes the template and updates the stored ref, without updating
// the chrome_branch.Manager.
func (b *ChromeBranch) updateRef() error {
	ref, err := b.m.Execute(b.tmpl)
	if err != nil {
		return skerr.Wrap(err)
	}
	b.ref = ref
	return nil
}

// NewChromeBranch returns a ChromeBranch instance.
func NewChromeBranch(ctx context.Context, c *http.Client, tmpl string) (*ChromeBranch, error) {
	m, err := chrome_branch.New(ctx, c)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	b := &ChromeBranch{
		m:    m,
		tmpl: tmpl,
	}
	if err := b.updateRef(); err != nil {
		return nil, skerr.Wrap(err)
	}
	return b, nil
}

var _ Branch = &StaticBranch{}
var _ Branch = &ChromeBranch{}
