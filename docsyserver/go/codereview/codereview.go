// Package codereview defines an interface to a code review system such as
// Gerrit.
package codereview

import "context"

// Issue is the identifier for a CodeReview issue.
type Issue string

// A special value for an issue which means to use the main branch at HEAD.
const MainIssue Issue = "main"

// ListModifiedFilesResult is the results from calling CodeReview.ListModifiedFiles.
type ListModifiedFilesResult struct {
	// Filename relative to the root of the git repo.
	Filename string

	// Deleted is true if the file was deleted at the given patchset.
	Deleted bool
}

// CodeReview represents an abstraction of the information we want from a code
// review system such as Gerrit.
type CodeReview interface {
	// ListModifiedFiles returns a list of the modified files for the given
	// issue at the given Ref, where Ref is a git ref, such a
	// "refs/head/master".
	ListModifiedFiles(ctx context.Context, issue Issue, ref string) ([]ListModifiedFilesResult, error)

	// GetFiles returns the contents of the given file at the given Ref as a
	// byte slice, where Ref is a git ref, such a "refs/head/master".
	GetFile(ctx context.Context, filename string, ref string) ([]byte, error)

	// GetPatchsetInfo returns the most recent patchset Ref of the given issue
	// and also if the issue has been closed . Ref is a git ref, such a
	// "refs/head/master".
	GetPatchsetInfo(ctx context.Context, issue Issue) (string, bool, error)
}
