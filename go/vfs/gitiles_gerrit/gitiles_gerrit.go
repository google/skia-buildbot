package gitiles_gerrit

import (
	"context"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vfs"
	gitiles_vfs "go.skia.org/infra/go/vfs/gitiles"
)

// FS implements vfs.FS using Gitiles and Gerrit.
type FS struct {
	*gitiles_vfs.FS
}

// New returns a FS which can create a CL if files are changed.
func New(wrapped *gitiles_vfs.FS) *FS {
	return &FS{
		FS: wrapped,
	}
}

// HasChanges returns true if the FS has changes.
func (fs *FS) HasChanges() bool {
	changes := fs.Changes()
	return len(changes) > 0
}

// CreateChange creates a CL with the changes to the FS.
func (fs *FS) CreateChange(ctx context.Context, g gerrit.GerritInterface, project, branch, commitMsg string, reviewers []string) (*gerrit.ChangeInfo, error) {
	if !fs.HasChanges() {
		return nil, skerr.Fmt("no changes to use for CL")
	}
	changesStr := make(map[string]string, len(fs.Changes()))
	for k, v := range fs.Changes() {
		changesStr[k] = string(v)
	}
	return gerrit.CreateCLWithChanges(ctx, g, project, branch, commitMsg, fs.BaseCommit(), "", changesStr, reviewers)
}

// MaybeCreateChange creates a CL if any changes were made.
func (fs *FS) MaybeCreateChange(ctx context.Context, g gerrit.GerritInterface, project, branch, commitMsg string, reviewers []string) (*gerrit.ChangeInfo, error) {
	if fs.HasChanges() {
		return fs.CreateChange(ctx, g, project, branch, commitMsg, reviewers)
	}
	return nil, nil
}

// Assert that FS implements vfs.FS.
var _ vfs.FS = &FS{}
